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

func TestFeatureOnlyConfig(t *testing.T) {
	// ogen-style configs carry only generator options; the spec comes from
	// the CLI arguments.
	dir := t.TempDir()
	path := filepath.Join(dir, "agen.yml")
	require.NoError(t, os.WriteFile(path, []byte("generator:\n  features:\n    enable: [fakes]\n"), 0o600))

	cfg, err := agenconfig.Load(path)
	require.NoError(t, err)
	require.Equal(t, []string{"fakes"}, cfg.Generator.Features.Enable)
}

func TestPackageNameDefault(t *testing.T) {
	cfg := &agenconfig.Config{}
	// The default package name is "api" (ogen behavior); configs or the
	// --package-name flag override it.
	require.Equal(t, "", cfg.Target.PackageName)
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
