# Webi Installer Submission Guide

This directory contains webi installers for NextDeploy.

## Structure

```
webi-installers/
├── nextdeploy/          # CLI installer (cross-platform)
│   ├── README.md        # Package description
│   ├── releases.js      # Fetches releases from GitHub
│   ├── install.sh       # POSIX shell installer
│   └── install.ps1      # PowerShell installer (Windows)
└── nextdeployd/         # Daemon installer (Linux only)
    ├── README.md
    ├── releases.js
    └── install.sh
```

## Testing Locally

Before submitting to webi-installers repo:

1. **Clone the webi-installers repo**:
   ```sh
   git clone https://github.com/webinstall/webi-installers.git
   cd webi-installers
   git submodule update --init
   npm clean-install
   ```

2. **Copy your installers**:
   ```sh
   cp -r /path/to/NextDeploy/webi-installers/nextdeploy ./
   cp -r /path/to/NextDeploy/webi-installers/nextdeployd ./
   ```

3. **Test the installers**:
   ```sh
   node _webi/test.js ./nextdeploy/
   node _webi/test.js ./nextdeployd/
   ```

## Release Requirements

Your GitHub releases **must** have binaries named like:

### CLI (nextdeploy)
- `nextdeploy-linux-amd64`
- `nextdeploy-linux-arm64`
- `nextdeploy-darwin-amd64`
- `nextdeploy-darwin-arm64`
- `nextdeploy-windows-amd64.exe`

### Daemon (nextdeployd)
- `nextdeployd-linux-amd64`
- `nextdeployd-linux-arm64`

## GoReleaser Configuration

Your `.goreleaser.yml` is already set up correctly! Just make sure:

1. **Binary names match** the expected format
2. **Releases are tagged** with `v*` (e.g., `v0.1.0`)
3. **Checksums are generated** (already configured)

## Submission Process

1. **Test locally** (see above)
2. **Fork** https://github.com/webinstall/webi-installers
3. **Copy** your installer directories to the fork
4. **Create PR** with title: "feat: add nextdeploy and nextdeployd"
5. **Wait for review** from webi maintainers

## After Approval

Users can install with:

```sh
# CLI (all platforms)
curl https://webi.sh/nextdeploy | sh

# Daemon (Linux only)
curl https://webi.sh/nextdeployd | sh
```

## Maintenance

When you release new versions:
1. Tag the release (e.g., `git tag v0.2.0`)
2. Push the tag: `git push origin v0.2.0`
3. GitHub Actions will build and release
4. Webi will automatically pick up the new version

No need to update the webi installers - they fetch from your GitHub releases API!

## Resources

- Webi docs: https://webinstall.dev
- Webi repo: https://github.com/webinstall/webi-installers
- Example installers:
  - Simple binary: https://github.com/webinstall/webi-installers/tree/master/ripgrep
  - Complex package: https://github.com/webinstall/webi-installers/tree/master/golang
  - With systemd: https://github.com/webinstall/webi-installers/tree/master/postgres
