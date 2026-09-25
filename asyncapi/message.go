package asyncapi

import (
	"github.com/ogen-go/ogen/jsonschema"
)

// Message Object describes a message sent to or received from a channel.
//
// See https://www.asyncapi.com/docs/reference/specification/3.0.0#messageObject.
type Message struct {
	// A machine-friendly name for the message.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// A human-friendly title for the message.
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
	// A short summary of the message.
	Summary string `json:"summary,omitempty" yaml:"summary,omitempty"`
	// A longer description of the message.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Schema definition of the application headers.
	Headers *jsonschema.RawSchema `json:"headers,omitempty" yaml:"headers,omitempty"`
	// Definition of the message payload.
	Payload *jsonschema.RawSchema `json:"payload,omitempty" yaml:"payload,omitempty"`
	// Definition of the correlation ID object for the message.
	CorrelationID *CorrelationID `json:"correlationId,omitempty" yaml:"correlationId,omitempty"`
	// A string containing the content type of the message payload.
	ContentType string `json:"contentType,omitempty" yaml:"contentType,omitempty"`
	// The name of a message bindings.
	Bindings MessageBindings `json:"bindings,omitempty" yaml:"bindings,omitempty"`
	// A schema object defining the type of the message to be used
	// when a message is an event (as opposed to a command).
	SchemaFormat string `json:"schemaFormat,omitempty" yaml:"schemaFormat,omitempty"`
	// A list of tags for logical message categorization.
	Tags Tags `json:"tags,omitempty" yaml:"tags,omitempty"`
	// An array of examples of the message.
	Examples []*MessageExample `json:"examples,omitempty" yaml:"examples,omitempty"`
	// List of traits to apply to the message.
	Traits []*MessageTrait `json:"traits,omitempty" yaml:"traits,omitempty"`
	// Additional external documentation.
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// MessageTrait defines a set of common fields that can be reused in messages.
type MessageTrait struct {
	// A machine-friendly name for the message.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// A human-friendly title for the message.
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
	// A short summary of the message.
	Summary string `json:"summary,omitempty" yaml:"summary,omitempty"`
	// A longer description of the message.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Schema definition of the application headers.
	Headers *jsonschema.RawSchema `json:"headers,omitempty" yaml:"headers,omitempty"`
	// Definition of the correlation ID object for the message.
	CorrelationID *CorrelationID `json:"correlationId,omitempty" yaml:"correlationId,omitempty"`
	// A string containing the content type of the message payload.
	ContentType string `json:"contentType,omitempty" yaml:"contentType,omitempty"`
	// The name of a message bindings.
	Bindings MessageBindings `json:"bindings,omitempty" yaml:"bindings,omitempty"`
	// A schema object defining the type of the message to be used
	// when a message is an event (as opposed to a command).
	SchemaFormat string `json:"schemaFormat,omitempty" yaml:"schemaFormat,omitempty"`
	// A list of tags for logical message categorization.
	Tags Tags `json:"tags,omitempty" yaml:"tags,omitempty"`
	// An array of examples of the message.
	Examples []*MessageExample `json:"examples,omitempty" yaml:"examples,omitempty"`
	// Additional external documentation.
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`
	// Reference to a message trait (in components).
	Ref string `json:"$ref,omitempty" yaml:"$ref,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// MessageExample object represents an example of a message.
type MessageExample struct {
	Headers map[string]any `json:"headers,omitempty" yaml:"headers,omitempty"`
	Payload any            `json:"payload,omitempty" yaml:"payload,omitempty"`
	Name    string         `json:"name,omitempty" yaml:"name,omitempty"`
	Summary string         `json:"summary,omitempty" yaml:"summary,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// CorrelationID object defines the location and understanding of a correlation
// ID for a message.
type CorrelationID struct {
	// An optional description of the correlation ID.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// REQUIRED. A runtime expression that specifies the location of the
	// correlation ID.
	Location string `json:"location" yaml:"location"`
	// Reference to a correlation ID (in components).
	Ref string `json:"$ref,omitempty" yaml:"$ref,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// MessageRef is either an inline Message or a `$ref` to one.
//
// In AsyncAPI 3.x every map entry that accepts a message accepts a Multi
// Resource Object: `{$ref: ...}` or the object inline.
type MessageRef struct {
	Ref string `json:"$ref,omitempty" yaml:"$ref,omitempty"`

	Message `json:",inline" yaml:",inline"`
}
