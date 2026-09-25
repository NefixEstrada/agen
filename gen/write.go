package gen

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"text/template"

	"github.com/go-faster/errors"
	"golang.org/x/tools/imports"

	"github.com/NefixEstrada/agen/internal/ir"
	"github.com/NefixEstrada/agen/internal/xmaps"
	"github.com/NefixEstrada/agen/runtime/ogenregex"
)

// TemplateConfig is the configuration passed to code generation templates.
type TemplateConfig struct {
	Package string

	Operations []*ir.Operation
	// SendOperations are operations with action `send` (publisher methods).
	SendOperations []*ir.Operation
	// ReceiveOperations are operations with action `receive` (handlers).
	ReceiveOperations []*ir.Operation
	// Channels is the list of unique channels used by operations.
	Channels []*ir.Channel
	// Servers is the list of servers from the spec.
	Servers []*ir.Server
	// Info is the API info.
	Info APIInfo

	Types       map[string]*ir.Type
	Interfaces  map[string]*ir.Type
	Imports     map[string]string
	Controllers []string

	// Feature flags.
	PublisherEnabled   bool
	SubscriberEnabled  bool
	MiddlewareEnabled  bool
	UnimplementedGiven bool
	GenerateFakes      bool

	// BrokerWired is true when all spec servers share a single protocol with
	// a shipped runtime backend (currently redis): server/client constructors
	// then wire that backend internally.
	BrokerWired bool
	// BrokerProtocol is the servers' protocol, empty when the spec declares
	// no servers.
	BrokerProtocol string

	// RuntimeImport is the base import path of the agen runtime.
	RuntimeImport string
}

// APIInfo carries spec info into generated godoc.
type APIInfo struct {
	Title       string
	Version     string
	Description string
}

// AnyOperationEnabled returns true if there is anything to generate.
func (t TemplateConfig) AnyOperationEnabled() bool {
	return len(t.Operations) > 0
}

// ClientEnabled returns true when the client (publisher) file is generated.
func (t TemplateConfig) ClientEnabled() bool {
	return t.PublisherEnabled && len(t.SendOperations) > 0
}

// ServerEnabled returns true when the server (consumer) file is generated:
// only specs whose servers all share a backend-wired protocol can wire the
// consumer internally.
func (t TemplateConfig) ServerEnabled() bool {
	return t.BrokerWired && t.SubscriberEnabled && len(t.ReceiveOperations) > 0
}

// ReceiveChannels returns the unique static-address channels consumed by
// receive operations, in order of first use.
func (t TemplateConfig) ReceiveChannels() []*ir.Channel {
	seen := map[*ir.Channel]bool{}
	var out []*ir.Channel
	for _, op := range t.ReceiveOperations {
		ch := op.Channel
		if ch == nil || ch.HasParams() || seen[ch] {
			continue
		}
		seen[ch] = true
		out = append(out, ch)
	}
	return out
}

func (t TemplateConfig) collectStrings(cb func(typ *ir.Type) []string) []string {
	var (
		add  func(typ *ir.Type)
		m    = map[string]struct{}{}
		seen = map[*ir.Type]struct{}{}
	)
	add = func(typ *ir.Type) {
		_, skip := seen[typ]
		if typ == nil || skip {
			return
		}
		seen[typ] = struct{}{}
		for _, got := range cb(typ) {
			m[got] = struct{}{}
		}

		for _, f := range typ.Fields {
			add(f.Type)
		}
		for _, f := range typ.SumOf {
			add(f)
		}
		add(typ.AliasTo)
		add(typ.PointerTo)
		add(typ.GenericOf)
		add(typ.Item)
	}

	for _, typ := range t.Types {
		add(typ)
	}
	for _, typ := range t.Interfaces {
		add(typ)
	}
	for _, op := range t.Operations {
		for _, msg := range op.Messages {
			add(msg.Type)
		}
	}

	return xmaps.SortedKeys(m)
}

// RegexStrings returns slice of all unique regex validators.
func (t TemplateConfig) RegexStrings() []string {
	return t.collectStrings(func(typ *ir.Type) (r []string) {
		for _, exp := range []ogenregex.Regexp{
			typ.Validators.String.Regex,
			typ.Validators.Int.Pattern,
			typ.Validators.Float.Pattern,
			typ.MapPattern,
		} {
			if exp == nil {
				continue
			}
			r = append(r, exp.String())
		}
		return
	})
}

// RatStrings returns slice of all unique big.Rat (multipleOf validation).
func (t TemplateConfig) RatStrings() []string {
	return t.collectStrings(func(typ *ir.Type) []string {
		if r := typ.Validators.Float.MultipleOf; r != nil {
			// `RatString` return a string with integer value if denominator is 1.
			//
			// That makes string representation of `big.Rat` shorter and simpler.
			// Also, it is better for executable size.
			return []string{r.RatString()}
		}
		return nil
	})
}

// FileSystem represents a directory of generated package.
type FileSystem interface {
	WriteFile(baseName string, source []byte) error
}

type writer struct {
	fs FileSystem
	t  *template.Template
}

// generatorBufSize is 1 MB, it's enough for most mid-size specs.
const generatorBufSize = 1024 * 1024

var bufPool = sync.Pool{
	New: func() any {
		var b bytes.Buffer
		b.Grow(generatorBufSize)
		b.Reset()
		return &b
	},
}

func getBuffer() *bytes.Buffer {
	b := bufPool.Get().(*bytes.Buffer)
	b.Reset()
	return b
}

func putBuffer(b *bytes.Buffer) {
	if b.Cap() > generatorBufSize {
		return
	}
	bufPool.Put(b)
}

