# Generator extensions

AsyncAPI specification extensions are patterned fields that are always prefixed by `x-`.

agen deliberately mirrors [ogen's](https://ogen.dev/docs/spec/extensions/) extension family
for muscle memory: before parsing, every `x-agen-foo` key in the document tree is rewritten to
its `x-ogen-foo` equivalent, so the vendored ogen engine honors it. Both spellings work
(`x-agen-*` is the recommended one; raw `x-ogen-*` keys pass through untouched), and the
rewrite covers the whole document — `components` and `$ref` targets included. Unknown `x-*`
keys are ignored.

The one exception is `x-oapi-codegen-extra-tags`, honored under its
[oapi-codegen](https://github.com/deepmap/oapi-codegen) name verbatim (aliasing it would
produce `x-ogen-oapi-codegen-extra-tags`, which the engine does not recognize).

Supported extensions:

- [Custom type name](#custom-type-name) — `x-agen-name`
- [Custom field name](#custom-field-name) — `x-agen-properties`
- [Extra struct field tags](#extra-struct-field-tags) — `x-oapi-codegen-extra-tags`
- [Custom type](#custom-type) — `x-agen-type`
- [Time formats](#time-formats) — `x-agen-time-format`
- [Custom validators](#custom-validators) — `x-agen-validate`
- [Redis bindings](#redis-bindings-x-redis) — `x-redis`

The snippets below assume this document skeleton (elided everywhere else):

```yaml
asyncapi: 3.0.0
info: { title: Telemetry API, version: 1.0.0 }
servers:
  production: { host: redis.example.io:6379, protocol: redis }
```

## Custom type name

Optionally, the Go names of messages, operations, channels and payload schemas can be
specified by `x-agen-name`, for example:

```yaml
channels:
  telemetry:
    address: telemetry
    messages:
      reading:
        x-agen-name: DeviceReading    # message envelope
        payload: { $ref: '#/components/schemas/telemetryReading' }
operations:
  receiveTelemetry:
    x-agen-name: ConsumeTelemetry    # handler interface and its method
    action: receive
    channel: { $ref: '#/channels/telemetry' }
components:
  schemas:
    telemetryReading:
      x-agen-name: TelemetryReading  # payload type
      type: object
      properties:
        deviceId: { type: string }
      required: [deviceId]
```

Generated code:

```go
type DeviceReading struct {
	Payload TelemetryReading `json:"-"`
}

// Ref: #/components/schemas/telemetryReading
type TelemetryReading struct {
	DeviceId string `json:"deviceId"`
}

// ConsumeTelemetry handles the "receiveTelemetry" operation.
type ConsumeTelemetryHandler interface {
	ConsumeTelemetry(ctx context.Context, msg *DeviceReading) error
}
```

The value must be an exported Go identifier; anything else is a located generation error.
Renaming a `send` operation renames the client method accordingly.

## Custom field name

Optionally, struct field names of generated types can be specified by `x-agen-properties`,
for example:

```yaml
channels:
  telemetry:
    address: telemetry
    messages:
      reading:
        payload:
          type: object
          properties:
            device_id: { type: string }
            measured_at: { type: string, format: date-time }
          required: [device_id, measured_at]
          x-agen-properties:
            device_id: { name: DeviceID }
operations:
  receiveTelemetry:
    action: receive
    channel: { $ref: '#/channels/telemetry' }
```

Generated code:

```go
type ReadingPayload struct {
	DeviceID   string    `json:"device_id"`
	MeasuredAt time.Time `json:"measured_at"`
}
```

Handy when the pascal-cased name is not what you want (`device_id` becomes `DeviceId` by
default; `DeviceID` requires either this extension or `generator.initialisms: true`).
Referenced properties must exist, and target names must be exported, unique Go identifiers —
violations are located errors.

## Extra struct field tags

Optionally, extra struct field tags can be added to a property with
`x-oapi-codegen-extra-tags`, for example:

```yaml
channels:
  pets:
    address: pets
    messages:
      created:
        payload:
          type: object
          properties:
            id: { type: integer, format: int64, x-oapi-codegen-extra-tags: { gorm: primaryKey, valid: customIdValidator } }
            name: { type: string }
          required: [id, name]
operations:
  petCreated:
    action: receive
    channel: { $ref: '#/channels/pets' }
```

Generated code:

```go
type CreatedPayload struct {
	ID   int64  `json:"id" gorm:"primaryKey" valid:"customIdValidator"`
	Name string `json:"name"`
}
```

## Custom type

Optionally, a schema can be mapped to an existing Go type by `x-agen-type`, for example:

```yaml
channels:
  telemetry:
    address: telemetry
    messages:
      reading:
        payload:
          type: object
          properties:
            batteryFor: { type: string, format: duration, x-agen-type: time.Duration }
          required: [batteryFor]
operations:
  receiveTelemetry:
    action: receive
    channel: { $ref: '#/channels/telemetry' }
```

Generated code:

```go
type ReadingPayload struct {
	BatteryFor time.Duration `json:"batteryFor"`
}
```

Supported values:

- a bare Go builtin (`uint8`, `int64`, `float64`, …), mapping the schema to that primitive;
- a type path such as `time.Duration` or `github.com/google/uuid.UUID`;
- a parenthesized path — `(github.com/foo/bar).Baz.Qux` — when the type name itself contains
  dots;
- a `*` prefix for pointer types.

agen loads the referenced package to detect which encoding interfaces the type implements
(`agen/runtime/codec` Marshaler/Unmarshaler, stdlib `encoding/json`,
`encoding.Text*Marshaler` or `encoding.Binary*Marshaler`), so the package must be resolvable
in the Go module agen runs in. The generated code imports it and encodes through the
detected interfaces.

## Time formats

Optionally, the wire layout of `time.Time` fields can be specified by `x-agen-time-format`.
The value is a Go time layout, passed verbatim to `time.Parse` and `Format` (see the
[reference time](https://pkg.go.dev/time#pkg-constants)), for example:

```yaml
channels:
  telemetry:
    address: telemetry
    messages:
      reading:
        payload:
          type: object
          properties:
            measuredAt:
              type: string
              format: date-time
              x-agen-time-format: "2006-01-02 15:04:05"
          required: [measuredAt]
operations:
  receiveTelemetry:
    action: receive
    channel: { $ref: '#/channels/telemetry' }
```

Generated code:

```go
MeasuredAt time.Time `json:"measuredAt"`

// …
v, err := json.DecodeTimeFormat(d, "2006-01-02 15:04:05")
// …
json.EncodeTimeFormat(e, s.MeasuredAt, "2006-01-02 15:04:05")
```

For Unix timestamps don't reach for this extension — they are JSON Schema string formats
instead: `{ type: string, format: unix }` maps to `time.Time` decoded from a JSON string of
Unix seconds (`json.DecodeStringUnixSeconds`).

## Custom validators

Optionally, pluggable validation can be attached to any schema by `x-agen-validate`: a map of
validator name to parameters, resolved at runtime through a registry, for example:

```yaml
channels:
  users:
    address: users
    messages:
      created:
        payload:
          type: object
          x-agen-validate:
            crossField: { separator: "-" }
          properties:
            email: { type: string, x-agen-validate: { corporate: true } }
            handle: { type: string, x-agen-validate: { reservedHandles: [admin, root] } }
          required: [email, handle]
operations:
  userCreated:
    action: receive
    channel: { $ref: '#/channels/users' }
```

Generated code (elided):

```go
func (s *CreatedPayload) Validate() error {
	// …
	// Object-level pluggable validation
	if err := validate.ValidateWith("crossField", s, map[string]interface{}{"separator": "-"}); err != nil {
		return errors.Wrap(err, "crossField")
	}
	// …
	if err := validate.Ogen("corporate", s.Email, true); err != nil {
		return errors.Wrap(err, "corporate")
	}
	if err := validate.Ogen("reservedHandles", s.Handle, []interface{}{"admin", "root"}); err != nil {
		return errors.Wrap(err, "reservedHandles")
	}
	// …
}
```

Property-level validators receive the field value through `validate.Ogen`; object-level ones
receive the whole generated struct through `validate.ValidateWith`. Both resolve through the
default registry of [`agen/runtime/validate`](../runtime/validate/ogen.go); register your
validators at startup:

```go
import "github.com/NefixEstrada/agen/runtime/validate"

func init() {
	_ = validate.RegisterValidator("corporate", func(value, params any) error {
		email := value.(string) // params == true here
		if !strings.HasSuffix(email, "@corp.example.com") {
			return errors.New("email is not corporate")
		}
		return nil
	})
}
```

An unregistered validator name fails validation at runtime with a `validate.ValidationError`
(`validator '<name>' not found`) rather than at startup.

> **Note:** for an object-level validator on a message payload, the generated message envelope
> also forwards the payload through the same validator (`validate.Ogen`), so it runs twice per
> decoded message — a quirk inherited from ogen's validators template.

## Redis bindings (`x-redis`)

The redis binding 0.2.0 draft ([asyncapi/bindings#313](https://github.com/asyncapi/bindings/pull/313))
defines the fields agen maps onto its Redis runtime: channel `type` and `maxLen`, operation
`consumerGroup`; server and message binding objects stay reserved. That version is **not published
yet**, so agen does not read `bindings.redis` — the fields are declared as the `x-redis`
specification extension on the channel or operation object instead:

```yaml
channels:
  events:
    address: 'app:events'
    x-redis: { type: stream, maxLen: 1, bindingVersion: '0.2.0' }
  notifications:
    address: 'app:notifications'
    x-redis: { type: pubsub, bindingVersion: '0.2.0' }
```

```yaml
operations:
  consumeEvent:
    action: receive
    channel:
      $ref: '#/channels/events'
    x-redis: { consumerGroup: my-service, bindingVersion: '0.2.0' }
```

(The values are the binding README's own example, its fields moved under `x-redis`;
`bindingVersion` is optional inside the extension and accepts `0.2.0`.)

Writing the draft fields under `bindings.redis` is a located parse error pointing at `x-redis`;
a fieldless `bindings.redis` — the published 0.1.0 stub, where every object was reserved — stays
valid and is ignored. When the binding is published, agen will read `bindings.redis` directly and
`x-redis` becomes a compatibility alias.

> **Note:** the example document above mixes `stream` and `pubsub` channels, which agen cannot
> generate yet — the runtime wires a single consumer and publisher per service, so a document must
> declare one type only (split it into two services, one per type, as `_testdata/positive/redis_binding`
> and `_testdata/positive/redis_pubsub` do).

What each field does:

- channel `type` — the Redis primitive of the channel. **Required on every channel** of a redis
  spec: a channel without it is a located generation error, since agen guesses no default mapping.
  All channels must agree: mixing `stream` and `pubsub` in one service is a located generation
  error. The declared type is baked into the constructors (`pubsub` ⇒ pub/sub, `stream` ⇒ streams)
  and the generated code exposes **no mode surface at all** — no `Mode` type, no constants, no
  `WithMode` — so the mapping cannot be misconfigured away.
- channel `maxLen` — the publisher XADDs with an exact `MAXLEN <maxLen>`, keeping the stream at
  that many entries. It must be ≥ 1 and MUST NOT appear on `pubsub` channels; all send channels
  must declare the same value (per-channel maxLen is future work).
- operation `consumerGroup` — the consumer group of the generated server, used by `XREADGROUP`.
  It is only valid on `receive` operations over `stream` channels, and **required** there: with
  no `WithGroup` existing, an undeclared group is a located generation error, not a default.
  `pubsub` receives need no group. The binding's groupless `XREAD` reading is not implemented.

Violations (missing or invalid `type`, `maxLen` on pubsub, `consumerGroup` on a send operation or a
pubsub channel, unknown fields, unsupported `bindingVersion`) are located parse errors.

## Planned extensions

- Per-channel redis wiring: mixed `maxLen` or `consumerGroup` declarations and groupless `XREAD`
  reads need per-channel consumers/publishers in the generated code; today heterogeneous
  declarations are generation errors and pub/sub receives need no group (see `DESIGN.md` §6.5).
