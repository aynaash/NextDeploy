package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// metadataMaxBytes caps how much of a tarball entry we'll read as metadata.
// The real file is a few KB; the cap stops a malformed or hostile archive from
// pulling an unbounded amount into memory during a cheap pre-flight check.
const metadataMaxBytes = 4 << 20

// errNoMetadata means the archive carried no metadata.json — an old or
// hand-built tarball. Callers treat it as "can't verify", not "mismatch".
var errNoMetadata = errors.New("no metadata.json in artifact")

// tarballAppName reads the app name recorded inside a deployment artifact.
//
// The daemon validates the app name from the tarball's OWN metadata.json, not
// from the config the CLI is holding. When an incremental skip reuses a stale
// tarball those two diverge, and the mismatch only surfaces after a multi-MB
// upload and an SSH round-trip. The CLI already has both values locally — this
// lets it compare them in milliseconds instead.
func tarballAppName(tarballPath string) (string, error) {
	f, err := os.Open(tarballPath) // #nosec G304 — path is the artifact this command just built or was told to ship
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("read artifact %s: %w", tarballPath, err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return "", errNoMetadata
		}
		if err != nil {
			return "", fmt.Errorf("read artifact %s: %w", tarballPath, err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		// Mirror the daemon's readMetadata, which tries <root>/.nextdeploy/
		// metadata.json then <root>/metadata.json. Leading "./" varies with how
		// tar was invoked, so normalize before comparing.
		clean := strings.TrimPrefix(path.Clean(hdr.Name), "./")
		if clean != ".nextdeploy/metadata.json" && clean != "metadata.json" {
			continue
		}
		var meta struct {
			AppName string `json:"app_name"`
		}
		if err := json.NewDecoder(io.LimitReader(tr, metadataMaxBytes)).Decode(&meta); err != nil {
			return "", fmt.Errorf("parse metadata.json in %s: %w", tarballPath, err)
		}
		if meta.AppName == "" {
			continue
		}
		return meta.AppName, nil
	}
}
