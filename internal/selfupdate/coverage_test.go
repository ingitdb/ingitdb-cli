package selfupdate

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- download.go: DownloadAndVerify / fetch error branches ---

func TestDownloadAndVerify_AssetFetchFails(t *testing.T) {
	t.Parallel()
	// Server returns 404 for everything → asset fetch fails first.
	srv := httptestNewNotFound(t)
	d := Downloader{BaseURL: srv, Client: http.DefaultClient}
	if _, err := d.DownloadAndVerify(context.Background(), "1.2.3", "linux", "amd64"); err == nil {
		t.Fatal("expected error when asset fetch fails, got nil")
	}
}

func TestDownloadAndVerify_ChecksumFetchFails(t *testing.T) {
	t.Parallel()
	version := "1.2.3"
	assetName := AssetName(version, "linux", "amd64")
	archive := makeTarGz(t, "ingitdb", []byte("bin"))

	mux := http.NewServeMux()
	mux.HandleFunc("/"+assetName, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	})
	// checksums endpoint intentionally absent → 404.
	srv := startServer(t, mux)

	d := Downloader{BaseURL: srv, Client: http.DefaultClient}
	if _, err := d.DownloadAndVerify(context.Background(), version, "linux", "amd64"); err == nil {
		t.Fatal("expected error when checksum fetch fails, got nil")
	}
}

