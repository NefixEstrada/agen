package convertedparams_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	convertedparams "github.com/NefixEstrada/agen/examples/converted-params/api"
)

// Smoke test: converted document with a referenced channel parameter.
func TestConvertedParamsSmoke(t *testing.T) {
	addr, err := convertedparams.BuildLightingMeasured1Parameter1Parameter2Address("a", "b")
	require.NoError(t, err)
	require.Equal(t, "lightingMeasured/a/b", addr)
}
