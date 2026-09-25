package gen_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ogen-go/ogen/gen/genfs"

	"github.com/NefixEstrada/agen/gen"
)

// TestPositiveCorpus walks every case under _testdata/positive, generates a
// package in memory and validates the output parses under go/format
// (genfs.CheckFS).
func TestPositiveCorpus(t *testing.T) {
	cases := []string{
		"../_testdata/positive/streetlights",
	}
	for _, dir := range cases {
		dir := dir
		t.Run(dir, func(t *testing.T) {
			api := parseSpec(t, dir+"/spec.yaml")
			g, err := gen.NewGenerator(api, gen.Options{})
			require.NoError(t, err)

			fs := genfs.CheckFS{}
			require.NoError(t, g.WriteSource(fs, "api"))
		})
	}
}
