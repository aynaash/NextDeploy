package initialcommand

// import (
// 	"nextdeploy/internal/detect"
// 	"nextdeploy/internal/fs"
// 	"nextdeploy/internal/prompt"
// 	"nextdeploy/internal/secrets"
// 	"nextdeploy/internal/types"
// )
//
// func InitFlow(opts types.InitOptions, prompter prompt.Prompter) error {
// 	// Detect package manager
// 	pm, err := detect.DetectPackageManager(".")
// 	if err != nil {
// 		return err
// 	}
//
// 	if opts.PackageManager == "" {
// 		opts.PackageManager = pm
// 	}
//
// 	// Confirm overwrites
// 	fw := fs.NewFileWriter(opts.Force, opts.DryRun)
// 	if err := fw.Write("Dockerfile", []byte("...docker content...")); err != nil {
// 		return err
// 	}
//
// 	if err := fw.Write("nextdeploy.yml", []byte("...config...")); err != nil {
// 		return err
// 	}
//
// 	// Secret bootstrapping
// 	if opts.SecretsProvider != "" {
// 		sp := secrets.NewProvider(opts.SecretsProvider)
// 		if err := sp.BootstrapSecrets("."); err != nil {
// 			return err
// 		}
// 	}
//
// 	return nil
// }
