package parser_test

import (
	"net/url"
	"testing"

	"github.com/go-faster/errors"
	"github.com/go-faster/yaml"
	"github.com/stretchr/testify/require"

	"github.com/ogen-go/ogen/location"

	"github.com/NefixEstrada/agen/asyncapi"
	parser "github.com/NefixEstrada/agen/asyncapi/parser"
)

// redisSpec builds a one-channel, one-operation document with the given
// channel and operation binding blocks (indented fragments).
func redisSpec(action, channelBinding, operationBinding string) string {
	return `asyncapi: 3.0.0
info: { title: T, version: 1.0.0 }
servers: { production: { host: redis.example.io:6379, protocol: redis } }
channels:
  events:
    address: app:events
` + channelBinding + `    messages:
      event: { payload: { type: string } }
operations:
  op:
    action: ` + action + `
    channel: { $ref: '#/channels/events' }
` + operationBinding
}

func parseString(t *testing.T, src string) (*parser.API, error) {
	t.Helper()
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(src), &root))
	var spec asyncapi.Spec
	require.NoError(t, root.Decode(&spec))
	return parser.Parse(&spec, &root, &url.URL{}, parser.Config{
		InferTypes: true,
		RootFile:   location.NewFile("spec.yaml", "spec.yaml", []byte(src)),
	})
}

func TestRedisExtension(t *testing.T) {
	t.Run("channel and operation", func(t *testing.T) {
		api, err := parseString(t, redisSpec("receive", `
    x-redis: { type: stream, maxLen: 1, bindingVersion: '0.2.0' }
`, `
    x-redis: { consumerGroup: my-service, bindingVersion: '0.2.0' }
`))
		require.NoError(t, err)
		ch := api.Operations[0].Channel
		require.NotNil(t, ch.Redis)
		require.Equal(t, parser.RedisTypeStream, ch.Redis.Type)
		require.Equal(t, int64(1), ch.Redis.MaxLen)
		require.Equal(t, parser.RedisBindingVersion, ch.Redis.BindingVersion)
		require.NotNil(t, api.Operations[0].Redis)
		require.Equal(t, "my-service", api.Operations[0].Redis.ConsumerGroup)
	})

	t.Run("no extension", func(t *testing.T) {
		api, err := parseString(t, redisSpec("receive", "", ""))
		require.NoError(t, err)
		require.Nil(t, api.Operations[0].Channel.Redis)
		require.Nil(t, api.Operations[0].Redis)
	})

	t.Run("fieldless extension", func(t *testing.T) {
		api, err := parseString(t, redisSpec("receive", `
    x-redis: { bindingVersion: '0.2.0' }
`, ""))
		require.NoError(t, err)
		require.Nil(t, api.Operations[0].Channel.Redis)
	})

	t.Run("fieldless 0.1.0 binding stub stays valid", func(t *testing.T) {
		api, err := parseString(t, redisSpec("receive", `
    bindings:
      redis: { bindingVersion: '0.1.0' }
`, ""))
		require.NoError(t, err)
		require.Nil(t, api.Operations[0].Channel.Redis)
	})
}

// The 0.2.0 binding is a draft: its fields live under the x-redis extension
// until the binding is published, so `bindings.redis` carrying them is an
// error pointing there.
func TestRedisUnpublishedBindingRejected(t *testing.T) {
	t.Run("fields", func(t *testing.T) {
		_, err := parseString(t, redisSpec("receive", `
    bindings:
      redis: { type: stream, maxLen: 1 }
`, ""))
		require.Error(t, err)
		require.Contains(t, err.Error(), "that version is not published yet: declare them as the x-redis specification extension")
	})

	t.Run("operation fields", func(t *testing.T) {
		_, err := parseString(t, redisSpec("receive", `
    x-redis: { type: stream }
`, `
    bindings:
      redis: { consumerGroup: my-service }
`))
		require.Error(t, err)
		require.Contains(t, err.Error(), "declare them as the x-redis specification extension")
	})

	t.Run("draft version marker alone", func(t *testing.T) {
		_, err := parseString(t, redisSpec("receive", `
    bindings:
      redis: { bindingVersion: '0.2.0' }
`, ""))
		require.Error(t, err)
		require.Contains(t, err.Error(), "not published yet")
	})
}

func TestRedisChannelExtensionNegative(t *testing.T) {
	tests := []struct {
		name     string
		binding  string
		contains string
	}{
		{
			name:     "type missing",
			binding:  "    x-redis: { maxLen: 100 }\n",
			contains: "x-redis.type is required (stream or pubsub)",
		},
		{
			name:     "type invalid",
			binding:  "    x-redis: { type: queue }\n",
			contains: `x-redis.type must be "stream" or "pubsub", got "queue"`,
		},
		{
			name:     "maxLen with pubsub",
			binding:  "    x-redis: { type: pubsub, maxLen: 5 }\n",
			contains: `x-redis.maxLen must not be present when type is "pubsub"`,
		},
		{
			name:     "maxLen below one",
			binding:  "    x-redis: { type: stream, maxLen: 0 }\n",
			contains: "x-redis.maxLen must be greater than or equal to 1",
		},
		{
			name:     "maxLen not an integer",
			binding:  "    x-redis: { type: stream, maxLen: many }\n",
			contains: "x-redis.maxLen must be an integer",
		},
		{
			name:     "unknown field",
			binding:  "    x-redis: { type: stream, groupId: g }\n",
			contains: `unknown field "groupId" in x-redis`,
		},
		{
			name:     "unsupported version with fields",
			binding:  "    x-redis: { type: stream, bindingVersion: '9.9.9' }\n",
			contains: `unsupported x-redis.bindingVersion "9.9.9": agen supports "0.2.0"`,
		},
		{
			name:     "extension not a mapping",
			binding:  "    x-redis: stream\n",
			contains: "x-redis must be a mapping of redis binding fields",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseString(t, redisSpec("receive", tt.binding, ""))
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.contains)
		})
	}
}

func TestRedisOperationExtensionNegative(t *testing.T) {
	stream := "    x-redis: { type: stream }\n"
	pubsub := "    x-redis: { type: pubsub }\n"
	group := "    x-redis: { consumerGroup: my-service }\n"

	t.Run("consumerGroup on send", func(t *testing.T) {
		_, err := parseString(t, redisSpec("send", stream, group))
		require.Error(t, err)
		require.Contains(t, err.Error(), "x-redis.consumerGroup is only valid on receive operations")
	})

	t.Run("consumerGroup on pubsub channel", func(t *testing.T) {
		_, err := parseString(t, redisSpec("receive", pubsub, group))
		require.Error(t, err)
		require.Contains(t, err.Error(), `x-redis.consumerGroup requires a channel with redis type "stream"`)
	})
}

func TestRedisExtensionErrorsLocated(t *testing.T) {
	_, err := parseString(t, redisSpec("receive", "    x-redis: { type: queue }\n", ""))
	require.Error(t, err)

	var locErr *parser.LocationError
	require.True(t, errors.As(err, &locErr), "expected location error, got: %+v", err)
	require.NotZero(t, locErr.Pos.Line)
}
