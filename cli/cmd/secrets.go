package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aynaash/nextdeploy/cli/internal/server"
	"github.com/aynaash/nextdeploy/cli/internal/serverless"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/spf13/cobra"
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage application secrets and environment variables",
	Long:  "Allows you to set, get, list, and unset environment variables for your deployed application. Secrets are securely synced to the daemon.",
}

var secretsSetCmd = &cobra.Command{
	Use:   "set KEY=VALUE...",
	Short: "Set one or more secrets",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runSecretAction("set", args)
	},
}

var secretsGetCmd = &cobra.Command{
	Use:   "get KEY",
	Short: "Get a secret value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runSecretAction("get", args)
	},
}

var secretsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all secret names",
	Run: func(cmd *cobra.Command, args []string) {
		runSecretAction("list", args)
	},
}

var secretsUnsetCmd = &cobra.Command{
	Use:   "unset KEY...",
	Short: "Remove one or more secrets",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runSecretAction("unset", args)
	},
}

var secretsLoadCmd = &cobra.Command{
	Use:   "load [FILENAME]",
	Short: "Load secrets from a .env file",
	Long:  "Reads a local .env file and uploads all key-value pairs to the daemon. Defaults to '.env' if no filename is provided.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filename := ".env"
		if len(args) > 0 {
			filename = args[0]
		}
		runSecretsLoad(filename)
	},
}

var (
	secretsPruneApply bool
)

var secretsPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Delete remote secrets that are not in the local allowlist",
	Long: `Diffs the secrets currently stored on your deploy target against the
canonical local set (Doppler keys + .env + nextdeploy managed store) and
removes anything on the remote that isn't owned by you.

Useful after a leaky deploy that pushed shell environment variables (e.g.
KITTY_PID, WAYLAND_DISPLAY, GNOME_DESKTOP_SESSION_ID) to a Cloudflare
Worker. Prints the diff and exits without changes by default — pass
--apply to actually delete.`,
	Run: func(cmd *cobra.Command, args []string) {
		runSecretsPrune(secretsPruneApply)
	},
}

// runSecretsPrune is the workhorse for `nextdeploy secrets prune`.
//
// Algorithm:
//  1. Compute the local allowlist via serverless.LoadLocalSecrets — this
//     is the same merge that `nextdeploy ship` would push (Doppler keys
//     + .env + managed store).
//  2. Fetch the remote secret name list via Provider.GetSecrets.
//  3. Diff: anything on the remote that isn't in the local allowlist is
//     a deletion candidate.
//  4. Print the candidates. With --apply, call Provider.UnsetSecret for
//     each.
//
// The dry-run-by-default policy is intentional: deleting secrets is
// destructive and the user should see exactly what will be removed
// before authorising it.
func runSecretsPrune(apply bool) {
	log := shared.PackageLogger("secrets", "🔐 SECRETS")
	cfg, err := config.Load()
	if err != nil {
		log.Error("Failed to load config: %v", err)
		os.Exit(1)
	}

	target := strings.ToLower(cfg.TargetType)
	if target != "serverless" {
		log.Error("`secrets prune` is only supported for serverless targets (got: %s)", target)
		os.Exit(1)
	}

	providerName := "aws"
	if cfg.Serverless != nil && cfg.Serverless.Provider != "" {
		providerName = strings.ToLower(cfg.Serverless.Provider)
	}

	p, err := serverless.New(providerName, false)
	if err != nil {
		log.Error("Failed to initialize serverless provider: %v", err)
		os.Exit(1)
	}
	ctx := context.Background()
	if err := p.Initialize(ctx, cfg); err != nil {
		log.Error("Failed to initialize provider: %v", err)
		os.Exit(1)
	}

	local, err := serverless.LoadLocalSecrets(cfg)
	if err != nil {
		log.Error("Failed to load local secrets: %v", err)
		os.Exit(1)
	}
	if len(local) == 0 {
		log.Error(
			"Local allowlist is empty — refusing to prune. " +
				"This usually means Doppler is not configured or .env is missing. " +
				"Pruning against an empty allowlist would delete every remote secret, " +
				"which is almost certainly not what you want.",
		)
		os.Exit(1)
	}

	remote, err := p.GetSecrets(ctx, cfg.App.Name)
	if err != nil {
		log.Error("Failed to list remote secrets: %v", err)
		os.Exit(1)
	}

	candidates := diffPruneCandidates(remote, local)
	if len(candidates) == 0 {
		log.Success("No leaked secrets — remote matches local allowlist (%d keys)", len(local))
		return
	}

	log.Info("Local allowlist: %d keys", len(local))
	log.Info("Remote: %d keys", len(remote))
	log.Warn("Candidates to delete: %d", len(candidates))
	for _, k := range candidates {
		fmt.Printf("  - %s\n", k)
	}

	if !apply {
		log.Info("Dry-run only. Re-run with `--apply` to delete these from the remote.")
		return
	}

	log.Info("Applying — deleting %d secrets...", len(candidates))
	deleted, failed := 0, 0
	for _, k := range candidates {
		if err := p.UnsetSecret(ctx, cfg.App.Name, k); err != nil {
			log.Warn("Failed to delete %s: %v", k, err)
			failed++
			continue
		}
		deleted++
	}
	log.Success("Pruned %d secrets (%d failures)", deleted, failed)
}

