// Package parser provides a semantic parser for AsyncAPI 3.x documents: it
// resolves references, merges traits, normalizes bindings, links operations to
// channels and messages, and delegates all JSON Schema parsing to ogen's
// jsonschema engine.
package parser

import (
	"github.com/ogen-go/ogen/jsonschema"

	"github.com/NefixEstrada/agen/asyncapi"
)

// Ref is a JSON Reference.
type Ref = string

// API represents a parsed AsyncAPI 3.x document.
type API struct {
	// Version is the AsyncAPI version (3.0.0, 3.0.1 or 3.1.0).
	Version string
	// Info provides metadata about the API.
	Info asyncapi.Info
	// ID is the application identifier, if any.
	ID string
	// DefaultContentType is the default content type of messages.
	DefaultContentType string
	// Servers maps server names to servers.
	Servers map[string]*Server
	// Channels is the list of channels referenced by operations.
	Channels []*Channel
	// Operations is the list of parsed operations.
	Operations []*Operation
}

// Server represents a parsed server.
type Server struct {
	// Name is the server map key.
	Name string
	// Host is the host on which the server can be reached.
	Host string
	// Protocol this server supports (e.g. "redis", "kafka", "mqtt").
	Protocol string
	// ProtocolVersion is the version of the protocol.
	ProtocolVersion string
	// Pathname on which the server can be reached.
	Pathname string
	// Title is a human-friendly title.
	Title string
	// Summary is a short summary.
	Summary string
	// Description is a longer description.
	Description string
	// Security is a list of security requirement alternatives.
	Security []asyncapi.SecurityRequirement
	// Bindings are the server bindings, keyed by protocol.
	Bindings asyncapi.ServerBindings
	// Tags is a list of tags.
	Tags asyncapi.Tags

	Common asyncapi.Common
}

// Channel represents a parsed channel.
type Channel struct {
	// Name is the channel map key.
	Name string
	// Address is the channel address (may contain `{param}` templates).
	Address string
	// Title is a human-friendly title.
	Title string
	// Summary is a short summary.
	Summary string
	// Description is a longer description.
	Description string
	// Messages is the list of messages that can be sent to this channel.
	Messages []*Message
	// Parameters is a map of channel address parameters.
	Parameters map[string]*Parameter
	// Servers lists the names of servers this channel is available on.
	Servers []string
	// Bindings are the channel bindings, keyed by protocol.
	Bindings asyncapi.ChannelBindings
	// Tags is a list of tags.
	Tags asyncapi.Tags

	Common asyncapi.Common
}

// Parameter represents a parsed channel address parameter.
type Parameter struct {
	// Name is the parameter name (as it appears in the address template).
	Name string
	// Enum is the enumeration of allowed values, if any.
	Enum []string
	// Default is the default value.
	Default string
	// Description is a verbose explanation.
	Description string
	// Location is a runtime expression of where the value is taken from.
	Location []string

	Common asyncapi.Common
}

// Action is the operation action, from the application's perspective.
type Action string

// Supported operation actions.
const (
	SendAction    Action = "send"    // the application publishes.
	ReceiveAction Action = "receive" // the application consumes.
)

// Operation represents a parsed operation.
type Operation struct {
	// Name is the operation map key (the `operationId`-like identifier).
	Name string
	// Action is send or receive, from the application's perspective.
	Action Action
	// Channel is the channel this operation is performed on.
	Channel *Channel
	// Messages is the list of messages this operation sends/receives.
	Messages []*Message
	// Title is a human-friendly title.
	Title string
	// Summary is a short summary.
	Summary string
	// Description is a longer description.
	Description string
	// Security is a list of security requirement alternatives.
	Security []asyncapi.SecurityRequirement
	// Bindings are the operation bindings, keyed by protocol.
	Bindings asyncapi.OperationBindings
	// Reply describes the operation reply, if any.
	Reply *Reply
	// Tags is a list of tags.
	Tags asyncapi.Tags

	Common asyncapi.Common
}

// Reply describes a parsed operation reply.
type Reply struct {
	// Channel is the reply channel.
	Channel *Channel
	// Messages is the list of reply messages.
	Messages []*Message
	// Title is a human-friendly title.
	Title string
	// Summary is a short summary.
	Summary string
	// Description is a longer description.
	Description string

	Common asyncapi.Common
}

// Message represents a parsed message.
type Message struct {
	// SpecName is the message name in the spec: the messages map key or
	// components.messages key (e.g. "lightMeasured").
	SpecName string
	// Name is the message `name` field, if any; defaults to SpecName.
	Name string
	// Title is a human-friendly title.
	Title string
	// Summary is a short summary.
	Summary string
	// Description is a longer description.
	Description string
	// Payload is the parsed payload schema, nil if the message has none.
	Payload *jsonschema.Schema
	// Headers is the parsed headers schema, nil if the message has none.
	Headers *jsonschema.Schema
	// ContentType is the resolved content type of the payload.
	ContentType string
	// CorrelationID describes the correlation ID location, if any.
	CorrelationID *asyncapi.CorrelationID
	// Bindings are the message bindings, keyed by protocol.
	Bindings asyncapi.MessageBindings
	// Tags is a list of tags.
	Tags asyncapi.Tags

	Common asyncapi.Common
}
