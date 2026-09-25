package gen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/NefixEstrada/agen/gen"
)

// TestNegativeCorpus walks every case under _testdata/negative that declares
// itself as a codegen-stage failure (stage.txt == "gen"): the spec must parse
// (if the error is related to the parser, the case belongs to the parser
// package testdata) and code generation must fail with the error from
// error.txt.
func TestNegativeCorpus(t *testing.T) {
	cases, err := filepath.Glob("../_testdata/negative/*/spec.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, cases)

	for _, path := range cases {
		path := path
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			t.Parallel()
			dir := filepath.Dir(path)

			stage, err := os.ReadFile(filepath.Join(dir, "stage.txt"))
			require.NoError(t, err)
			if s := strings.TrimSpace(string(stage)); s != "gen" {
				t.Skipf("case belongs to stage %s", s)
			}

			want, err := os.ReadFile(filepath.Join(dir, "error.txt"))
			require.NoError(t, err)

			api := parseSpec(t, path)
			_, err = gen.NewGenerator(api, gen.Options{})
			require.Error(t, err)
			require.Contains(t, err.Error(), strings.TrimSpace(string(want)))
		})
	}
}
