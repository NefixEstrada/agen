package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	agenconfig "github.com/NefixEstrada/agen/internal/config"
)

func TestLoadStrict(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agen.yml")
	require.NoError(t, os.WriteFile(path, []byte(`
target:
  spec: ./spec.yaml
  dir: ./api
  package_name: myapi
parser:
  infer_types: true
  ignore_unsupported: [schema.not]
generator:
  features:
    enable: [fakes]
    disable: [middleware]
`), 0o600))

	cfg, err := agenconfig.Load(path)
	require.NoError(t, err)
	require.Equal(t, "myapi", cfg.Target.PackageName)
	require.Equal(t, []string{"schema.not"}, cfg.Parser.IgnoreUnsupported)
	require.Equal(t, "streams", cfg.Broker.RedisMode())

	// Relative paths resolve against the config file directory.
	require.Equal(t, filepath.Join(dir, "spec.yaml"), cfg.Target.Spec)
	require.Equal(t, filepath.Join(dir, "api"), cfg.Target.Dir)
}

func TestLoadUnknownFieldFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agen.yml")
	require.NoError(t, os.WriteFile(path, []byte(`
target:
  spec: ./spec.yaml
  pakage_name: typo
`), 0o600))

	_, err := agenconfig.Load(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "pakage_name")
}

func TestSpecRequired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agen.yml")
	require.NoError(t, os.WriteFile(path, []byte("target:\n  dir: ./api\n"), 0o600))

	_, err := agenconfig.Load(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "target.spec is required")
}

func TestPackageNameFromTitle(t *testing.T) {
	cfg := &agenconfig.Config{}
	require.Equal(t, "streetlightsapi", cfg.PackageName("Streetlights API"))
	require.Equal(t, "orders", cfg.PackageName("Orders"))
	require.Equal(t, "api", cfg.PackageName(""))
}

func TestAutoDiscover(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "a", "b")
	require.NoError(t, os.MkdirAll(sub, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agen.yml"), []byte("target: {}"), 0o600))

	p, ok := agenconfig.AutoDiscover(sub)
	require.True(t, ok)
	require.Equal(t, filepath.Join(dir, "agen.yml"), p)
}
