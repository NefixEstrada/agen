package parser_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ogen-go/ogen/location"
)

// TestLocationExcerpts renders located errors with the source listing, like
// the CLI does (§8 of the design).
func TestLocationExcerpts(t *testing.T) {
	cases := map[string]string{
		"bad_action.yaml":    "invalid action \"observe\"",
		"missing_param.yaml": "address parameter \"deviceId\" is not declared",
	}
	for name, wantMsg := range cases {
		name, wantMsg := name, wantMsg
		t.Run(name, func(t *testing.T) {
			path, err := filepath.Abs("../../_testdata/location/" + name)
			require.NoError(t, err)
			_, err = parseFile(t, path)
			require.Error(t, err)

			var buf bytes.Buffer
			ok := location.PrintPrettyError(&buf, false, err)
			require.True(t, ok, "error must be pretty-printable")
			out := buf.String()
			require.Contains(t, out, wantMsg)
			// The excerpt renders the offending source line with a pointer.
			require.Contains(t, out, "→")
			require.Contains(t, out, name)
			require.False(t, strings.Contains(out, "\x1b["), "no ANSI colors expected")
		})
	}
}
