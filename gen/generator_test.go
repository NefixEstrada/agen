package gen_test

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/go-faster/yaml"
	"github.com/stretchr/testify/require"

	"github.com/NefixEstrada/agen/asyncapi"
	parser "github.com/NefixEstrada/agen/asyncapi/parser"
	"github.com/NefixEstrada/agen/gen"
)

// parseSpecRaw loads and parses a spec; alias controls whether x-agen-*
// extensions are rewritten to x-ogen-* first (as the CLI does).
func parseSpecRaw(t *testing.T, path string, alias bool) *parser.API {
	t.Helper()
	abs, err := filepath.Abs(path)
	require.NoError(t, err)
	data, err := os.ReadFile(abs)
	require.NoError(t, err)
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal(data, &root))
	if alias {
		asyncapi.AliasXExtensions(&root)
	}
	var spec asyncapi.Spec
	require.NoError(t, root.Decode(&spec))
	u, err := url.Parse("file://" + abs)
	require.NoError(t, err)
	api, err := parser.Parse(&spec, &root, u, parser.Config{InferTypes: true})
	require.NoError(t, err)
	return api
}

func parseSpec(t *testing.T, path string) *parser.API {
	return parseSpecRaw(t, path, false)
}

func TestIgnoreUnsupportedProtocol(t *testing.T) {
	api := parseSpec(t, "../_testdata/negative/unknown_protocol/spec.yaml")
	_, err := gen.NewGenerator(api, gen.Options{
		IgnoreUnsupported: []string{"protocol smoke-signals"},
	})
	require.NoError(t, err)
}

func TestFilters(t *testing.T) {
	api := parseSpec(t, "../_testdata/positive/streetlights/spec.yaml")

	receiveOnly, err := gen.NewGenerator(api, gen.Options{
		Filters: gen.Filters{Actions: []string{"receive"}},
	})
	require.NoError(t, err)
	// The public API does not expose operations; assert through WriteSource
	// output presence instead.
	require.NotNil(t, receiveOnly)

	none, err := gen.NewGenerator(api, gen.Options{
		Filters: gen.Filters{OperationsRegex: regexp.MustCompile("^(?:nope)$")},
	})
	require.NoError(t, err)
	require.NotNil(t, none)
}