// Generate executes template to file using config.
func (w *writer) Generate(templateName, fileName string, cfg TemplateConfig) (rerr error) {
	buf := getBuffer()
	defer putBuffer(buf)

	if err := w.t.ExecuteTemplate(buf, templateName, cfg); err != nil {
		return errors.Wrap(err, "execute")
	}

	generated := buf.Bytes()
	defer func() {
		if rerr != nil {
			_ = os.WriteFile(fileName+".dump", generated, 0o644)
		}
	}()

	formatted, err := imports.Process(fileName, generated, nil)
	if err != nil {
		return &ErrGoFormat{
			err: err,
		}
	}

	if err := w.fs.WriteFile(fileName, formatted); err != nil {
		return errors.Wrap(err, "write")
	}

	return nil
}

// WriteSource writes generated definitions to fs.
func (g *Generator) WriteSource(fs FileSystem, pkgName string) error {
	w := &writer{
		fs: fs,
		t:  vendoredTemplates(),
	}

	// Historically we separate interfaces from other types.
	// This is done for backward compatibility.
	types := make(map[string]*ir.Type, len(g.tstorage))
	interfaces := make(map[string]*ir.Type)
	for name, t := range g.tstorage {
		if t.IsInterface() {
			interfaces[name] = t
			continue
		}
		types[name] = t
	}

	wired, protocol := brokerBackend(g.servers)
	cfg := TemplateConfig{
		Package:           pkgName,
		Operations:        g.operations,
		SendOperations:    g.sendOperations(),
		ReceiveOperations: g.receiveOperations(),
		Channels:          g.channels,
		Servers:           g.servers,
		Info:              g.info,
		Types:             types,
		Interfaces:        interfaces,
		Imports:           g.imports,
		Controllers:       nil,

		PublisherEnabled:   g.features.Has(Publisher),
		SubscriberEnabled:  g.features.Has(Subscriber),
		MiddlewareEnabled:  g.features.Has(Middleware),
		UnimplementedGiven: g.features.Has(Unimplemented),
		GenerateFakes:      g.features.Has(Fakes),

		BrokerWired:    wired,
		BrokerProtocol: protocol,

		RuntimeImport: "github.com/NefixEstrada/agen/runtime",
	}

	for _, t := range []struct {
		name    string
		enabled bool
	}{
		{"schemas", true},
		{"interfaces", len(interfaces) > 0},
		{"json", g.hasJSON()},
		{"validators", g.hasValidators()},
		{"defaults", g.hasDefaultFields()},
		{"cfg", true},
		{"servers", len(cfg.Servers) > 0},
		{"operations", cfg.AnyOperationEnabled()},
		{"msg", true},
		{"handlers", cfg.SubscriberEnabled && len(cfg.ReceiveOperations) > 0},
		{"subscriber", cfg.SubscriberEnabled && len(cfg.ReceiveOperations) > 0},
		{"server", cfg.ServerEnabled()},
		{"client", cfg.ClientEnabled()},
		{"middleware", cfg.MiddlewareEnabled && cfg.SubscriberEnabled && len(cfg.ReceiveOperations) > 0},
		{"unimplemented", cfg.UnimplementedGiven && cfg.SubscriberEnabled && len(cfg.ReceiveOperations) > 0},
		{"fakes", cfg.GenerateFakes},
	} {
		if !t.enabled {
			continue
		}
		fileName := fmt.Sprintf("aas_%s_gen.go", t.name)
		if err := w.Generate(t.name, fileName, cfg); err != nil {
			return errors.Wrapf(err, "template %q", t.name)
		}
	}

	// Generate Equal() and Hash() methods for complex uniqueItems validation
	if len(g.equalitySpecs) > 0 {
		if err := g.generateEqualityMethodsWithFS(fs, pkgName); err != nil {
			return errors.Wrap(err, "equality methods")
		}
		// Generate validateUnique[TypeName]() functions for runtime validation
		if err := g.generateUniqueValidators(fs, pkgName); err != nil {
			return errors.Wrap(err, "unique validators")
		}
	}

	return nil
}

func (g *Generator) sendOperations() (r []*ir.Operation) {
	for _, op := range g.operations {
		if op.IsSend() {
			r = append(r, op)
		}
	}
	return r
}

func (g *Generator) receiveOperations() (r []*ir.Operation) {
	for _, op := range g.operations {
		if op.IsReceive() {
			r = append(r, op)
		}
	}
	return r
}

func (g *Generator) hasAnyType(check func(t *ir.Type) bool) bool {
	for _, t := range g.tstorage {
		if check(t) {
			return true
		}
	}
	return false
}

func (g *Generator) hasDefaultFields() bool {
	return g.hasAnyType((*ir.Type).HasDefaultFields)
}

func (g *Generator) hasJSON() bool {
	return g.hasAnyType(func(t *ir.Type) bool {
		return t.HasFeature("json")
	})
}

func (g *Generator) hasValidators() bool {
	return g.hasAnyType((*ir.Type).NeedValidation)
}

// ErrGoFormat reports that generated code formatting failed.
type ErrGoFormat struct {
	err error
}

// Unwrap implements errors.Wrapper.
func (e *ErrGoFormat) Unwrap() error { return e.err }

// FormatError implements errors.Formatter.
func (e *ErrGoFormat) FormatError(p errors.Printer) (next error) {
	p.Print("goimports")
	return e.err
}

// Format implements fmt.Formatter.
func (e *ErrGoFormat) Format(s fmt.State, verb rune) { errors.FormatError(e, s, verb) }

// Error implements error.
func (e *ErrGoFormat) Error() string { return fmt.Sprintf("goimports: %s", e.err) }
