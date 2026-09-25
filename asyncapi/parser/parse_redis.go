package parser

import (
	"github.com/go-faster/errors"
	"github.com/go-faster/yaml"

	"github.com/ogen-go/ogen/location"

	"github.com/NefixEstrada/agen/asyncapi"
)

// Redis channel binding types (redis binding 0.2.0 draft).
const (
	// RedisTypeStream maps the channel address to a Redis stream: XADD on
	// send, XREADGROUP on receive.
	RedisTypeStream = "stream"
	// RedisTypePubSub maps the channel address to a Redis pub/sub channel:
	// PUBLISH on send, SUBSCRIBE/PSUBSCRIBE on receive.
	RedisTypePubSub = "pubsub"
)

// RedisBindingVersion is the draft version of the redis binding whose fields
// agen understands; an omitted bindingVersion means "latest", which resolves
// to it.
const RedisBindingVersion = "0.2.0"

// RedisChannelBinding is the typed `x-redis` specification extension of
// channels: the fields of the draft redis binding 0.2.0
// (see https://github.com/asyncapi/bindings/pull/313). The binding is not
// published yet, so agen reads the fields from the extension, not from
// `bindings.redis`.
type RedisChannelBinding struct {
	// Type is the Redis primitive: "stream" or "pubsub". Required.
	Type string
	// MaxLen is the maximum number of entries kept in the stream: producers
	// trim with an exact `XADD ... MAXLEN <maxLen>`. Streams only; 0 = unset.
	MaxLen int64
	// BindingVersion is the declared binding version, "" when omitted.
	BindingVersion string
}

// RedisOperationBinding is the typed `x-redis` specification extension of
// operations: `consumerGroup` names the XREADGROUP group of a receive
// operation (draft redis binding 0.2.0).
type RedisOperationBinding struct {
	// ConsumerGroup names the consumer group used to read via XREADGROUP.
	// agen takes the group from configuration when it is omitted.
	ConsumerGroup string
	// BindingVersion is the declared binding version, "" when omitted.
	BindingVersion string
}

// xRedisExtension is the specification-extension spelling of the draft
// binding's fields.
const xRedisExtension = "x-redis"

// redisExtensionFields is the field set of an `x-redis` extension object,
// with the locator for located errors. bindingVersion is carried separately
// from the typed fields.
type redisExtensionFields struct {
	fields   map[string]yaml.Node
	version  string
	hasField bool // any typed field (not bindingVersion) is present
	loc      location.Locator
}

func (f *redisExtensionFields) has(key string) bool {
	_, ok := f.fields[key]
	return ok
}

func (f *redisExtensionFields) string(key string, dst *string) error {
	n, ok := f.fields[key]
	if !ok {
		return nil
	}
	var v string
	if err := n.Decode(&v); err != nil {
		return errors.Errorf("%s.%s must be a string", xRedisExtension, key)
	}
	*dst = v
	return nil
}

func (f *redisExtensionFields) integer(key string, dst *int64) error {
	n, ok := f.fields[key]
	if !ok {
		return nil
	}
	if n.Kind != yaml.ScalarNode || n.Tag != "!!int" {
		return errors.Errorf("%s.%s must be an integer", xRedisExtension, key)
	}
	var v int64
	if err := n.Decode(&v); err != nil {
		return errors.Errorf("%s.%s must be an integer", xRedisExtension, key)
	}
	*dst = v
	return nil
}

// checkUnknownFields rejects fields agen does not know, so typos fail loudly
// instead of silently changing the mapping.
func (f *redisExtensionFields) checkUnknownFields(known ...string) error {
	for k := range f.fields {
		if len(k) > 1 && k[0] == 'x' && k[1] == '-' {
			continue // nested specification extensions
		}
		found := false
		for _, k2 := range known {
			if k == k2 {
				found = true
				break
			}
		}
		if !found {
			return errors.Errorf("unknown field %q in %s", k, xRedisExtension)
		}
	}
	return nil
}

// checkBindingVersion accepts an omitted version ("latest") and the draft
// version the fields above come from.
func (f *redisExtensionFields) checkBindingVersion() error {
	if f.version == "" || f.version == RedisBindingVersion {
		return nil
	}
	return errors.Errorf("unsupported %s.bindingVersion %q: agen supports %q", xRedisExtension, f.version, RedisBindingVersion)
}

// xRedisExtensionFields decodes the `x-redis` extension of a channel or
// operation. A missing extension returns nil fields; bindingVersion is
// split from the typed fields.
func (p *parser) xRedisExtensionFields(
	extensions asyncapi.Extensions,
	file location.File,
	loc location.Locator,
) (*redisExtensionFields, error) {
	extension := extensionNode(extensions)
	if extension.IsZero() {
		return nil, nil
	}
	out := &redisExtensionFields{
		fields: map[string]yaml.Node{},
		loc:    loc.Field(xRedisExtension),
	}
	if extension.Kind != yaml.MappingNode {
		err := errors.Errorf("%s must be a mapping of redis binding fields", xRedisExtension)
		return nil, p.wrapLocation(file, out.loc, err)
	}
	var raw map[string]yaml.Node
	if err := extension.Decode(&raw); err != nil {
		err := errors.Errorf("%s must be a mapping of redis binding fields", xRedisExtension)
		return nil, p.wrapLocation(file, out.loc, err)
	}
	for k, v := range raw {
		if k == "bindingVersion" {
			if err := v.Decode(&out.version); err != nil {
				err := errors.Errorf("%s.bindingVersion must be a string", xRedisExtension)
				return nil, p.wrapLocation(file, out.loc, err)
			}
			continue
		}
		if len(k) > 1 && k[0] == 'x' && k[1] == '-' {
			continue // nested specification extensions
		}
		out.fields[k] = v
		out.hasField = true
	}
	return out, nil
}

