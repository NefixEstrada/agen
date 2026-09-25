package convertedstreetlights_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	convertedstreetlights "github.com/NefixEstrada/agen/examples/converted-streetlights/api"
)

// Smoke test: a real 2.6 document upgraded with the official converter
// (including intentionally circular schemas) generates a usable package.
func TestConvertedStreetlightsSmoke(t *testing.T) {
	require.Equal(t, "lightingMeasured", convertedstreetlights.LightingMeasuredAddress)
	require.Equal(t, "turnOn", convertedstreetlights.TurnOnAddress)
}
