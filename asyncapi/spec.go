// Package asyncapi represents the raw AsyncAPI 3.x document object model (DOM),
// 1:1 with the specification. Every object carries specification extensions and
// a location locator, so downstream stages can report precise errors.
//
// See https://www.asyncapi.com/docs/reference/specification/3.0.0.
package asyncapi

import (
	"github.com/ogen-go/ogen/jsonschema"
)

type (
	// Num represents JSON number.
	Num = jsonschema.Num
	// Extensions is a map of specification extensions (`x-*` keys).
	Extensions = jsonschema.Extensions
	// Common is embedded into every DOM object to provide extensions and locator.
	Common = jsonschema.OpenAPICommon
)

// Spec is the AsyncAPI document root.
//
// See https://www.asyncapi.com/docs/reference/specification/3.0.0#asyncapiObject.
type Spec struct {
	// REQUIRED. This string MUST be the version number of the AsyncAPI
	// Specification that the document uses.
	AsyncAPI string `json:"asyncapi" yaml:"asyncapi"`
	// Unique string representing the application.
	ID string `json:"id,omitempty" yaml:"id,omitempty"`
	// REQUIRED. Provides metadata about the API.
	Info Info `json:"info" yaml:"info"`
	// Default content type to use when encoding/decoding a message.
	DefaultContentType string `json:"defaultContentType,omitempty" yaml:"defaultContentType,omitempty"`
	// Information about the servers.
	Servers map[string]*Server `json:"servers,omitempty" yaml:"servers,omitempty"`
	// The channels used by the application.
	Channels map[string]*Channel `json:"channels,omitempty" yaml:"channels,omitempty"`
	// The operations this application performs.
	Operations map[string]*Operation `json:"operations,omitempty" yaml:"operations,omitempty"`
	// An element to hold various reusable objects.
	Components *Components `json:"components,omitempty" yaml:"components,omitempty"`
	// A list of tags used by the specification with additional metadata.
	Tags Tags `json:"tags,omitempty" yaml:"tags,omitempty"`
	// Additional external documentation.
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// Info provides metadata about the API.
//
// See https://www.asyncapi.com/docs/reference/specification/3.0.0#infoObject.
type Info struct {
	// REQUIRED. The title of the application.
	Title string `json:"title" yaml:"title"`
	// REQUIRED. Provides the version of the application API.
	Version string `json:"version" yaml:"version"`
	// A short description of the application.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// A URL to the Terms of Service for the API.
	TermsOfService string `json:"termsOfService,omitempty" yaml:"termsOfService,omitempty"`
	// Contact information for the exposed API.
	Contact *Contact `json:"contact,omitempty" yaml:"contact,omitempty"`
	// License information for the exposed API.
	License *License `json:"license,omitempty" yaml:"license,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// Contact information.
type Contact struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	URL   string `json:"url,omitempty" yaml:"url,omitempty"`
	Email string `json:"email,omitempty" yaml:"email,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// License information.
type License struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	URL  string `json:"url,omitempty" yaml:"url,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// ExternalDocumentation references external resource.
type ExternalDocumentation struct {
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// REQUIRED. URL for the target documentation.
	URL string `json:"url" yaml:"url"`

	Common Common `json:"-" yaml:",inline"`
}

// Tag object.
type Tag struct {
	// REQUIRED. The name of the tag.
	Name string `json:"name" yaml:"name"`
	// A description for the tag.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Additional external documentation.
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`

	Common Common `json:"-" yaml:",inline"`
}

// Tags is a list of tags.
type Tags []Tag
