package asyncapi

import (
	"github.com/go-faster/errors"
	"github.com/go-faster/yaml"
)

// ChannelRef is either an inline Channel or a `$ref` to one (Multi Resource Object).
type ChannelRef struct {
	Ref string `json:"$ref,omitempty" yaml:"$ref,omitempty"`

	Channel `json:",inline" yaml:",inline"`
}

// ServerRef is a reference to a server.
type ServerRef struct {
	Ref string `json:"$ref,omitempty" yaml:"$ref,omitempty"`
}

// ParameterLocation is a runtime expression or a list of them.
type ParameterLocation []string

// UnmarshalYAML implements yaml.Unmarshaler, accepting a scalar or a list.
func (l *ParameterLocation) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		var single string
		if err := n.Decode(&single); err != nil {
			return err
		}
		if single != "" {
			*l = []string{single}
		}
		return nil
	case yaml.SequenceNode:
		var list []string
		if err := n.Decode(&list); err != nil {
			return err
		}
		*l = list
		return nil
	default:
		return errors.Errorf("location must be a string or a list")
	}
}

// ParameterRef is either an inline Parameter or a `$ref` to one.
type ParameterRef struct {
	Ref string `json:"$ref,omitempty" yaml:"$ref,omitempty"`

	Parameter `json:",inline" yaml:",inline"`
}

// Channel object.
//
// See https://www.asyncapi.com/docs/reference/specification/3.0.0#channelObject.
type Channel struct {
	// REQUIRED. The actual address a connection pretends to be connected to.
	Address string `json:"address,omitempty" yaml:"address,omitempty"`
	// A list of references to servers on which the channel is available.
	Servers []*ServerRef `json:"servers,omitempty" yaml:"servers,omitempty"`
	// A map of the messages that will be sent to this channel.
	Messages Messages `json:"messages,omitempty" yaml:"messages,omitempty"`
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
	// A list of traits to apply to the channel.
	Traits []*ChannelTrait `json:"traits,omitempty" yaml:"traits,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// Messages is the Multi Operation Message Object: a map of message names to
// (possibly referenced) messages.
type Messages map[string]*MessageRef

// Parameter object describes a parameter included in a channel address.
//
// See https://www.asyncapi.com/docs/reference/specification/3.0.0#parametersObject.
type Parameter struct {
	// An enumeration of allowed values.
	Enum []string `json:"enum,omitempty" yaml:"enum,omitempty"`
	// The default value this parameter will use if not provided.
	Default string `json:"default,omitempty" yaml:"default,omitempty"`
	// A verbose explanation of the parameter.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// A runtime expression or URI-template location the parameter value
	// should be taken from: a string or a list of strings per the spec.
	Location ParameterLocation `json:"location,omitempty" yaml:"location,omitempty"`
	// An array of examples of the parameter value.
	Examples []string `json:"examples,omitempty" yaml:"examples,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// ServerBindings is a map of protocol-specific server bindings.
type ServerBindings map[string]*ServerBinding

// ChannelBindings is a map of protocol-specific channel bindings.
type ChannelBindings map[string]*ChannelBinding

// ChannelBinding is a single protocol channel binding.
type ChannelBinding struct {
	// Binding version, e.g. "kafka 0.5.0".
	BindingVersion string `json:"bindingVersion,omitempty" yaml:"bindingVersion,omitempty"`
	// Fields holds raw protocol-specific binding fields (typed access happens in the parser).
	Fields map[string]yaml.Node `json:"-" yaml:"-"`

	Common Common `json:"-" yaml:",inline"`
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (b *ChannelBinding) UnmarshalYAML(n *yaml.Node) error {
	return decodeBinding(n, &b.BindingVersion, &b.Fields, &b.Common)
}

// ServerBinding is a single protocol server binding.
type ServerBinding struct {
	// Binding version, e.g. "kafka 0.5.0".
	BindingVersion string `json:"bindingVersion,omitempty" yaml:"bindingVersion,omitempty"`
	// Fields holds raw protocol-specific binding fields.
	Fields map[string]yaml.Node `json:"-" yaml:"-"`

	Common Common `json:"-" yaml:",inline"`
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (b *ServerBinding) UnmarshalYAML(n *yaml.Node) error {
	return decodeBinding(n, &b.BindingVersion, &b.Fields, &b.Common)
}

// OperationBindings is a map of protocol-specific operation bindings.
type OperationBindings map[string]*OperationBinding

// OperationBinding is a single protocol operation binding.
type OperationBinding struct {
	// Binding version.
	BindingVersion string `json:"bindingVersion,omitempty" yaml:"bindingVersion,omitempty"`
	// Fields holds raw protocol-specific binding fields.
	Fields map[string]yaml.Node `json:"-" yaml:"-"`

	Common Common `json:"-" yaml:",inline"`
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (b *OperationBinding) UnmarshalYAML(n *yaml.Node) error {
	return decodeBinding(n, &b.BindingVersion, &b.Fields, &b.Common)
}

// MessageBindings is a map of protocol-specific message bindings.
type MessageBindings map[string]*MessageBinding

// MessageBinding is a single protocol message binding.
type MessageBinding struct {
	// Binding version.
	BindingVersion string `json:"bindingVersion,omitempty" yaml:"bindingVersion,omitempty"`
	// Fields holds raw protocol-specific binding fields.
	Fields map[string]yaml.Node `json:"-" yaml:"-"`

	Common Common `json:"-" yaml:",inline"`
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (b *MessageBinding) UnmarshalYAML(n *yaml.Node) error {
	return decodeBinding(n, &b.BindingVersion, &b.Fields, &b.Common)
}

// decodeBinding decodes a binding object, splitting `bindingVersion` and the
// common fields from the protocol-specific payload.
func decodeBinding(n *yaml.Node, version *string, fields *map[string]yaml.Node, common *Common) error {
	var raw map[string]yaml.Node
	if err := n.Decode(&raw); err != nil {
		return err
	}
	m := map[string]yaml.Node{}
	for k, v := range raw {
		switch k {
		case "bindingVersion":
			if err := v.Decode(version); err != nil {
				return err
			}
		default:
			if len(k) > 0 && k[0] == 'x' && len(k) > 1 && k[1] == '-' {
				continue // specification extension
			}
			m[k] = v
		}
	}
	*fields = m
	// Decode common (extensions + locator) from the same node.
	type commonOnly struct {
		Common `json:"-" yaml:",inline"`
	}
	var c commonOnly
	if err := n.Decode(&c); err != nil {
		return err
	}
	*common = c.Common
	return nil
}
