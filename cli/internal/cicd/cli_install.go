package cicd

import "fmt"

func renderCLIInstallStep() string {
	return fmt.Sprintf(`      # Download the pinned, prebuilt release and verify its published checksum.
      - name: Install nextdeploy CLI (pinned)
        env:
          CLI_VERSION: %q
        run: |
          set -euo pipefail
          VERSION="${CLI_VERSION#v}"
          ARCHIVE="nextdeploy_${VERSION}_linux_amd64.tar.gz"
          RELEASE_URL="https://github.com/aynaash/NextDeploy/releases/download/${CLI_VERSION}"
          TMP_DIR="$(mktemp -d "$RUNNER_TEMP/nextdeploy.XXXXXX")"
          mkdir -p "$HOME/.local/bin"
          curl --fail --location --silent --show-error "$RELEASE_URL/$ARCHIVE" --output "$TMP_DIR/$ARCHIVE"
          curl --fail --location --silent --show-error "$RELEASE_URL/checksums.txt" --output "$TMP_DIR/checksums.txt"
          (cd "$TMP_DIR" && awk -v archive="$ARCHIVE" '$2 == archive { print; found=1 } END { if (!found) exit 1 }' checksums.txt | sha256sum --check --status)
          tar -xzf "$TMP_DIR/$ARCHIVE" -C "$TMP_DIR" nextdeploy
          install -m 0755 "$TMP_DIR/nextdeploy" "$HOME/.local/bin/nextdeploy"
          echo "$HOME/.local/bin" >> "$GITHUB_PATH"
`, CLIVersion)
}
