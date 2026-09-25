package asyncapi

// Operation object describes a publish or a subscribe operation.
//
// See https://www.asyncapi.com/docs/reference/specification/3.0.0#operationObject.
type Operation struct {
	// REQUIRED. The type of action: `send` or `receive`. From the application's
	// perspective.
	Action string `json:"action" yaml:"action"`
	// The channel on which this operation is performed: `{$ref}` or inline.
	Channel *ChannelRef `json:"channel,omitempty" yaml:"channel,omitempty"`
	// A list of messages that will be sent to or received from this operation.
	Messages Messages `json:"messages,omitempty" yaml:"messages,omitempty"`
	// A human-friendly title for the operation.
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
	// A short summary of the operation.
	Summary string `json:"summary,omitempty" yaml:"summary,omitempty"`
	// A longer description of the operation.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// An `operationId`-like identifier: the map key in `operations`.
	//
	// Kept here for convenience; not a spec field.
	// A list of security requirements.
	Security SecurityRequirements `json:"security,omitempty" yaml:"security,omitempty"`
	// A map of the bindings for this operation.
	Bindings OperationBindings `json:"bindings,omitempty" yaml:"bindings,omitempty"`
	// A list of traits to apply to the operation.
	Traits []*OperationTrait `json:"traits,omitempty" yaml:"traits,omitempty"`
	// A list of tags for logical operation categorization.
	Tags Tags `json:"tags,omitempty" yaml:"tags,omitempty"`
	// Additional external documentation.
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`
	// The reply to this operation.
	Reply *OperationReply `json:"reply,omitempty" yaml:"reply,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// OperationTrait defines a set of common fields that can be reused in operations.
type OperationTrait struct {
	// A human-friendly title for the operation.
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
	// A short summary of the operation.
	Summary string `json:"summary,omitempty" yaml:"summary,omitempty"`
	// A longer description of the operation.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// A list of security requirements.
	Security SecurityRequirements `json:"security,omitempty" yaml:"security,omitempty"`
	// A map of the bindings for this operation.
	Bindings OperationBindings `json:"bindings,omitempty" yaml:"bindings,omitempty"`
	// A list of tags for logical operation categorization.
	Tags Tags `json:"tags,omitempty" yaml:"tags,omitempty"`
	// Additional external documentation.
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`
	// Reference to an operation trait (in components).
	Ref string `json:"$ref,omitempty" yaml:"$ref,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// OperationReply object defines the reply to an operation.
//
// See https://www.asyncapi.com/docs/reference/specification/3.0.0#operationReplyObject.
type OperationReply struct {
	// REQUIRED. The channel on which the reply is sent: `{$ref}` or inline.
	Channel *ChannelRef `json:"channel" yaml:"channel"`
	// A list of messages that can be sent to the reply channel.
	Messages Messages `json:"messages,omitempty" yaml:"messages,omitempty"`
	// A human-friendly title for the operation reply.
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
	// A short summary of the operation reply.
	Summary string `json:"summary,omitempty" yaml:"summary,omitempty"`
	// A longer description of the operation reply.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// A list of traits to apply to the operation reply object.
	Traits []*OperationReplyTrait `json:"traits,omitempty" yaml:"traits,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// OperationReplyTrait object defines a set of common fields that can be reused
// in operation replies.
type OperationReplyTrait struct {
	// A human-friendly title for the operation reply.
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
	// A short summary of the operation reply.
	Summary string `json:"summary,omitempty" yaml:"summary,omitempty"`
	// A longer description of the operation reply.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Reference to an operation reply trait (in components).
	Ref string `json:"$ref,omitempty" yaml:"$ref,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}
