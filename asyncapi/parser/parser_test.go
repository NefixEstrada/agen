package parser_test

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-faster/errors"
	"github.com/go-faster/yaml"
	"github.com/stretchr/testify/require"

	"github.com/ogen-go/ogen/location"

	"github.com/NefixEstrada/agen/asyncapi"
	parser "github.com/NefixEstrada/agen/asyncapi/parser"
)

func parseFile(t *testing.T, path string) (*parser.API, error) {
	t.Helper()
	abs, err := filepath.Abs(path)
	require.NoError(t, err)
	data, err := os.ReadFile(abs)
	require.NoError(t, err)
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal(data, &root))
	var spec asyncapi.Spec
	require.NoError(t, root.Decode(&spec))
	u, err := url.Parse("file://" + abs)
	require.NoError(t, err)
	return parser.Parse(&spec, &root, u, parser.Config{
		InferTypes: true,
		RootFile:   location.NewFile(abs, abs, data),
	})
}

func TestNegative(t *testing.T) {
	cases, err := filepath.Glob("../../_testdata/negative/*/spec.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, cases)

	for _, path := range cases {
		path := path
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			// Some cases exercise later stages (e.g. codegen), not the parser.
			if stage, err := os.ReadFile(filepath.Join(filepath.Dir(path), "stage.txt")); err == nil {
				if strings.TrimSpace(string(stage)) != "parser" {
					t.Skipf("case belongs to stage %s", strings.TrimSpace(string(stage)))
				}
			}
			want, err := os.ReadFile(filepath.Join(filepath.Dir(path), "error.txt"))
			require.NoError(t, err)

			_, err = parseFile(t, path)
			require.Error(t, err)
			require.Contains(t, err.Error(), strings.TrimSpace(string(want)))
		})
	}
}

func TestNegativeLocated(t *testing.T) {
	_, err := parseFile(t, "../../_testdata/negative/bad_action/spec.yaml")
	require.Error(t, err)

	// The error must carry a location for pretty printing.
	var locErr *parser.LocationError
	require.True(t, errors.As(err, &locErr), "expected location error, got: %+v", err)
	require.NotZero(t, locErr.Pos.Line)
}

func TestPositiveStreetlights(t *testing.T) {
	api, err := parseFile(t, "../../_testdata/positive/streetlights/spec.yaml")
	require.NoError(t, err)

	require.Equal(t, "3.0.0", api.Version)
	require.Equal(t, "application/json", api.DefaultContentType)
	require.Len(t, api.Operations, 2)

	byName := map[string]*parser.Operation{}
	for _, op := range api.Operations {
		byName[op.Name] = op
	}

	recv := byName["receiveLightMeasurement"]
	require.NotNil(t, recv)
	require.Equal(t, parser.ReceiveAction, recv.Action)
	require.NotNil(t, recv.Channel)
	require.Equal(t, "streetlights.{streetlightId}.light", recv.Channel.Address)
	require.Len(t, recv.Channel.Parameters, 1)
	require.Equal(t, []string{"1", "2", "3"}, recv.Channel.Parameters["streetlightId"].Enum)
	require.Len(t, recv.Messages, 1)
	msg := recv.Messages[0]
	require.Equal(t, "lightMeasured", msg.Name)
	require.NotNil(t, msg.Payload)
	require.NotNil(t, msg.Headers)

	send := byName["sendLightCommand"]
	require.NotNil(t, send)
	require.Equal(t, parser.SendAction, send.Action)
}
