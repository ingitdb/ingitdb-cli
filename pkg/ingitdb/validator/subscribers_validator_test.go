package validator

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadSubscribers(t *testing.T) {
	dir := t.TempDir()

	ingitdbDir := filepath.Join(dir, ".ingitdb")
	err := os.MkdirAll(ingitdbDir, 0755)
	require.NoError(t, err)

	content := `
subscribers:
  all-changes:
    name: "Notify backend"
    for:
      events:
        - created
        - updated
    webhooks:
      - name: Primary endpoint
        url: https://api.example.com/ingitdb-webhooks
        headers:
          Authorization: "Bearer TOKEN"
`
	err = os.WriteFile(filepath.Join(ingitdbDir, "subscribers.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	subs, err := ReadSubscribers(dir, ingitdb.NewReadOptions(ingitdb.Validate()))
	require.NoError(t, err)

	require.NotNil(t, subs["all-changes"])
	assert.Equal(t, "Notify backend", subs["all-changes"].Name)
	assert.Len(t, subs["all-changes"].For.Events, 2)
	assert.Equal(t, ingitdb.TriggerEventCreated, subs["all-changes"].For.Events[0])
	assert.Len(t, subs["all-changes"].Webhooks, 1)
	assert.Equal(t, "Primary endpoint", subs["all-changes"].Webhooks[0].Name)
	assert.Equal(t, "https://api.example.com/ingitdb-webhooks", subs["all-changes"].Webhooks[0].URL)
	assert.Equal(t, "Bearer TOKEN", subs["all-changes"].Webhooks[0].Headers["Authorization"])
}

func TestReadSubscribers_NotFound(t *testing.T) {
	dir := t.TempDir()
	subs, err := ReadSubscribers(dir, ingitdb.NewReadOptions(ingitdb.Validate()))
	require.NoError(t, err)
	assert.Empty(t, subs)
}

func TestReadSubscribers_InvalidNoFor(t *testing.T) {
	dir := t.TempDir()
	ingitdbDir := filepath.Join(dir, ".ingitdb")
	err := os.MkdirAll(ingitdbDir, 0755)
	require.NoError(t, err)

	content := `
subscribers:
  invalid-sub:
    name: "Missing for block and handlers"
`
	err = os.WriteFile(filepath.Join(ingitdbDir, "subscribers.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	_, err = ReadSubscribers(dir, ingitdb.NewReadOptions(ingitdb.Validate()))
	require.ErrorContains(t, err, "subscriber must have 'for' selector")
}

func TestReadSubscribers_InvalidNoHandler(t *testing.T) {
	dir := t.TempDir()
	ingitdbDir := filepath.Join(dir, ".ingitdb")
	err := os.MkdirAll(ingitdbDir, 0755)
	require.NoError(t, err)

	content := `
subscribers:
  test-sub:
    for:
      events:
        - created
`
	err = os.WriteFile(filepath.Join(ingitdbDir, "subscribers.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	_, err = ReadSubscribers(dir, ingitdb.NewReadOptions(ingitdb.Validate()))
	require.ErrorContains(t, err, "must have at least one handler")
}

func TestReadSubscribers_EmptyPath(t *testing.T) {
	dir := t.TempDir()
	ingitdbDir := filepath.Join(dir, ".ingitdb")
	err := os.MkdirAll(ingitdbDir, 0755)
	require.NoError(t, err)

	content := `
subscribers:
  all-changes:
    for:
      events:
        - created
    webhooks:
      - url: https://api.example.com/ingitdb-webhooks
`
	err = os.WriteFile(filepath.Join(ingitdbDir, "subscribers.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	ogDir := os.Getenv("PWD")
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(ogDir) }()

	subs, err := ReadSubscribers("", ingitdb.NewReadOptions(ingitdb.Validate()))
	require.NoError(t, err)

	require.NotNil(t, subs["all-changes"])
}

func TestReadSubscribers_PanicRecover(t *testing.T) {
	dir := t.TempDir()

	readFileMockPanic := func(string) ([]byte, error) {
		panic("test panic")
	}

	_, err := readSubscribers(dir, ingitdb.NewReadOptions(), readFileMockPanic)
	require.ErrorContains(t, err, "test panic")
}

func TestReadSubscribers_ReadFileError(t *testing.T) {
	dir := t.TempDir()

	readFileMockErr := func(string) ([]byte, error) {
		return nil, errors.New("test read error")
	}

	_, err := readSubscribers(dir, ingitdb.NewReadOptions(), readFileMockErr)
	require.ErrorContains(t, err, "failed to read subscribers config file: test read error")
}

func TestReadSubscribers_ParseError(t *testing.T) {
	dir := t.TempDir()
	ingitdbDir := filepath.Join(dir, ".ingitdb")
	err := os.MkdirAll(ingitdbDir, 0755)
	require.NoError(t, err)

	content := `
subscribers:
  - invalid-list-item-instead-of-map
`
	err = os.WriteFile(filepath.Join(ingitdbDir, "subscribers.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	_, err = ReadSubscribers(dir, ingitdb.NewReadOptions())
	require.ErrorContains(t, err, "failed to parse subscribers config file")
}
