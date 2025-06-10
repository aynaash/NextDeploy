package types

type InitOptions struct {
	Force            bool
	SkipPrompts      bool
	UseDefaultConfig bool
	PackageManager   string
	SecretsProvider  string
	NonInteractive   bool
	DryRun           bool
}
