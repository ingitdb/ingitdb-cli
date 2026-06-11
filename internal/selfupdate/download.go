package selfupdate

// specscore: feature/cli/self-update

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"
)

// binaryName is the name of the executable inside the release archive.
const binaryName = "ingitdb"

// defaultDownloadBaseURL is where GoReleaser publishes release assets; the
// per-release directory is "<base>/v<version>".
const defaultDownloadBaseURL = "https://github.com/ingitdb/ingitdb-cli/releases/download"

// AssetName builds the GoReleaser archive name for the given version and target
// OS/arch. The version's leading "v" is stripped to match GoReleaser's
// name_template ("ingitdb_{Version}_{os}_{arch}"). Windows uses .zip; all
// other platforms use .tar.gz.
func AssetName(version, goos, goarch string) string {
	v := normalize(version)
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("ingitdb_%s_%s_%s.%s", v, goos, goarch, ext)
}

// checksumsName returns the per-OS checksums file name published with each
// release. Unlike a single checksums.txt, ingitdb releases publish one
// checksum file per OS build job: checksums.txt (linux),
// checksums-darwin.txt (darwin), and checksums-windows.txt (windows).
func checksumsName(goos string) string {
	switch goos {
	case "darwin":
		return "checksums-darwin.txt"
	case "windows":
		return "checksums-windows.txt"
	default:
		return "checksums.txt"
	}
}

// Downloader fetches and verifies release assets. BaseURL and Client are
// injectable so tests can target an httptest.Server.
type Downloader struct {
	// BaseURL is the directory under which the release's assets and checksum
	// files live. When empty, the versioned GitHub release download directory
	// ("<defaultDownloadBaseURL>/v<version>") is used.
	BaseURL string
	// Client is the HTTP client used for requests. When nil, http.DefaultClient
	// is used.
	Client *http.Client
}

// DownloadAndVerify downloads the release asset matching the given version and
// target OS/arch, verifies its sha256 against the release's per-OS checksum
// file, and — only on a successful match — extracts the ingitdb binary into a
// temp file whose path is returned. When goos/goarch are empty,
// runtime.GOOS/GOARCH are used.
//
// Verification happens before any extraction. On a checksum mismatch or a
// missing/unfetchable checksum entry, it returns a non-nil error and an empty
// path, having touched no executable. This function never writes to or
// replaces the running executable; the swap is a separate concern.
func (d Downloader) DownloadAndVerify(ctx context.Context, version, goos, goarch string) (string, error) {
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}

	base := d.BaseURL
	if base == "" {
		base = fmt.Sprintf("%s/v%s", defaultDownloadBaseURL, normalize(version))
	}

	asset := AssetName(version, goos, goarch)

	assetBytes, err := d.fetch(ctx, base, asset)
	if err != nil {
		return "", err
	}

	checksumFile := checksumsName(goos)
	checksumBytes, err := d.fetch(ctx, base, checksumFile)
	if err != nil {
		return "", err
	}

	expected, err := findChecksum(checksumBytes, asset)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(assetBytes)
	actual := hex.EncodeToString(sum[:])
	if !strings.EqualFold(actual, expected) {
		return "", fmt.Errorf(
			"checksum verification failed for %s: expected %s, got %s", asset, expected, actual)
	}

	// Verified — safe to extract.
	return extractBinary(assetBytes, goos)
}

// fetch GETs name relative to base and returns the body.
func (d Downloader) fetch(ctx context.Context, base, name string) ([]byte, error) {
	client := d.Client
	if client == nil {
		client = http.DefaultClient
	}

	url := strings.TrimRight(base, "/") + "/" + name
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download of %s failed: status %d", name, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// findChecksum parses a GoReleaser checksum file (lines of "<sha256hex>  <file>")
// and returns the expected hex digest for asset. A missing entry is an error.
func findChecksum(checksums []byte, asset string) (string, error) {
	reader := bytes.NewReader(checksums)
	sc := bufio.NewScanner(reader)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[1] == asset {
			return fields[0], nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no checksum entry found for %s", asset)
}

// extractBinary extracts the ingitdb executable from a verified archive into a
// temp file and returns its path. The archive format is chosen by goos
// (.zip for windows, .tar.gz otherwise).
func extractBinary(archive []byte, goos string) (string, error) {
	want := binaryName
	if goos == "windows" {
		want = binaryName + ".exe"
	}

	if goos == "windows" {
		return extractFromZip(archive, want)
	}
	return extractFromTarGz(archive, want)
}

func extractFromTarGz(archive []byte, want string) (string, error) {
	reader := bytes.NewReader(archive)
	gr, err := gzip.NewReader(reader)
	if err != nil {
		return "", fmt.Errorf("open gzip archive: %w", err)
	}
	defer func() {
		_ = gr.Close()
	}()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read tar archive: %w", err)
		}
		if path.Base(hdr.Name) == want {
			return writeTempBinary(tr)
		}
	}
	return "", fmt.Errorf("binary %s not found in archive", want)
}

func extractFromZip(archive []byte, want string) (string, error) {
	reader := bytes.NewReader(archive)
	zr, err := zip.NewReader(reader, int64(len(archive)))
	if err != nil {
		return "", fmt.Errorf("open zip archive: %w", err)
	}
	for _, f := range zr.File {
		if path.Base(f.Name) == want {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("open %s in archive: %w", want, err)
			}
			defer func() {
				_ = rc.Close()
			}()
			return writeTempBinary(rc)
		}
	}
	return "", fmt.Errorf("binary %s not found in archive", want)
}

// tempFile is the subset of *os.File used when writing a temp file. It is an
// interface so tests can inject a file whose Close fails after a successful
// copy.
type tempFile interface {
	io.WriteCloser
	Name() string
}

// createTempFunc is a test seam over os.CreateTemp so the temp-file creation
// and close error branches are exercisable. Tests that replace it MUST NOT
// run in parallel.
var createTempFunc = func(dir, pattern string) (tempFile, error) {
	return os.CreateTemp(dir, pattern)
}

// writeTempBinary copies r into a new temp file and returns its path.
func writeTempBinary(r io.Reader) (string, error) {
	f, err := createTempFunc("", "ingitdb-update-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("write temp binary: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("close temp binary: %w", err)
	}
	return f.Name(), nil
}