// rejectUnpublishedBinding errors on `bindings.redis` objects carrying draft
// 0.2.0 fields (or the draft version marker): the binding version is not
// published yet, so those fields belong under the x-redis extension.
// Fieldless bindings — the published 0.1.0 stub, where every object was
// reserved — stay valid and are ignored.
func (p *parser) rejectUnpublishedBinding(
	fields map[string]yaml.Node,
	version string,
	file location.File,
	loc location.Locator,
) error {
	if len(fields) == 0 && version != RedisBindingVersion {
		return nil
	}
	err := errors.Errorf(
		"bindings.redis carries redis binding %s fields, but that version is not published yet: declare them as the %s specification extension",
		RedisBindingVersion, xRedisExtension,
	)
	return p.wrapLocation(file, loc.Field("bindings").Field("redis"), err)
}

// parseRedisChannelBinding decodes and validates the channel's `x-redis`
// extension. An extension that declares no typed fields parses as nil.
func (p *parser) parseRedisChannelBinding(
	bindings asyncapi.ChannelBindings,
	extensions asyncapi.Extensions,
	file location.File,
	loc location.Locator,
) (*RedisChannelBinding, error) {
	if b := bindings["redis"]; b != nil {
		if err := p.rejectUnpublishedBinding(b.Fields, b.BindingVersion, file, loc); err != nil {
			return nil, err
		}
	}
	f, err := p.xRedisExtensionFields(extensions, file, loc)
	if err != nil || f == nil {
		return nil, err
	}

	var out RedisChannelBinding
	if err := f.string("type", &out.Type); err != nil {
		return nil, p.wrapLocation(file, f.loc, err)
	}
	if err := f.integer("maxLen", &out.MaxLen); err != nil {
		return nil, p.wrapLocation(file, f.loc, err)
	}
	if err := f.checkUnknownFields("type", "maxLen"); err != nil {
		return nil, p.wrapLocation(file, f.loc, err)
	}
	if !f.hasField {
		// Fieldless extension: nothing to map.
		return nil, nil
	}
	out.BindingVersion = f.version
	if err := f.checkBindingVersion(); err != nil {
		return nil, p.wrapLocation(file, f.loc, err)
	}
	if !f.has("type") {
		err := errors.Errorf("%s.type is required (%s or %s)", xRedisExtension, RedisTypeStream, RedisTypePubSub)
		return nil, p.wrapLocation(file, f.loc, err)
	}
	if out.Type != RedisTypeStream && out.Type != RedisTypePubSub {
		err := errors.Errorf("%s.type must be %q or %q, got %q", xRedisExtension, RedisTypeStream, RedisTypePubSub, out.Type)
		return nil, p.wrapLocation(file, f.loc, err)
	}
	if f.has("maxLen") {
		if out.Type == RedisTypePubSub {
			err := errors.Errorf("%s.maxLen must not be present when type is %q", xRedisExtension, RedisTypePubSub)
			return nil, p.wrapLocation(file, f.loc, err)
		}
		if out.MaxLen < 1 {
			err := errors.Errorf("%s.maxLen must be greater than or equal to 1", xRedisExtension)
			return nil, p.wrapLocation(file, f.loc, err)
		}
	}
	return &out, nil
}

// parseRedisOperationBinding decodes and validates the operation's `x-redis`
// extension. As with channels, a fieldless extension parses as nil.
func (p *parser) parseRedisOperationBinding(
	bindings asyncapi.OperationBindings,
	extensions asyncapi.Extensions,
	file location.File,
	loc location.Locator,
) (*RedisOperationBinding, error) {
	if b := bindings["redis"]; b != nil {
		if err := p.rejectUnpublishedBinding(b.Fields, b.BindingVersion, file, loc); err != nil {
			return nil, err
		}
	}
	f, err := p.xRedisExtensionFields(extensions, file, loc)
	if err != nil || f == nil {
		return nil, err
	}

	var out RedisOperationBinding
	if err := f.string("consumerGroup", &out.ConsumerGroup); err != nil {
		return nil, p.wrapLocation(file, f.loc, err)
	}
	if err := f.checkUnknownFields("consumerGroup"); err != nil {
		return nil, p.wrapLocation(file, f.loc, err)
	}
	if !f.hasField {
		return nil, nil
	}
	out.BindingVersion = f.version
	if err := f.checkBindingVersion(); err != nil {
		return nil, p.wrapLocation(file, f.loc, err)
	}
	return &out, nil
}

// validateRedisOperationBinding enforces the draft 0.2.0 constraint that
// consumerGroup is only used by receive operations on stream channels.
func (p *parser) validateRedisOperationBinding(
	op *Operation, file location.File, loc location.Locator,
) error {
	if op.Redis == nil || op.Redis.ConsumerGroup == "" {
		return nil
	}
	if op.Action != ReceiveAction {
		err := errors.Errorf("%s.consumerGroup is only valid on receive operations", xRedisExtension)
		return p.wrapLocation(file, loc.Field(xRedisExtension), err)
	}
	if op.Channel == nil || op.Channel.Redis == nil || op.Channel.Redis.Type != RedisTypeStream {
		err := errors.Errorf("%s.consumerGroup requires a channel with redis type %q", xRedisExtension, RedisTypeStream)
		return p.wrapLocation(file, loc.Field(xRedisExtension), err)
	}
	return nil
}

// extensionNode returns the x-redis extension node, or the zero node.
func extensionNode(extensions asyncapi.Extensions) yaml.Node {
	if extensions == nil {
		return yaml.Node{}
	}
	return extensions[xRedisExtension]
}
