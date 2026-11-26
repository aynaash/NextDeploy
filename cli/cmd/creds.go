package cmd

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/credstore"
)

var providerSchemas = map[string][]credField{
	"cloudflare": {
		{Key: "api_token", Label: "Cloudflare API token", Required: true, Hidden: true},
		{Key: "account_id", Label: "Cloudflare account ID", Required: true, Hidden: false},
		{Key: "r2_access_key_id", Label: "R2 access key ID (optional, for R2 uploads)", Required: false, Hidden: false},
		{Key: "r2_secret_key", Label: "R2 secret access key (optional, for R2 uploads)", Required: false, Hidden: true},
	},
	"aws": {
		{Key: "access_key_id", Label: "AWS access key ID", Required: true, Hidden: false},
		{Key: "secret_access_key", Label: "AWS secret access key", Required: true, Hidden: true},
		{Key: "session_token", Label: "AWS session token (optional)", Required: false, Hidden: true},
	},
	"vps": {
		{Key: "host", Label: "VPS host / IP address", Required: true, Hidden: false},
		{Key: "username", Label: "SSH username (optional)", Required: false, Hidden: false},
		{Key: "key_path", Label: "SSH private key path (optional, e.g. ~/.ssh/id_ed25519)", Required: false, Hidden: false},
		{Key: "key_passphrase", Label: "SSH key passphrase (optional)", Required: false, Hidden: true},
	},
}

type credField struct {
	Key      string
	Label    string
	Required bool
	Hidden   bool
}

var credsCmd = &cobra.Command{
	Use:   "creds",
	Short: "Manage cloud-provider and VPS credentials in the encrypted credstore",
	Long: `Stores cloud-provider and VPS server credentials in an AES-GCM encrypted file under
~/.nextdeploy/credstore/. Per-machine, mode 0600. Use this instead of
committing keys or VPS IPs to nextdeploy.yml or relying on shell-exported env vars.

Resolution order at runtime: env vars → credstore → nextdeploy.yml.`,
}

var (
	credsProviderFlag string
	credsServerFlag   string
)

func resolveCredsTarget(providerFlag, serverFlag string, args []string) (provider, storeKey string, err error) {
	flagVal := strings.ToLower(strings.TrimSpace(providerFlag))
	var argVal string
	if len(args) > 0 {
		argVal = strings.ToLower(strings.TrimSpace(args[0]))
	}
	if flagVal != "" && argVal != "" && flagVal != argVal {
		return "", "", fmt.Errorf("conflicting provider argument %q and --provider %q", argVal, flagVal)
	}
	provider = flagVal
	if provider == "" {
		provider = argVal
	}
	if provider == "" {
		return "", "", fmt.Errorf("provider is required (cloudflare, aws, vps); e.g. 'nextdeploy creds set vps'")
	}
	if _, ok := providerSchemas[provider]; !ok {
		return "", "", fmt.Errorf("unknown provider %q (supported: cloudflare, aws, vps)", provider)
	}

	storeKey = provider
	if srv := strings.TrimSpace(serverFlag); srv != "" {
		if provider != "vps" {
			return "", "", fmt.Errorf("--server is only supported with provider \"vps\"")
		}
		if strings.Contains(srv, "/") || strings.Contains(srv, `\`) || strings.Contains(srv, "..") {
			return "", "", fmt.Errorf("invalid server name %q", srv)
		}
		key := config.ServerCredstoreKey(srv)
		if key == "" {
			return "", "", fmt.Errorf("invalid server name %q", srv)
		}
		storeKey = key
	}
	return provider, storeKey, nil
}

var credsSetCmd = &cobra.Command{
	Use:   "set [provider]",
	Short: "Store credentials for a provider or VPS server (interactive)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log := shared.PackageLogger("creds", "🔒 CREDS")
		provider, storeKey, err := resolveCredsTarget(credsProviderFlag, credsServerFlag, args)
		if err != nil {
			log.Error("%v", err)
			os.Exit(2)
		}
		schema := providerSchemas[provider]

		existing, _ := credstore.Load(storeKey)
		fields := map[string]string{}
		maps.Copy(fields, existing)

		for _, f := range schema {
			val, err := promptCredential(f, fields[f.Key] != "")
			if err != nil {
				log.Error("input error: %v", err)
				os.Exit(1)
			}
			if val == "" && fields[f.Key] != "" {
				continue // keep existing
			}
			if val == "" && f.Required {
				log.Error("%s is required", f.Label)
				os.Exit(2)
			}
			if val != "" {
				fields[f.Key] = val
			}
		}

		if err := credstore.Save(storeKey, fields); err != nil {
			log.Error("save failed: %v", err)
			os.Exit(1)
		}
		log.Success("Stored %d field(s) for %q", len(fields), storeKey)
	},
}

var credsClearCmd = &cobra.Command{
	Use:   "clear [provider]",
	Short: "Remove stored credentials for a provider or VPS server",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log := shared.PackageLogger("creds", "🔒 CREDS")
		_, storeKey, err := resolveCredsTarget(credsProviderFlag, credsServerFlag, args)
		if err != nil {
			log.Error("%v", err)
			os.Exit(2)
		}
		if err := credstore.Delete(storeKey); err != nil {
			log.Error("delete failed: %v", err)
			os.Exit(1)
		}
		log.Success("Cleared credentials for %q", storeKey)
	},
}

var credsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List providers with stored credentials (names only, never values)",
	Run: func(cmd *cobra.Command, args []string) {
		log := shared.PackageLogger("creds", "🔒 CREDS")
		providers, err := credstore.List()
		if err != nil {
			log.Error("list failed: %v", err)
			os.Exit(1)
		}
		if len(providers) == 0 {
			fmt.Println("(no credentials stored)")
			return
		}
		sort.Strings(providers)
		fmt.Println("Stored credentials:")
		for _, p := range providers {
			creds, err := credstore.Load(p)
			if err != nil {
				fmt.Printf("  %s — error: %v\n", p, err)
				continue
			}
			keys := make([]string, 0, len(creds))
			for k := range creds {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			fmt.Printf("  %s: %s\n", p, strings.Join(keys, ", "))
		}
	},
}

func promptCredential(f credField, hasExisting bool) (string, error) {
	label := f.Label
	if hasExisting {
		label += " (leave blank to keep existing)"
	}
	var ans string
	var prompt survey.Prompt
	if f.Hidden {
		prompt = &survey.Password{Message: label}
	} else {
		prompt = &survey.Input{Message: label}
	}
	if err := survey.AskOne(prompt, &ans); err != nil {
		if errors.Is(err, os.ErrClosed) || err.Error() == "interrupt" {
			return "", err
		}
		return "", err
	}
	return strings.TrimSpace(ans), nil
}

func init() {
	credsSetCmd.Flags().StringVar(&credsProviderFlag, "provider", "", "provider name (cloudflare, aws, vps)")
	credsSetCmd.Flags().StringVar(&credsServerFlag, "server", "", "VPS server name from nextdeploy.yml (for multi-server vps entries)")
	credsClearCmd.Flags().StringVar(&credsProviderFlag, "provider", "", "provider name (cloudflare, aws, vps)")
	credsClearCmd.Flags().StringVar(&credsServerFlag, "server", "", "VPS server name from nextdeploy.yml (for multi-server vps entries)")

	credsCmd.AddCommand(credsSetCmd)
	credsCmd.AddCommand(credsClearCmd)
	credsCmd.AddCommand(credsListCmd)
	rootCmd.AddCommand(credsCmd)
}