// diffPruneCandidates returns the remote secret names that are absent
// from the local allowlist, sorted ascending so the dry-run output is
// stable across runs.
func diffPruneCandidates(remote, local map[string]string) []string {
	var out []string
	for k := range remote {
		if _, ok := local[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func runSecretAction(action string, args []string) {
	log := shared.PackageLogger("secrets", "🔐 SECRETS")
	cfg, err := config.Load()
	if err != nil {
		log.Error("Failed to load config: %v", err)
		os.Exit(1)
	}

	appName := cfg.App.Name
	target := strings.ToLower(cfg.TargetType)

	if target == "" {
		log.Warn("No target_type specified in nextdeploy.yml, defaulting to VPS")
		target = "vps"
	}

	switch target {
	case "serverless":
		runServerlessSecretAction(action, args, appName, cfg, log)
	case "vps":
		log.Info("Using Server Secrets (VPS NextDeploy Daemon)")
		runVPSSecretAction(action, args, appName, log)
	default:
		log.Warn("Unknown target_type '%s', attempting VPS fallback", target)
		runVPSSecretAction(action, args, appName, log)
	}
}

func runServerlessSecretAction(action string, args []string, appName string, cfg *config.NextDeployConfig, log *shared.Logger) {
	providerName := "aws"
	if cfg.Serverless != nil && cfg.Serverless.Provider != "" {
		providerName = strings.ToLower(cfg.Serverless.Provider)
	}

	storeName := "AWS Secrets Manager"
	switch providerName {
	case "cloudflare":
		storeName = "Cloudflare Worker secrets"
	}
	log.Info("Using Cloud Secrets (%s)", storeName)

	p, err := serverless.New(providerName, false)
	if err != nil {
		log.Error("Failed to initialize serverless provider: %v", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := p.Initialize(ctx, cfg); err != nil {
		log.Error("Failed to initialize provider: %v", err)
		os.Exit(1)
	}

	switch action {
	case "set":
		for _, arg := range args {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				log.Warn("Invalid format for '%s', expected KEY=VALUE", arg)
				continue
			}
			if err := p.SetSecret(ctx, appName, parts[0], parts[1]); err != nil {
				log.Error("Failed to set secret %s: %v", parts[0], err)
			} else {
				log.Success("Secret %s set in %s", parts[0], storeName)
			}
		}
	case "get":
		secrets, err := p.GetSecrets(ctx, appName)
		if err != nil {
			log.Error("Failed to get secrets: %v", err)
		} else {
			if val, ok := secrets[args[0]]; ok {
				// Intentional plaintext sink: 'secrets get' is the canonical
				// way to retrieve a value. Do NOT route through sensitive.Printf
				// — that would scrub registered values and break the command.
				fmt.Printf("%s=%s\n", args[0], val)
			} else {
				log.Warn("Secret %s not found", args[0])
			}
		}
	case "list":
		secrets, err := p.GetSecrets(ctx, appName)
		if err != nil {
			log.Error("Failed to list secrets: %v", err)
		} else {
			fmt.Printf("Secrets for %s (%s):\n", appName, storeName)
			keys := make([]string, 0, len(secrets))
			for k := range secrets {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Println(k)
			}
		}
	case "unset":
		for _, key := range args {
			if err := p.UnsetSecret(ctx, appName, key); err != nil {
				log.Error("Failed to unset secret %s: %v", key, err)
			} else {
				log.Success("Secret %s removed from %s", key, storeName)
			}
		}
	}
}

func runVPSSecretAction(action string, args []string, appName string, log *shared.Logger) {
	srv, err := server.New(server.WithConfig(), server.WithSSH())
	if err != nil {
		log.Error("Failed to initialize server connection: %v", err)
		os.Exit(1)
	}
	defer srv.CloseSSHConnection()

	deploymentServer, err := srv.GetDeploymentServer()
	if err != nil {
		log.Error("Failed to get deployment server: %v", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch action {
	case "set":
		for _, arg := range args {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				log.Warn("Invalid format for '%s', expected KEY=VALUE", arg)
				continue
			}
			key, value := parts[0], parts[1]
			daemonCmd := fmt.Sprintf("sudo /usr/local/bin/nextdeployd secrets --action=set --appName=%s --key=%s --value=%s", shellQuote(appName), shellQuote(key), shellQuote(value))
			output, err := srv.ExecuteCommand(ctx, deploymentServer, daemonCmd, nil)
			if err != nil {
				log.Error("Failed to set secret %s: %v\nOutput: %s", key, err, output)
			} else {
				log.Success(" Secret %s set", key)
			}
		}
	case "get":
		key := args[0]
		daemonCmd := fmt.Sprintf("sudo /usr/local/bin/nextdeployd secrets --action=get --appName=%s --key=%s", shellQuote(appName), shellQuote(key))
		output, err := srv.ExecuteCommand(ctx, deploymentServer, daemonCmd, nil)
		if err != nil {
			log.Error("Failed to get secret %s: %v\nOutput: %s", key, err, output)
		} else {
			fmt.Printf("%s=%s\n", key, strings.TrimSpace(output))
		}
	case "list":
		daemonCmd := fmt.Sprintf("sudo /usr/local/bin/nextdeployd secrets --action=list --appName=%s", shellQuote(appName))
		output, err := srv.ExecuteCommand(ctx, deploymentServer, daemonCmd, nil)
		if err != nil {
			log.Error("Failed to list secrets: %v\nOutput: %s", err, output)
		} else {
			fmt.Printf("Secrets for %s:\n%s\n", appName, output)
		}
	case "unset":
		for _, key := range args {
			daemonCmd := fmt.Sprintf("sudo /usr/local/bin/nextdeployd secrets --action=unset --appName=%s --key=%s", shellQuote(appName), shellQuote(key))
			output, err := srv.ExecuteCommand(ctx, deploymentServer, daemonCmd, nil)
			if err != nil {
				log.Error("Failed to unset secret %s: %v\nOutput: %s", key, err, output)
			} else {
				log.Success("✅ Secret %s removed", key)
			}
		}
	}
}

func runSecretsLoad(filename string) {
	log := shared.PackageLogger("secrets", "🔐 SECRETS")
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Error("Failed to read file %s: %v", filename, err)
		os.Exit(1)
	}

	lines := strings.Split(string(data), "\n")
	var secrets []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle basic KEY=VALUE parsing
		if strings.Contains(line, "=") {
			secrets = append(secrets, line)
		}
	}

	if len(secrets) == 0 {
		log.Warn("No valid secrets found in %s", filename)
		return
	}

	log.Info("Loading %d secrets from %s...", len(secrets), filename)
	runSecretAction("set", secrets)
}

func init() {
	secretsCmd.AddCommand(secretsSetCmd)
	secretsCmd.AddCommand(secretsGetCmd)
	secretsCmd.AddCommand(secretsListCmd)
	secretsCmd.AddCommand(secretsUnsetCmd)
	secretsCmd.AddCommand(secretsLoadCmd)

	secretsPruneCmd.Flags().BoolVar(&secretsPruneApply, "apply", false, "Actually delete the candidates (default is dry-run)")
	secretsCmd.AddCommand(secretsPruneCmd)

	rootCmd.AddCommand(secretsCmd)
}