func TestDownloadAndVerify_DefaultsRuntimeOSArch(t *testing.T) {
	t.Parallel()
	// Empty goos/goarch take runtime defaults; serving the matching asset path
	// exercises the default-assignment branches.
	version := "1.2.3"
	asset := AssetName(version, runtime.GOOS, runtime.GOARCH)
	var archive []byte
	if runtime.GOOS == "windows" {
		archive = makeZip(t, "ingitdb.exe", []byte("bin"))
	} else {
		archive = makeTarGz(t, "ingitdb", []byte("bin"))
	}
	baseURL := serveRelease(t, runtime.GOOS, asset, archive, sha256Hex(archive))
	d := Downloader{BaseURL: baseURL, Client: http.DefaultClient}
	path, err := d.DownloadAndVerify(context.Background(), version, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
}

func TestFetch_RequestError(t *testing.T) {
	t.Parallel()
	// A control character in the URL makes http.NewRequestWithContext fail.
	d := Downloader{Client: http.DefaultClient}
	if _, err := d.fetch(context.Background(), "http://example.com/\x7f", "asset"); err == nil {
		t.Fatal("expected request build error, got nil")
	}
}

func TestFetch_DoError(t *testing.T) {
	t.Parallel()
	// No server listening at this address → client.Do fails.
	d := Downloader{Client: &http.Client{}}
	if _, err := d.fetch(context.Background(), "http://127.0.0.1:1", "asset"); err == nil {
		t.Fatal("expected transport error, got nil")
	}
}

func TestDownloadAndVerify_DefaultBaseURLAndClient(t *testing.T) {
	t.Parallel()
	// Empty BaseURL and nil Client exercise the default-assignment branches
	// (versioned GitHub download directory, http.DefaultClient); a cancelled
	// context makes client.Do fail immediately without real network.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := Downloader{}
	if _, err := d.DownloadAndVerify(ctx, "1.2.3", "linux", "amd64"); err == nil {
		t.Fatal("expected error with cancelled context, got nil")
	}
}

// --- download.go: findChecksum branches ---

func TestFindChecksum_SkipsBlankAndMalformedLines(t *testing.T) {
	t.Parallel()
	checksums := []byte("\n  \nonlyonefield\nabc def ghi\ndeadbeef  ingitdb_x.tar.gz\n")
	got, err := findChecksum(checksums, "ingitdb_x.tar.gz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "deadbeef" {
		t.Fatalf("findChecksum = %q, want deadbeef", got)
	}
}

func TestFindChecksum_ScannerError(t *testing.T) {
	t.Parallel()
	// A single line longer than bufio.Scanner's max token size triggers
	// bufio.ErrTooLong via sc.Err().
	big := make([]byte, 80*1024)
	for i := range big {
		big[i] = 'a'
	}
	if _, err := findChecksum(big, "whatever"); err == nil {
		t.Fatal("expected scanner error for oversized line, got nil")
	}
}

// --- download.go: extract error branches ---

func TestExtractFromTarGz_BadGzip(t *testing.T) {
	t.Parallel()
	if _, err := extractFromTarGz([]byte("not a gzip"), "ingitdb"); err == nil {
		t.Fatal("expected gzip error, got nil")
	}
}

func TestExtractFromTarGz_BadTar(t *testing.T) {
	t.Parallel()
	// Valid gzip wrapping invalid tar bytes → tr.Next returns a non-EOF error.
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, _ = gw.Write([]byte("this is definitely not a valid tar stream payload"))
	_ = gw.Close()
	if _, err := extractFromTarGz(buf.Bytes(), "ingitdb"); err == nil {
		t.Fatal("expected tar read error, got nil")
	}
}

func TestExtractFromTarGz_NotFound(t *testing.T) {
	t.Parallel()
	archive := makeTarGz(t, "other-file", []byte("x"))
	if _, err := extractFromTarGz(archive, "ingitdb"); err == nil {
		t.Fatal("expected not-found error, got nil")
	}
}

func TestExtractFromZip_BadZip(t *testing.T) {
	t.Parallel()
	if _, err := extractFromZip([]byte("not a zip"), "ingitdb.exe"); err == nil {
		t.Fatal("expected zip error, got nil")
	}
}

func TestExtractFromZip_NotFound(t *testing.T) {
	t.Parallel()
	archive := makeZip(t, "other.exe", []byte("x"))
	if _, err := extractFromZip(archive, "ingitdb.exe"); err == nil {
		t.Fatal("expected not-found error, got nil")
	}
}

func TestExtractFromZip_OpenError(t *testing.T) {
	t.Parallel()
	// Build a normal (stored) zip, then byte-patch the compression-method field
	// in both the local and central headers to an unregistered method. The
	// central directory still parses, so zip.NewReader succeeds, but f.Open()
	// fails with "unsupported compression algorithm".
	raw := makeZip(t, "ingitdb.exe", []byte("payload"))
	patchZipMethod(t, raw, 99)
	if _, err := extractFromZip(raw, "ingitdb.exe"); err == nil {
		t.Fatal("expected open error for unsupported compression, got nil")
	}
}

// patchZipMethod rewrites the 2-byte compression-method field (little-endian)
// in every local-file-header (PK\x03\x04, field at +8) and central-directory
// header (PK\x01\x02, field at +10) found in buf.
func patchZipMethod(t *testing.T, buf []byte, method uint16) {
	t.Helper()
	lo, hi := byte(method), byte(method>>8)
	for i := 0; i+4 <= len(buf); i++ {
		switch {
		case buf[i] == 'P' && buf[i+1] == 'K' && buf[i+2] == 3 && buf[i+3] == 4 && i+10 <= len(buf):
			buf[i+8], buf[i+9] = lo, hi
		case buf[i] == 'P' && buf[i+1] == 'K' && buf[i+2] == 1 && buf[i+3] == 2 && i+12 <= len(buf):
			buf[i+10], buf[i+11] = lo, hi
		}
	}
}

// --- download.go: writeTempBinary branches ---

func TestWriteTempBinary_CreateError(t *testing.T) {
	orig := createTempFunc
	t.Cleanup(func() { createTempFunc = orig })
	createTempFunc = func(string, string) (tempFile, error) {
		return nil, errors.New("no temp")
	}
	reader := strings.NewReader("x")
	if _, err := writeTempBinary(reader); err == nil {
		t.Fatal("expected create-temp error, got nil")
	}
}

func TestWriteTempBinary_CopyError(t *testing.T) {
	t.Parallel()
	if _, err := writeTempBinary(errReader{}); err == nil {
		t.Fatal("expected copy error, got nil")
	}
}

func TestWriteTempBinary_CloseError(t *testing.T) {
	orig := createTempFunc
	t.Cleanup(func() { createTempFunc = orig })
	createTempFunc = func(dir, pattern string) (tempFile, error) {
		f, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		return &closeErrFile{File: f}, nil
	}
	reader := strings.NewReader("x")
	if _, err := writeTempBinary(reader); err == nil {
		t.Fatal("expected close error, got nil")
	}
}

// --- release.go: LatestStableTag branches ---

func TestLatestStableTag_RequestError(t *testing.T) {
	t.Parallel()
	r := Resolver{BaseURL: "http://example.com/\x7f", Client: http.DefaultClient}
	if _, err := r.LatestStableTag(context.Background()); err == nil {
		t.Fatal("expected request build error, got nil")
	}
}

func TestLatestStableTag_DoError(t *testing.T) {
	t.Parallel()
	r := Resolver{BaseURL: "http://127.0.0.1:1", Client: &http.Client{}}
	if _, err := r.LatestStableTag(context.Background()); err == nil {
		t.Fatal("expected transport error, got nil")
	}
}

func TestLatestStableTag_DefaultURLAndClient(t *testing.T) {
	t.Parallel()
	// Empty BaseURL and nil Client exercise the default-assignment branches; a
	// cancelled context makes client.Do fail without real network.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := Resolver{}
	if _, err := r.LatestStableTag(ctx); err == nil {
		t.Fatal("expected error with cancelled context, got nil")
	}
}

func TestLatestStableTag_DecodeError(t *testing.T) {
	t.Parallel()
	srv := newReleasesServer(t, "{not json")
	r := Resolver{BaseURL: srv.URL, Client: srv.Client()}
	if _, err := r.LatestStableTag(context.Background()); err == nil {
		t.Fatal("expected decode error, got nil")
	}
}

func TestLatestStableTag_NoStableRelease(t *testing.T) {
	t.Parallel()
	srv := newReleasesServer(t, `[{"tag_name":"v1.0.0-rc.1","prerelease":true,"draft":false}]`)
	r := Resolver{BaseURL: srv.URL, Client: srv.Client()}
	if _, err := r.LatestStableTag(context.Background()); err == nil {
		t.Fatal("expected no-stable-release error, got nil")
	}
}

// --- release.go: splitVersion non-numeric component ---

func TestSplitVersion_NonNumeric(t *testing.T) {
	t.Parallel()
	core, pre := splitVersion("1.x.3-beta")
	if core != [3]int{1, 0, 3} {
		t.Fatalf("core = %v, want [1 0 3]", core)
	}
	if pre != "beta" {
		t.Fatalf("pre = %q, want beta", pre)
	}
}

// --- replace.go: ReplaceExecutable / stage / VerifyBinaryVersion branches ---

func TestReplaceExecutable_RenameError(t *testing.T) {
	orig := renameFunc
	t.Cleanup(func() { renameFunc = orig })
	renameFunc = func(string, string) error { return errors.New("rename fail") }

	dir := t.TempDir()
	target := filepath.Join(dir, "ingitdb")
	if err := os.WriteFile(target, []byte("orig"), 0o755); err != nil {
		t.Fatal(err)
	}
	newBin := filepath.Join(dir, "new")
	if err := os.WriteFile(newBin, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ReplaceExecutable(target, newBin); err == nil {
		t.Fatal("expected rename error, got nil")
	}
}

func TestReplaceExecutable_WindowsPath(t *testing.T) {
	origGoos := goosName
	t.Cleanup(func() { goosName = origGoos })
	goosName = "windows"

	dir := t.TempDir()
	target := filepath.Join(dir, "ingitdb.exe")
	if err := os.WriteFile(target, []byte("orig"), 0o755); err != nil {
		t.Fatal(err)
	}
	newBin := filepath.Join(dir, "new")
	if err := os.WriteFile(newBin, []byte("new bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ReplaceExecutable(target, newBin); err != nil {
		t.Fatalf("windows replace: %v", err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "new bytes" {
		t.Fatalf("target = %q, want %q", got, "new bytes")
	}
}

func TestReplaceExecutable_WindowsMoveAsideError(t *testing.T) {
	origGoos, origRename := goosName, renameFunc
	t.Cleanup(func() { goosName, renameFunc = origGoos, origRename })
	goosName = "windows"
	renameFunc = func(string, string) error { return errors.New("rename fail") }

	dir := t.TempDir()
	target := filepath.Join(dir, "ingitdb.exe")
	if err := os.WriteFile(target, []byte("orig"), 0o755); err != nil {
		t.Fatal(err)
	}
	newBin := filepath.Join(dir, "new")
	if err := os.WriteFile(newBin, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ReplaceExecutable(target, newBin); err == nil {
		t.Fatal("expected move-aside error, got nil")
	}
}

func TestReplaceExecutable_WindowsInstallError(t *testing.T) {
	origGoos, origRename := goosName, renameFunc
	t.Cleanup(func() { goosName, renameFunc = origGoos, origRename })
	goosName = "windows"
	// First rename (move aside) succeeds, second (install) fails, third
	// (restore) is best-effort.
	calls := 0
	renameFunc = func(oldp, newp string) error {
		calls++
		if calls == 2 {
			return errors.New("install fail")
		}
		return os.Rename(oldp, newp)
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "ingitdb.exe")
	if err := os.WriteFile(target, []byte("orig"), 0o755); err != nil {
		t.Fatal(err)
	}
	newBin := filepath.Join(dir, "new")
	if err := os.WriteFile(newBin, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ReplaceExecutable(target, newBin); err == nil {
		t.Fatal("expected install error, got nil")
	}
}

func TestStage_CreateTempError(t *testing.T) {
	orig := stageCreateTmp
	t.Cleanup(func() { stageCreateTmp = orig })
	stageCreateTmp = func(string, string) (tempFile, error) { return nil, errors.New("no temp") }

	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := stage(dir, src); err == nil {
		t.Fatal("expected create-temp error, got nil")
	}
}

func TestStage_CopyError(t *testing.T) {
	t.Parallel()
	// src is a directory → io.Copy from a directory read fails.
	dir := t.TempDir()
	if _, err := stage(dir, dir); err == nil {
		t.Fatal("expected copy error reading a directory, got nil")
	}
}

func TestStage_CloseError(t *testing.T) {
	orig := stageCreateTmp
	t.Cleanup(func() { stageCreateTmp = orig })
	stageCreateTmp = func(d, pattern string) (tempFile, error) {
		f, err := os.CreateTemp(d, pattern)
		if err != nil {
			return nil, err
		}
		return &closeErrFile{File: f}, nil
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := stage(dir, src); err == nil {
		t.Fatal("expected close error, got nil")
	}
}

func TestStage_ChmodError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on windows")
	}
	orig := stageCreateTmp
	t.Cleanup(func() { stageCreateTmp = orig })
	// Return a file whose Name points somewhere that no longer exists after
	// close, so os.Chmod(name) fails.
	stageCreateTmp = func(d, pattern string) (tempFile, error) {
		f, err := os.CreateTemp(d, pattern)
		if err != nil {
			return nil, err
		}
		return &removeOnCloseFile{File: f}, nil
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := stage(dir, src); err == nil {
		t.Fatal("expected chmod error, got nil")
	}
}

func TestVerifyBinaryVersion_RunError(t *testing.T) {
	t.Parallel()
	// Nonexistent path → exec fails to start.
	missing := filepath.Join(t.TempDir(), "nope")
	if err := VerifyBinaryVersion(missing, "1.0.0"); err == nil {
		t.Fatal("expected run error, got nil")
	}
}

// --- helpers ---

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read fail") }

// closeErrFile copies through to the real file for writes/name but returns an
// error from Close (after closing the underlying file so no fd leaks).
type closeErrFile struct{ *os.File }

func (c *closeErrFile) Close() error {
	_ = c.File.Close()
	return errors.New("close fail")
}

// removeOnCloseFile removes the underlying file on Close so a subsequent
// path-based os.Chmod fails.
type removeOnCloseFile struct{ *os.File }

func (r *removeOnCloseFile) Close() error {
	name := r.Name()
	err := r.File.Close()
	_ = os.Remove(name)
	return err
}

func httptestNewNotFound(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	})
	return startServer(t, mux)
}

func startServer(t *testing.T, h http.Handler) string {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL
}
