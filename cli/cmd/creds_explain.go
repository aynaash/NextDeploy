package cmd

var credsExplanation = explanation{
	Name:     "creds",
	Synopsis: "Manage cloud-provider and VPS credentials in the encrypted credstore.",
	Summary: "`creds` manages the encrypted credential store at " +
		"~/.nextdeploy/credstore (mode 0600). AWS, Cloudflare, and VPS credentials " +
		"resolve in order: env vars (and local .env) first, then this credstore, then " +
		"nextdeploy.yml. Using the credstore keeps secrets and VPS IPs off disk in " +
		"cleartext, out of public git repos, and out of CI logs.",
	Phases: []phase{
		{
			Num:       1,
			Title:     "creds set <aws|cloudflare|vps> [--server <name>]",
			Narrative: "Interactive prompts for each field the target needs: AWS wants access_key + secret_key; Cloudflare wants api_token + account_id + R2 access/secret; VPS wants host plus optional username, key_path, and key_passphrase (with --server <name> for multi-server entries). Values are encrypted with the per-machine key and written to ~/.nextdeploy/credstore.",
			Ref:       "cli/cmd/creds.go",
			Function:  "credstore.Save",
			Output:    "encrypted credstore entry",
		},
		{
			Num:       2,
			Title:     "creds clear <aws|cloudflare|vps> [--server <name>]",
			Narrative: "Removes one provider's (or VPS server's) entry from the credstore. The ship path will fall back to env vars or yaml on the next deploy.",
			Ref:       "cli/cmd/creds.go",
			Function:  "credstore.Delete",
		},
		{
			Num:       3,
			Title:     "creds list",
			Narrative: "Shows which providers have credstore entries, without revealing values. Useful for verifying the credstore is populated before a deploy.",
			Ref:       "cli/cmd/creds.go",
			Function:  "credstore.List",
		},
	},
}

func init() {
	registerExplain(credsCmd, &credsExplanation)
}
