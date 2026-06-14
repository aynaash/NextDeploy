package shared

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aynaash/nextdeploy/shared/sensitive"
)

// withinDir reports whether the cleaned path p is dest itself or lies beneath
// it. Used to reject archive entries (and symlink/hardlink targets) that would
// escape the extraction directory ("zip slip").
func withinDir(dest, p string) bool {
	dest = filepath.Clean(dest)
	p = filepath.Clean(p)
	if p == dest {
		return true
	}
	return strings.HasPrefix(p, dest+string(os.PathSeparator))
}

// ExtractTarGz extracts a gzipped tarball into dest using a pure-Go reader.
// Unlike a `tar -xzf` shell-out, every entry is validated so a malicious
// archive cannot write outside dest via "../" components, absolute paths, or
// symlink/hardlink targets that point outside the extraction root.
func ExtractTarGz(src, dest string) error {
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("resolve dest %s: %w", dest, err)
	}
	if err := os.MkdirAll(destAbs, 0750); err != nil {
		return fmt.Errorf("mkdir %s: %w", destAbs, err)
	}

	// #nosec G304 — src is validated by the caller (must be within uploadsDir).
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open archive %s: %w", src, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		// Reject absolute paths and any ".." traversal up front.
		if filepath.IsAbs(hdr.Name) || strings.Contains(hdr.Name, "..") {
			return fmt.Errorf("unsafe archive entry path: %q", hdr.Name)
		}
		target := filepath.Join(destAbs, hdr.Name)
		if !withinDir(destAbs, target) {
			return fmt.Errorf("archive entry escapes destination: %q", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0750); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
				return fmt.Errorf("mkdir parent of %s: %w", target, err)
			}
			mode := hdr.FileInfo().Mode().Perm()
			if mode == 0 {
				mode = 0600
			}
			// #nosec G304 — target is validated to be within destAbs above.
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
			if err != nil {
				return fmt.Errorf("create %s: %w", target, err)
			}
			// #nosec G110 — decompression bomb is a DoS concern, out of scope here.
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return fmt.Errorf("write %s: %w", target, err)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("close %s: %w", target, err)
			}

		case tar.TypeSymlink:
			// Resolve the link target relative to the symlink's own location
			// and refuse anything that would point outside destAbs.
			var resolved string
			if filepath.IsAbs(hdr.Linkname) {
				resolved = filepath.Clean(hdr.Linkname)
			} else {
				resolved = filepath.Join(filepath.Dir(target), hdr.Linkname)
			}
			if !withinDir(destAbs, resolved) {
				return fmt.Errorf("symlink %q -> %q escapes destination", hdr.Name, hdr.Linkname)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
				return fmt.Errorf("mkdir parent of symlink %s: %w", target, err)
			}
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return fmt.Errorf("symlink %s: %w", target, err)
			}

		case tar.TypeLink:
			// Hardlink target is relative to the archive root (destAbs).
			linkTarget := filepath.Join(destAbs, hdr.Linkname)
			if !withinDir(destAbs, linkTarget) {
				return fmt.Errorf("hardlink %q -> %q escapes destination", hdr.Name, hdr.Linkname)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
				return fmt.Errorf("mkdir parent of hardlink %s: %w", target, err)
			}
			_ = os.Remove(target)
			if err := os.Link(linkTarget, target); err != nil {
				return fmt.Errorf("hardlink %s: %w", target, err)
			}

		default:
			// Skip char/block devices, FIFOs, etc. — not needed for app artifacts.
			continue
		}
	}
	return nil
}

// CreateTarGz packs srcDir into a gzipped tar at targetTar. Entries are
// relative to srcDir (no leading dir component). Uses system `tar` when
// available so behaviour matches `ExtractTarGz`; no pure-Go fallback yet
// because every supported dev/CI host ships tar.
func CreateTarGz(srcDir, targetTar string) error {
	absTarget, err := filepath.Abs(targetTar)
	if err != nil {
		return err
	}
	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return err
	}
	if info, err := os.Stat(absSrc); err != nil {
		return fmt.Errorf("tar source %s: %w", absSrc, err)
	} else if !info.IsDir() {
		return fmt.Errorf("tar source %s is not a directory", absSrc)
	}
	if err := os.MkdirAll(filepath.Dir(absTarget), 0o750); err != nil {
		return fmt.Errorf("mkdir for tar target: %w", err)
	}

	tarPath, err := exec.LookPath("tar")
	if err != nil {
		return fmt.Errorf("tar utility not found in PATH: %w", err)
	}

	// -C srcDir, then pack '.' so entries are stored without a leading
	// directory — symmetric with ExtractTarGz.
	// #nosec G204
	cmd := exec.Command(tarPath, "-czf", absTarget, "-C", absSrc, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar creation failed: %v - %s", err, string(out))
	}
	return nil
}

func CreateZip(srcDir, targetZip string) error {
	absTarget, _ := filepath.Abs(targetZip)
	absSrc, _ := filepath.Abs(srcDir)

	zipPath, err := exec.LookPath("zip")
	if err == nil {
		cmd := exec.Command(zipPath, "-rq9", absTarget, ".")
		cmd.Dir = absSrc
		if out, err := cmd.CombinedOutput(); err != nil {
			sensitive.Printf("System zip failed, falling back: %v - %s\n", err, string(out))
		} else {
			return nil
		}
	}

	zipFile, err := os.Create(targetZip)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		absPath, _ := filepath.Abs(path)
		if absPath == absTarget {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if d.IsDir() {
			header.Name += "/"
			header.Method = zip.Store
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}
