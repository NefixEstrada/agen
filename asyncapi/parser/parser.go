package parser

import (
	"net/url"
	"strings"

	"github.com/go-faster/errors"
	"github.com/go-faster/yaml"
	"go.uber.org/zap"

	"github.com/ogen-go/ogen/jsonpointer"
	"github.com/ogen-go/ogen/jsonschema"
	"github.com/ogen-go/ogen/location"

	"github.com/NefixEstrada/agen/asyncapi"
)

// DefaultDepthLimit is the default maximum depth of resolved references.
const DefaultDepthLimit = 1000

// Config is the parser configuration.
type Config struct {
	// External is the resolver for external (file/http) references. Nil forbids them.
	External jsonschema.ExternalResolver
	// InferTypes enables schema type inference when `type` is not set explicitly.
	InferTypes bool
	// DepthLimit limits reference resolution depth.
	DepthLimit int
	// Logger.
	Logger *zap.Logger
	// RootFile carries the root document content for pretty error listings;
	// build with location.NewFile(name, source, data). Optional.
	RootFile location.File
}

func (c *Config) setDefaults() {
	if c.DepthLimit == 0 {
		c.DepthLimit = DefaultDepthLimit
	}
	if c.Logger == nil {
		c.Logger = zap.NewNop()
	}
}

// parser holds the parsing state.
type parser struct {
	spec    *asyncapi.Spec
	root    *yaml.Node
	rootURL *url.URL
	cfg     Config

	schemaParser *jsonschema.Parser
	schemas      *jsonschema.RootResolver

	// channels caches parsed channels by JSON pointer to avoid infinite recursion.
	channels map[string]*Channel
	// messages caches parsed messages by JSON pointer.
	messages map[string]*Message
	// parameters caches parsed parameters by JSON pointer.
	parameters map[string]*Parameter
}

// LocationError is a wrapper for an error that has a location.
type LocationError = location.Error

func (p *parser) file(ctx *jsonpointer.ResolveCtx) location.File {
	file := ctx.File()
	if file.IsZero() {
		return p.rootFile()
	}
	return file
}

// rootFile returns the location.File for the root document: the configured
// RootFile when given (it carries the source for listings), else a name-only
// file.
func (p *parser) rootFile() location.File {
	if !p.cfg.RootFile.IsZero() {
		return p.cfg.RootFile
	}
	name := p.rootURL.String()
	if p.rootURL.Scheme == "file" {
		name = p.rootURL.Path
	}
	return location.File{Name: name}
}

func (p *parser) wrapField(field string, file location.File, l location.Locator, err error) error {
	if err == nil || p == nil {
		return err
	}
	return p.wrapLocation(file, l.Field(field), err)
}

func (p *parser) wrapLocation(file location.File, l location.Locator, err error) error {
	var locErr *LocationError
	if err == nil || p == nil || errors.As(err, &locErr) {
		return err
	}
	pos, ok := l.Position()
	if !ok {
		return err
	}
	if file.IsZero() {
		file = p.rootFile()
	}
	return &LocationError{
		File: file,
		Pos:  pos,
		Err:  err,
	}
}

// Parse parses an AsyncAPI 3.x document into the semantic API model.
func Parse(spec *asyncapi.Spec, root *yaml.Node, rootURL *url.URL, cfg Config) (_ *API, rerr error) {
	cfg.setDefaults()

	p := &parser{
		spec:       spec,
		root:       root,
		rootURL:    rootURL,
		cfg:        cfg,
		channels:   map[string]*Channel{},
		messages:   map[string]*Message{},
		parameters: map[string]*Parameter{},
	}
	p.schemas = jsonschema.NewRootResolver(root)
	settings := jsonschema.Settings{
		Resolver:   p.schemas,
		InferTypes: cfg.InferTypes,
	}
	if cfg.External != nil {
		settings.External = cfg.External
	}
	p.schemaParser = jsonschema.NewParser(settings)

	defer func() {
		if rerr != nil {
			rerr = errors.Wrap(rerr, "parse AsyncAPI")
		}
	}()

	version, err := parseVersion(spec.AsyncAPI)
	if err != nil {
		return nil, p.wrapField("asyncapi", location.File{}, spec.Common.Locator, err)
	}

	api := &API{
		Version:            version,
		Info:               spec.Info,
		ID:                 spec.ID,
		DefaultContentType: defaultContentType(spec),
		Servers:            map[string]*Server{},
	}

	ctx := jsonpointer.NewResolveCtx(rootURL, cfg.DepthLimit)
	if err := p.parseServers(api, ctx); err != nil {
		return nil, err
	}
	if err := p.parseOperations(api, ctx); err != nil {
		return nil, err
	}
	return api, nil
}

// validateInfo checks the required fields of the Info Object.
func validateInfo(info *asyncapi.Info) error {
	if info.Title == "" {
		return errors.New("info.title is required")
	}
	if info.Version == "" {
		return errors.New("info.version is required")
	}
	return nil
}

func defaultContentType(spec *asyncapi.Spec) string {
	if spec.DefaultContentType != "" {
		return spec.DefaultContentType
	}
	return "application/json"
}

func parseVersion(v string) (string, error) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return "", errors.Errorf("invalid version %q", v)
	}
	major, minor := parts[0], parts[1]
	switch major {
	case "3":
		switch minor {
		case "0", "1":
			return v, nil
		default:
			return "", errors.Errorf("unsupported AsyncAPI version %q: supported versions are 3.0.x and 3.1.x", v)
		}
	case "2", "1":
		return "", errors.Errorf("AsyncAPI %s is not supported: upgrade the document to 3.x first (e.g. with the official @asyncapi/converter)", v)
	default:
		return "", errors.Errorf("unsupported AsyncAPI version %q: supported versions are 3.0.x and 3.1.x", v)
	}
}

func (p *parser) parseServers(api *API, ctx *jsonpointer.ResolveCtx) error {
	for name, s := range p.spec.Servers {
		if s == nil {
			continue
		}
		if s.Host == "" {
			err := errors.New("server.host is required")
			return p.wrapField("host", p.file(ctx), s.Common.Locator, err)
		}
		if s.Protocol == "" {
			err := errors.New("server.protocol is required")
			return p.wrapField("protocol", p.file(ctx), s.Common.Locator, err)
		}
		api.Servers[name] = &Server{
			Name:            name,
			Host:            s.Host,
			Protocol:        s.Protocol,
			ProtocolVersion: s.ProtocolVersion,
			Pathname:        s.Pathname,
			Title:           s.Title,
			Summary:         s.Summary,
			Description:     s.Description,
			Security:        s.Security,
			Bindings:        s.Bindings,
			Tags:            s.Tags,
			Common:          s.Common,
		}
	}
	return nil
}
