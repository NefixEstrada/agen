package lowering

import (
	"github.com/go-faster/errors"
	"go.uber.org/zap"

	"github.com/NefixEstrada/agen/internal/ir"
	"github.com/NefixEstrada/agen/internal/naming"
	"github.com/ogen-go/ogen/jsonschema"
)

// EngineOptions configures a lowering Engine.
type EngineOptions struct {
	// Fail is called for each error before it is returned; it may convert
	// an unsupported-feature error into nil to skip it (ignore lists).
	Fail func(err error) error
	// Logger.
	Logger *zap.Logger
	// SchemaDepthLimit limits schema generation recursion.
	SchemaDepthLimit int
	// Initialisms enables the initialism naming rules.
	Initialisms bool
	// Rules is a custom naming ruleset, nil for package default.
	Rules *naming.Ruleset
	// NameRef derives a Go type name from a schema reference; when nil, the
	// default (last pointer segment) is used.
	NameRef func(ref jsonschema.Ref) (string, error)
}

// Engine drives JSON Schema to IR lowering for a set of schemas, sharing one
// type storage so `$ref`'d schemas are generated once.
type Engine struct {
	ctx *genctx
	gen *schemaGen

	// checkedRecursion tracks struct types already processed by
	// checkStructRecursions (it mutates field types, so it must run once).
	checkedRecursion map[*ir.Type]struct{}

	fail        func(err error) error
	depthLimit  int
	initialisms bool
	rules       *naming.Ruleset
	nameRef     func(ref jsonschema.Ref) (string, error)
	log         *zap.Logger
}

// NewEngine creates a new lowering engine.
func NewEngine(opts EngineOptions) *Engine {
	fail := opts.Fail
	if fail == nil {
		fail = func(err error) error { return err }
	}
	log := opts.Logger
	if log == nil {
		log = zap.NewNop()
	}
	depthLimit := opts.SchemaDepthLimit
	if depthLimit == 0 {
		depthLimit = defaultSchemaDepthLimit
	}
	e := &Engine{
		nameRef: opts.NameRef,
		ctx: &genctx{
			global: newTStorage(),
			local:  newTStorage(),
		},
		checkedRecursion: map[*ir.Type]struct{}{},
		fail:             fail,
		depthLimit:       depthLimit,
		initialisms:      opts.Initialisms,
		rules:            opts.Rules,
		log:              log,
	}
	return e
}

// Generate lowers the given schema into an IR type with the provided name
// (used when the schema is anonymous).
func (e *Engine) Generate(name string, schema *jsonschema.Schema, optional bool) (_ *ir.Type, rerr error) {
	defer handleSchemaDepth(schema, &rerr)

	gen := newSchemaGen(func(ref jsonschema.Ref) (*ir.Type, bool) {
		return e.ctx.lookupRef(ref, ir.EncodingJSON)
	})
	gen.initialisms = e.initialisms
	gen.rules = e.rules
	gen.log = e.log.Named("schemagen")
	gen.fail = e.fail
	gen.depthLimit = e.depthLimit
	if hook := e.nameRef; hook != nil {
		prev := gen.nameRef
		gen.nameRef = func(ref jsonschema.Ref) (string, error) {
			if n, err := hook(ref); err != nil {
				return "", err
			} else if n != "" {
				return n, nil
			}
			return prev(ref)
		}
	}

	t, err := gen.generate(name, schema, optional)
	if err != nil {
		return nil, e.fail(err)
	}

	if err := saveSchemaTypes(e.ctx, gen, nil); err != nil {
		return nil, errors.Wrap(err, "save schema types")
	}

	// Recursive optional fields must be pointers instead of Opt generics
	// (Go forbids recursive generic instantiations).
	candidates := append([]*ir.Type{t}, gen.side...)
	for _, nt := range gen.localRefs {
		candidates = append(candidates, nt)
	}
	for _, candidate := range candidates {
		if candidate == nil || !candidate.IsStruct() {
			continue
		}
		if _, done := e.checkedRecursion[candidate]; done {
			continue
		}
		e.checkedRecursion[candidate] = struct{}{}
		if err := checkStructRecursions(candidate); err != nil {
			return nil, errors.Wrap(err, candidate.Name)
		}
	}
	return t, nil
}

// AddFeature adds a feature (e.g. "json") to the given type, propagating it
// through the type graph.
func (e *Engine) AddFeature(t *ir.Type, feature string) {
	t.AddFeature(feature)
}

// Types returns all named types generated so far.
func (e *Engine) Types() map[string]*ir.Type {
	return e.ctx.local.types
}

// SaveType registers a manually built type in the engine storage.
func (e *Engine) SaveType(t *ir.Type) error {
	return e.ctx.saveType(t)
}

// TStorageView is a view of named types.
type TStorageView = map[string]*ir.Type

// CheckStructRecursions checks the given struct type for infinite recursion.
func CheckStructRecursions(t *ir.Type) error {
	if t.IsStruct() {
		return checkStructRecursions(t)
	}
	return nil
}

// saveSchemaTypes saves schemaGen side and referenced types into the context.
func saveSchemaTypes(ctx *genctx, gen *schemaGen, refEncoding map[jsonschema.Ref]ir.Encoding) error {
	for _, t := range gen.side {
		if err := ctx.saveType(t); err != nil {
			return errors.Wrap(err, "save inlined type")
		}
	}

	for ref, t := range gen.localRefs {
		encoding := ir.EncodingJSON
		if e, ok := refEncoding[ref]; ok {
			encoding = e
		}
		if err := ctx.saveRef(ref, encoding, t); err != nil {
			return errors.Wrap(err, "save referenced type")
		}
	}
	return nil
}
