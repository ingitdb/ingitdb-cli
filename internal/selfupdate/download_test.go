package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestAssetName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		version string
		goos    string
		goarch  string
		want    string
	}{
		{"linux amd64", "1.2.3", "linux", "amd64", "ingitdb_1.2.3_linux_amd64.tar.gz"},
		{"darwin arm64", "1.2.3", "darwin", "arm64", "ingitdb_1.2.3_darwin_arm64.tar.gz"},
		{"windows amd64", "1.2.3", "windows", "amd64", "ingitdb_1.2.3_windows_amd64.zip"},
		{"strips leading v", "v1.2.3", "linux", "amd64", "ingitdb_1.2.3_linux_amd64.tar.gz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AssetName(tt.version, tt.goos, tt.goarch); got != tt.want {
				t.Errorf("AssetName(%q,%q,%q) = %q, want %q", tt.version, tt.goos, tt.goarch, got, tt.want)
			}
		})
	}
}

// Checksum files are published per OS, not as a single checksums.txt.
func TestChecksumsName_PerOS(t *testing.T) {
	t.Parallel()
	cases := []struct {
		goos string
		want string
	}{
		{"linux", "checksums.txt"},
		{"darwin", "checksums-darwin.txt"},
		{"windows", "checksums-windows.txt"},
	}
	for _, c := range cases {
		if got := checksumsName(c.goos); got != c.want {
			t.Errorf("checksumsName(%q) = %q, want %q", c.goos, got, c.want)
		}
	}
}

// makeTarGz builds a .tar.gz archive containing a single file named binName
// with the given bytes.
func makeTarGz(t *testing.T, binName string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{Name: binName, Mode: 0o755, Size: int64(len(content))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func makeZip(t *testing.T, binName string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(binName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// serveRelease starts an httptest.Server that serves the named asset and the
// per-OS checksum file with the given (possibly wrong) hash for the asset.
func serveRelease(t *testing.T, goos, assetName string, assetBytes []byte, checksumHash string) string {
	t.Helper()
	checksumName := checksumsName(goos)
	checksums := fmt.Sprintf("%s  %s\n", checksumHash, assetName)
	mux := http.NewServeMux()
	mux.HandleFunc("/"+assetName, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(assetBytes)
	})
	mux.HandleFunc("/"+checksumName, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(checksums))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestDownloadAndVerify_Success(t *testing.T) {
	t.Parallel()
	version := "1.2.3"
	binContent := []byte("the new ingitdb binary bytes")
	assetName := AssetName(version, "linux", "amd64")
	archive := makeTarGz(t, "ingitdb", binContent)
	baseURL := serveRelease(t, "linux", assetName, archive, sha256Hex(archive))

	d := Downloader{BaseURL: baseURL, Client: http.DefaultClient}
	path, err := d.DownloadAndVerify(context.Background(), version, "linux", "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if !bytes.Equal(got, binContent) {
		t.Errorf("extracted binary = %q, want %q", got, binContent)
	}
}

func TestDownloadAndVerify_Zip(t *testing.T) {
	t.Parallel()
	version := "1.2.3"
	binContent := []byte("windows binary bytes")
	assetName := AssetName(version, "windows", "amd64")
	archive := makeZip(t, "ingitdb.exe", binContent)
	baseURL := serveRelease(t, "windows", assetName, archive, sha256Hex(archive))

	d := Downloader{BaseURL: baseURL, Client: http.DefaultClient}
	path, err := d.DownloadAndVerify(context.Background(), version, "windows", "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if !bytes.Equal(got, binContent) {
		t.Errorf("extracted binary = %q, want %q", got, binContent)
	}
}

// The downloader must fetch the checksum file matching the target OS
// (checksums-darwin.txt for darwin builds), not the linux checksums.txt.
func TestDownloadAndVerify_DarwinChecksumFile(t *testing.T) {
	t.Parallel()
	version := "1.2.3"
	binContent := []byte("darwin binary bytes")
	assetName := AssetName(version, "darwin", "arm64")
	archive := makeTarGz(t, "ingitdb", binContent)
	baseURL := serveRelease(t, "darwin", assetName, archive, sha256Hex(archive))

	d := Downloader{BaseURL: baseURL, Client: http.DefaultClient}
	path, err := d.DownloadAndVerify(context.Background(), version, "darwin", "arm64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
}

func TestDownloadAndVerify_ChecksumMismatchAborts(t *testing.T) {
	t.Parallel()
	version := "1.2.3"
	binContent := []byte("the new ingitdb binary bytes")
	assetName := AssetName(version, "linux", "amd64")
	archive := makeTarGz(t, "ingitdb", binContent)
	// Wrong checksum: hash of unrelated bytes.
	wrongHash := sha256Hex([]byte("not the archive"))
	baseURL := serveRelease(t, "linux", assetName, archive, wrongHash)

	d := Downloader{BaseURL: baseURL, Client: http.DefaultClient}
	path, err := d.DownloadAndVerify(context.Background(), version, "linux", "amd64")
	if err == nil {
		t.Fatalf("expected verification error, got nil (path=%q)", path)
	}
	if path != "" {
		_ = os.Remove(path)
		t.Fatalf("expected no extracted file on mismatch, got path %q", path)
	}
}

func TestDownloadAndVerify_MissingChecksumEntry(t *testing.T) {
	t.Parallel()
	version := "1.2.3"
	assetName := AssetName(version, "linux", "amd64")
	archive := makeTarGz(t, "ingitdb", []byte("the new ingitdb binary bytes"))
	checksumName := checksumsName("linux")

	mux := http.NewServeMux()
	mux.HandleFunc("/"+assetName, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/"+checksumName, func(w http.ResponseWriter, _ *http.Request) {
		// Lists a different file, not our asset.
		otherAsset := AssetName(version, "linux", "arm64")
		_, _ = fmt.Fprintf(w, "%s  %s\n", sha256Hex(archive), otherAsset)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	d := Downloader{BaseURL: srv.URL, Client: http.DefaultClient}
	path, err := d.DownloadAndVerify(context.Background(), version, "linux", "amd64")
	if err == nil {
		_ = os.Remove(path)
		t.Fatalf("expected error for missing checksum entry, got nil")
	}
	if path != "" {
		_ = os.Remove(path)
		t.Fatalf("expected no extracted file, got path %q", path)
	}
}

// A release without an asset for the host OS/arch (e.g. the darwin job
// failing to publish) must produce a clear non-nil error before any
// extraction or write.
func TestDownloadAndVerify_MissingOSAsset(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	d := Downloader{BaseURL: srv.URL, Client: http.DefaultClient}
	path, err := d.DownloadAndVerify(context.Background(), "1.2.3", "darwin", "arm64")
	if err == nil {
		_ = os.Remove(path)
		t.Fatal("expected error when the OS asset is missing, got nil")
	}
	if path != "" {
		t.Fatalf("expected no extracted file, got path %q", path)
	}
}
