// NextDeploy CLI is a command-line interface for interacting with and managing
// Next.js app deployments across self-hosted infrastructure.
//
// It allows developers to initialize deployments, push code, monitor logs,
// and configure services using a simple declarative `nextdeploy.yml` file.
//
// Typical usage:
//   nextdeploy init        # Scaffold a Dockerfile and config
//   nextdeploy ship    # Build and deploy app to server
//
// Author: Yussuf Hersi <dev@hersi.dev>
// License: MIT
// Source: https://github.com/aynaash/nextdeploy
//
// ─────────────────────────────────────────────────────────────────────────────
// Planned Features:
// - SSH key management with passphrase support
// - GitHub Webhook integration
// - Environment-specific overrides in `nextdeploy.yml`
// - Encrypted secrets store integration (e.g. Doppler, Vault)
// - Telemetry (opt-in)

package main

import (
	"nextdeploy/cli/cmd"
)

func main() {
	cmd.Execute()
}
