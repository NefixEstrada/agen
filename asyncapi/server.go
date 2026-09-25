package asyncapi

import "github.com/go-faster/yaml"

// Server object.
//
// See https://www.asyncapi.com/docs/reference/specification/3.0.0#serverObject.
type Server struct {
	// Ref references a server in components, when set.
	Ref string `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	// REQUIRED. The host on which the server can be reached.
	Host string `json:"host" yaml:"host"`
	// REQUIRED. The protocol this server supports for communication.
	Protocol string `json:"protocol" yaml:"protocol"`
	// The version of the protocol used for communication.
	ProtocolVersion string `json:"protocolVersion,omitempty" yaml:"protocolVersion,omitempty"`
	// The pathname on which the server can be reached.
	Pathname string `json:"pathname,omitempty" yaml:"pathname,omitempty"`
	// A human-friendly title for the server.
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
	// A short summary of the server.
	Summary string `json:"summary,omitempty" yaml:"summary,omitempty"`
	// A longer description of the server.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// An array of security requirement objects.
	Security SecurityRequirements `json:"security,omitempty" yaml:"security,omitempty"`
	// A map of the bindings for this server.
	Bindings ServerBindings `json:"bindings,omitempty" yaml:"bindings,omitempty"`
	// A list of tags for logical server categorization.
	Tags Tags `json:"tags,omitempty" yaml:"tags,omitempty"`
	// Additional external documentation.
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// SecurityRequirements is a list of security requirement alternatives: to
// authorize an operation, only one of the requirement objects must be satisfied.
type SecurityRequirements []SecurityRequirement

// SecurityRequirement lists security schemes and their scopes, or is a
// reference to a security scheme (as used by the official AsyncAPI examples:
// `{$ref: '#/components/securitySchemes/x'}`).
type SecurityRequirement struct {
	// Ref references a security scheme, when set.
	Ref string `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	// Schemes maps security scheme names to the required scopes.
	Schemes map[string][]string `json:"-" yaml:"-"`
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (r *SecurityRequirement) UnmarshalYAML(n *yaml.Node) error {
	var raw map[string]yaml.Node
	if err := n.Decode(&raw); err != nil {
		return err
	}
	if ref, ok := raw["$ref"]; ok && len(raw) == 1 {
		return ref.Decode(&r.Ref)
	}
	r.Schemes = map[string][]string{}
	for name, scopes := range raw {
		var list []string
		switch scopes.Kind {
		case yaml.SequenceNode:
			if err := scopes.Decode(&list); err != nil {
				return err
			}
		case yaml.ScalarNode:
			// `scheme: scope` shorthand.
			var single string
			if err := scopes.Decode(&single); err != nil {
				return err
			}
			if single != "" {
				list = []string{single}
			}
		}
		r.Schemes[name] = list
	}
	return nil
}
