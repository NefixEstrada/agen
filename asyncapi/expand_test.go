package asyncapi_test

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-faster/yaml"
	"github.com/stretchr/testify/require"

	"github.com/NefixEstrada/agen/asyncapi"
	parser "github.com/NefixEstrada/agen/asyncapi/parser"
)

func TestExpandRoundTrip(t *testing.T) {
	path, err := filepath.Abs("../_testdata/positive/streetlights/spec.yaml")
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var root yaml.Node
	require.NoError(t, yaml.Unmarshal(data, &root))
	asyncapi.AliasXExtensions(&root)
	require.NoError(t, asyncapi.Expand(&root))

	// The expanded document must not contain internal references anymore
	// (except cyclic ones, which this spec does not have).
	dump, err := yaml.Marshal(root.Content[0])
	require.NoError(t, err)
	require.NotContains(t, string(dump), "$ref")

	// And it must re-parse into the same semantic model.
	var spec asyncapi.Spec
	require.NoError(t, root.Decode(&spec))
	u, err := url.Parse("file://" + path)
	require.NoError(t, err)
	api, err := parser.Parse(&spec, &root, u, parser.Config{InferTypes: true})
	require.NoError(t, err)
	require.Len(t, api.Operations, 2)
}
