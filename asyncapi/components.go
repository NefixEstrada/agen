package asyncapi

import (
	"github.com/ogen-go/ogen/jsonschema"
)

// Components object holds a set of reusable objects for different aspects of
// the AsyncAPI specification.
//
// See https://www.asyncapi.com/docs/reference/specification/3.0.0#componentsObject.
type Components struct {
	// An object to hold reusable Schema Objects.
	Schemas map[string]*jsonschema.RawSchema `json:"schemas,omitempty" yaml:"schemas,omitempty"`
	// An object to hold reusable Server Objects.
	Servers map[string]*Server `json:"servers,omitempty" yaml:"servers,omitempty"`
	// An object to hold reusable Channel Objects.
	Channels map[string]*Channel `json:"channels,omitempty" yaml:"channels,omitempty"`
	// An object to hold reusable Operation Objects.
	Operations map[string]*Operation `json:"operations,omitempty" yaml:"operations,omitempty"`
	// An object to hold reusable Message Objects.
	Messages map[string]*MessageRef `json:"messages,omitempty" yaml:"messages,omitempty"`
	// An object to hold reusable Security Scheme Objects.
	SecuritySchemes map[string]*SecurityScheme `json:"securitySchemes,omitempty" yaml:"securitySchemes,omitempty"`
	// An object to hold reusable Parameter Objects.
	Parameters map[string]*Parameter `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	// An object to hold reusable Correlation ID Objects.
	CorrelationIDs map[string]*CorrelationID `json:"correlationIds,omitempty" yaml:"correlationIds,omitempty"`
	// An object to hold reusable Operation Trait Objects.
	OperationTraits map[string]*OperationTrait `json:"operationTraits,omitempty" yaml:"operationTraits,omitempty"`
	// An object to hold reusable Channel Trait Objects.
	ChannelTraits map[string]*ChannelTrait `json:"channelTraits,omitempty" yaml:"channelTraits,omitempty"`
	// An object to hold reusable Message Trait Objects.
	MessageTraits map[string]*MessageTrait `json:"messageTraits,omitempty" yaml:"messageTraits,omitempty"`
	// An object to hold reusable Server Bindings Objects.
	ServerBindings map[string]*ServerBinding `json:"serverBindings,omitempty" yaml:"serverBindings,omitempty"`
	// An object to hold reusable Channel Bindings Objects.
	ChannelBindings map[string]*ChannelBindings `json:"channelBindings,omitempty" yaml:"channelBindings,omitempty"`
	// An object to hold reusable Operation Bindings Objects.
	OperationBindings map[string]*OperationBindings `json:"operationBindings,omitempty" yaml:"operationBindings,omitempty"`
	// An object to hold reusable Message Bindings Objects.
	MessageBindings map[string]*MessageBindings `json:"messageBindings,omitempty" yaml:"messageBindings,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// ChannelTrait defines a set of common fields that can be reused in channels.
type ChannelTrait struct {
	// A list of references to servers on which the channel is available.
	Servers []*ServerRef `json:"servers,omitempty" yaml:"servers,omitempty"`
	// A map of parameters included in the channel address.
	Parameters map[string]*ParameterRef `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	// A human-friendly title for the channel.
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
	// A short summary of the channel.
	Summary string `json:"summary,omitempty" yaml:"summary,omitempty"`
	// A longer description of the channel.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// A map of the bindings for this channel.
	Bindings ChannelBindings `json:"bindings,omitempty" yaml:"bindings,omitempty"`
	// A list of tags for logical channel categorization.
	Tags Tags `json:"tags,omitempty" yaml:"tags,omitempty"`
	// Reference to a channel trait (in components).
	Ref string `json:"$ref,omitempty" yaml:"$ref,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// SecurityScheme object defines a security scheme that can be used to secure
// the connection.
//
// See https://www.asyncapi.com/docs/reference/specification/3.0.0#securitySchemeObject.
type SecurityScheme struct {
	// REQUIRED. The type of the security scheme.
	Type string `json:"type" yaml:"type"`
	// A short description of the security scheme.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Name of the header, query or cookie parameter to be used.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// The location of the API key.
	In string `json:"in,omitempty" yaml:"in,omitempty"`
	// The URL to the authorization server.
	BearerFormat string `json:"bearerFormat,omitempty" yaml:"bearerFormat,omitempty"`
	// The name of the HTTP Authorization scheme.
	Scheme string `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	// An object holding the security flows information.
	Flows *SecuritySchemeFlows `json:"flows,omitempty" yaml:"flows,omitempty"`
	// OpenId Connect URL to discover OAuth2 configuration values.
	OpenIDConnectURL string `json:"openIdConnectUrl,omitempty" yaml:"openIdConnectUrl,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// SecuritySchemeFlows holds OAuth2 flow configurations.
type SecuritySchemeFlows struct {
	Implicit          *SecuritySchemeFlow `json:"implicit,omitempty" yaml:"implicit,omitempty"`
	Password          *SecuritySchemeFlow `json:"password,omitempty" yaml:"password,omitempty"`
	ClientCredentials *SecuritySchemeFlow `json:"clientCredentials,omitempty" yaml:"clientCredentials,omitempty"`
	AuthorizationCode *SecuritySchemeFlow `json:"authorizationCode,omitempty" yaml:"authorizationCode,omitempty"`
}

// SecuritySchemeFlow is a single OAuth2 flow configuration.
type SecuritySchemeFlow struct {
	// REQUIRED. The URL to be used for the flow.
	AuthorizationURL string `json:"authorizationUrl,omitempty" yaml:"authorizationUrl,omitempty"`
	// REQUIRED. The URL to be used for the token flow.
	TokenURL string `json:"tokenUrl,omitempty" yaml:"tokenUrl,omitempty"`
	// The URL to be used for obtaining refresh tokens.
	RefreshURL string `json:"refreshUrl,omitempty" yaml:"refreshUrl,omitempty"`
	// REQUIRED. The available scopes for the flow.
	Scopes map[string]string `json:"scopes,omitempty" yaml:"scopes,omitempty"`
}
