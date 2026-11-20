# Local Development & Contributing

Developing on NextDeploy itself requires Go 1.25+, Node.js 20+, and optional live-reloading tooling.

---

## Tooling Prerequisites

- **Go**: 1.25 or later (`go version`)
- **Mage**: Build tool used for development workflows (`go install github.com/magefile/mage@latest`)
- **Air**: Live reloader for rapid CLI and daemon iteration (`go install github.com/air-verse/air@latest`)

---

## Running with Live Reloading (Air)

NextDeploy includes pre-configured Air configuration files:

- `.air.cli.toml`: Watches the `cli/` directory and recompiles `nextdeploy` to `~/.nextdeploy/bin/nextdeploy` on file saves.
- `.air.daemon.toml`: Watches `daemon/` and recompiles `nextdeployd`.

### Starting Dev Mode

Run via Mage:

```bash
mage dev
```

Or invoke Air directly:

```bash
air -c .air.cli.toml
```

Ensure your `PATH` includes `~/.nextdeploy/bin` so that running `nextdeploy` always uses your latest live-compiled binary:

```bash
export PATH="$HOME/.nextdeploy/bin:$PATH"
```

---

## Common Mage Commands

NextDeploy defines developer tasks in `magefile.go`:

| Command | Description |
| :--- | :--- |
| `mage build` | Compile `nextdeploy` CLI and `nextdeployd` daemon. |
| `mage test` | Run all unit and integration tests across packages. |
| `mage lint` | Run `golangci-lint` with the project's ruleset. |
| `mage coverage` | Generate test coverage report in `coverage.html`. |
| `mage clean` | Clean up build artifacts and temporary test files. |

---

## Testing

Run tests with race detection:

```bash
go test -race ./...
```

To run end-to-end serverless tests:

```bash
go test -v ./cli/internal/serverless -run TestCloudflare
```
