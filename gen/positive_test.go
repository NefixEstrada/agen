package gen_test

import (
	"path/filepath"
	"testing"

	"github.com/ogen-go/ogen/gen/genfs"
	"github.com/stretchr/testify/require"

	"github.com/NefixEstrada/agen/gen"
)

// featureMatrix is the feature-set matrix every corpus spec must generate
// under, mirroring ogen's internal/integration sample_api variants: default
// features, everything enabled, subscriber-only and client-only outputs.
var featureMatrix = []struct {
	name    string
	options func(t *testing.T) gen.Options
}{
	{"Default", func(*testing.T) gen.Options { return gen.Options{} }},
	{"AllFeatures", func(t *testing.T) gen.Options {
		features, err := gen.BuildFeatures([]gen.Feature{gen.Fakes, gen.Unimplemented}, nil, false)
		require.NoError(t, err)
		return gen.Options{Features: features}
	}},
	{"SubscriberOnly", func(t *testing.T) gen.Options {
		features, err := gen.BuildFeatures(nil, []gen.Feature{gen.Publisher}, false)
		require.NoError(t, err)
		return gen.Options{Features: features}
	}},
	{"ClientOnly", func(t *testing.T) gen.Options {
		features, err := gen.BuildFeatures(nil, []gen.Feature{gen.Subscriber, gen.Middleware}, false)
		require.NoError(t, err)
		return gen.Options{Features: features}
	}},
}

// generateCorpus generates package "api" for every spec matching the patterns
// under every feature set, validating that the output parses under go/format
// (genfs.CheckFS).
func generateCorpus(t *testing.T, patterns ...string) {
	t.Helper()

	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		require.NoError(t, err)
		files = append(files, matches...)
	}
	require.NotEmpty(t, files)

	for _, file := range files {
		file := file
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			api := parseSpecRaw(t, file, true) // x-agen-* → x-ogen-*, as the CLI does.
			for _, combo := range featureMatrix {
				combo := combo
				t.Run(combo.name, func(t *testing.T) {
					t.Parallel()
					g, err := gen.NewGenerator(api, combo.options(t))
					require.NoError(t, err)
					require.NoError(t, g.WriteSource(genfs.CheckFS{}, "api"))
				})
			}
		})
	}
}

// TestPositiveCorpus walks every case under _testdata/positive.
func TestPositiveCorpus(t *testing.T) {
	generateCorpus(t, "../_testdata/positive/*/spec.yaml")
}

// TestExamplesCorpus generates the real-world documents under
// _testdata/examples, the same specs the examples module builds upon.
func TestExamplesCorpus(t *testing.T) {
	generateCorpus(t, "../_testdata/examples/*.yaml", "../_testdata/examples/*.json")
}
