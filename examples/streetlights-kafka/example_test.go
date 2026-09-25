package streetlightskafka_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	streetlightskafka "github.com/NefixEstrada/agen/examples/streetlights-kafka/api"
)

// Smoke test: the canonical AsyncAPI streetlights Kafka document generates a
// usable typed package (compilation itself is the main drift check).
func TestStreetlightsKafkaSmoke(t *testing.T) {
	addr, err := streetlightskafka.BuildLightingMeasuredAddress("1")
	require.NoError(t, err)
	require.Equal(t, "smartylighting.streetlights.1.0.event.1.lighting.measured", addr)

	require.NotEmpty(t, streetlightskafka.Servers)
	var _ streetlightskafka.Handler = streetlightskafka.UnimplementedHandler{}
}
