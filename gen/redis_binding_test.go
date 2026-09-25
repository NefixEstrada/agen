package gen_test

import (
	"net/url"
	"testing"

	"github.com/go-faster/yaml"
	"github.com/stretchr/testify/require"

	"github.com/NefixEstrada/agen/asyncapi"
	parser "github.com/NefixEstrada/agen/asyncapi/parser"
	"github.com/NefixEstrada/agen/gen"
)

// parseInline parses an inline spec string the way the CLI does (extension
// aliasing included).
func parseInline(t *testing.T, src string) *parser.API {
	t.Helper()
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(src), &root))
	asyncapi.AliasXExtensions(&root)
	var spec asyncapi.Spec
	require.NoError(t, root.Decode(&spec))
	api, err := parser.Parse(&spec, &root, &url.URL{}, parser.Config{InferTypes: true})
	require.NoError(t, err)
	return api
}

// TestRedisBindingDefaults asserts that the x-redis extension fields (draft
// binding 0.2.0) are baked into the generated wiring: consumerGroup becomes
// the server group, maxLen the publisher trim length, and no mode or group
// machinery is exposed at all.
func TestRedisBindingDefaults(t *testing.T) {
	fs := generateTo(t, "../_testdata/positive/redis_binding/spec.yaml", gen.Options{})
	out := joined(t, fs)

	// The declared consumerGroup and maxLen are baked into the constructors.
	require.Contains(t, out, `Group:`)
	require.Contains(t, out, `"my-service"`)
	require.Contains(t, out, "MaxLen: 1,")
	// No mode or group surface exists: the spec is the single source.
	require.NotContains(t, out, "func WithMode(")
	require.NotContains(t, out, "func WithGroup(")
	require.NotContains(t, out, "type modeOption struct")
	require.NotContains(t, out, "type Mode =")
	require.NotContains(t, out, "ModeStreams")
	require.NotContains(t, out, "ModePubSub")
}

// TestRedisPubSubBindingDefaults asserts the pubsub half of the binding
// example: the constructors run in pub/sub mode and no group is wired.
func TestRedisPubSubBindingDefaults(t *testing.T) {
	fs := generateTo(t, "../_testdata/positive/redis_pubsub/spec.yaml", gen.Options{})
	out := joined(t, fs)

	require.Contains(t, out, "Mode: redisruntime.ModePubSub,")
	require.NotContains(t, out, `Group: "`)
	require.NotContains(t, out, "func WithMode(")
	require.NotContains(t, out, "func WithGroup(")
	require.NotContains(t, out, "type Mode =")
}

// TestRedisTypeRequired asserts that a redis-wired channel without x-redis
// explodes: agen guesses no default mapping.
func TestRedisTypeRequired(t *testing.T) {
	spec := `asyncapi: 3.0.0
info: { title: T, version: 1.0.0 }
servers: { production: { host: redis.example.io:6379, protocol: redis } }
channels:
  events:
    address: app:events
    messages: { event: { payload: { type: string } } }
operations:
  consumeEvent:
    action: receive
    channel: { $ref: '#/channels/events' }
`
	api := parseInline(t, spec)
	_, err := gen.NewGenerator(api, gen.Options{})
	require.Error(t, err)
	require.Contains(t, err.Error(), `channel "events": x-redis.type is required (stream or pubsub)`)
}

// TestRedisConsumerGroupRequired asserts that a streams-mode receive
// operation without consumerGroup explodes: there is no WithGroup to
// compensate anymore.
func TestRedisConsumerGroupRequired(t *testing.T) {
	spec := `asyncapi: 3.0.0
info: { title: T, version: 1.0.0 }
servers: { production: { host: redis.example.io:6379, protocol: redis } }
channels:
  events:
    address: app:events
    x-redis: { type: stream }
    messages: { event: { payload: { type: string } } }
operations:
  consumeEvent:
    action: receive
    channel: { $ref: '#/channels/events' }
`
	api := parseInline(t, spec)
	_, err := gen.NewGenerator(api, gen.Options{})
	require.Error(t, err)
	require.Contains(t, err.Error(), `operation "consumeEvent": x-redis.consumerGroup is required in streams mode`)
}

// TestRedisMixedBindingTypes asserts that declaring both stream and pubsub
// channels fails generation: the runtime wires a single consumer/publisher.
func TestRedisMixedBindingTypes(t *testing.T) {
	spec := `asyncapi: 3.0.0
info: { title: T, version: 1.0.0 }
servers: { production: { host: redis.example.io:6379, protocol: redis } }
channels:
  events:
    address: app:events
    x-redis: { type: stream }
    messages: { event: { payload: { type: string } } }
  notifications:
    address: app:notifications
    x-redis: { type: pubsub }
    messages: { note: { payload: { type: string } } }
operations:
  consumeEvent:
    action: receive
    channel: { $ref: '#/channels/events' }
    x-redis: { consumerGroup: my-service }
  consumeNotification:
    action: receive
    channel: { $ref: '#/channels/notifications' }
`
	api := parseInline(t, spec)
	_, err := gen.NewGenerator(api, gen.Options{})
	require.Error(t, err)
	require.Contains(t, err.Error(), `channel "notifications" declares redis type "pubsub" but "stream" was declared elsewhere`)
}
