package ir

import "fmt"

// Operation is a pub/sub operation, from the application's perspective:
// send operations generate publisher methods, receive operations generate
// handler interfaces and subscriber wiring.
type Operation struct {
	// Name is the Go method name, e.g. "SendLightCommand".
	Name string
	// OperationID is the spec operation key, e.g. "sendLightCommand".
	OperationID string
	// Action is "send" or "receive".
	Action string
	// Description is a longer description.
	Description string
	// Deprecated is true when the operation is marked deprecated.
	Deprecated bool
	// Channel the operation is performed on.
	Channel *Channel
	// Messages is the list of message envelopes (1..N).
	Messages []*Message
	// SumType is set when the operation has multiple messages: a
	// type-based dispatch interface.
	SumType *Type
	// GoDoc returns operation godoc.
	GoDoc []string
}

// IsSend returns true if the application publishes on this operation.
func (o *Operation) IsSend() bool { return o.Action == "send" }

// IsReceive returns true if the application consumes on this operation.
func (o *Operation) IsReceive() bool { return o.Action == "receive" }

// GoName returns the operation name.
func (o *Operation) GoName() string { return o.Name }

// Type returns the operation argument type: the single message envelope or the
// multi-message sum interface.
func (o *Operation) Type() *Type {
	if o.SumType != nil {
		return o.SumType
	}
	if len(o.Messages) > 0 {
		return o.Messages[0].Type
	}
	return nil
}

// Channel is a message channel with an address (possibly templated).
type Channel struct {
	// Name is the channel key, e.g. "lightMeasured".
	Name string
	// GoName is the Go identifier, e.g. "LightMeasured".
	GoName string
	// Address is the channel address; may contain `{param}` templates.
	Address string
	// Description is a longer description.
	Description string
	// Params is the list of address template parameters (sorted by appearance
	// in the address).
	Params []*ChannelParam
}

// HasParams returns true if the address contains parameters.
func (c *Channel) HasParams() bool { return len(c.Params) > 0 }

// BuilderName returns the name of the generated address builder function.
func (c *Channel) BuilderName() string { return "Build" + c.GoName + "Address" }

// ChannelParam is an address template parameter.
type ChannelParam struct {
	// Name is the parameter name in the address template, e.g. "streetlightId".
	Name string
	// GoName is the Go identifier, e.g. "StreetlightId".
	GoName string
	// Enum is the list of allowed values, if any.
	Enum []string
	// Default is the default value, if any.
	Default string
	// Description is a longer description.
	Description string
}

// Message is a message envelope: headers + payload.
type Message struct {
	// Name is the Go type name of the envelope, e.g. "LightMeasured".
	Name string
	// SpecName is the message name in the spec, e.g. "lightMeasured".
	SpecName string
	// Title is a human-friendly title.
	Title string
	// Description is a longer description.
	Description string
	// Type is the envelope struct type.
	Type *Type
	// Headers is the headers schema type, nil if absent.
	Headers *Type
	// Payload is the payload schema type, nil if absent.
	Payload *Type
	// ContentType is the resolved content type of the payload.
	ContentType string
	// SumMarker is the marker method name when the message is a variant of a
	// multi-message dispatch interface.
	SumMarker string
}

// HasHeaders returns true if the message has a headers schema.
func (m *Message) HasHeaders() bool { return m.Headers != nil }

// HasPayload returns true if the message has a payload schema.
func (m *Message) HasPayload() bool { return m.Payload != nil }

// Server is a broker server entry.
type Server struct {
	// Name is the server key, e.g. "production".
	Name string
	// Host is the broker host (e.g. "redis.example.io:6379").
	Host string
	// Protocol is the server protocol (e.g. "redis").
	Protocol string
	// ProtocolVersion is the protocol version, if any.
	ProtocolVersion string
	// Description is a longer description.
	Description string
}

// String returns a human-readable representation.
func (s Server) String() string {
	return fmt.Sprintf("%s (%s://%s)", s.Name, s.Protocol, s.Host)
}

// ArgGo returns the Go type of the typed handler/publisher argument: a
// pointer to the envelope for single-message operations, the dispatch
// interface for multi-message operations.
func (o *Operation) ArgGo() string {
	if o.SumType != nil {
		return o.SumType.Name
	}
	if t := o.Type(); t != nil {
		return "*" + t.Name
	}
	return ""
}
