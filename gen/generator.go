// Package gen generates Go code from a parsed AsyncAPI 3.x document: message
// types, JSON codecs, validators, typed publishers and subscribers.
package gen

import (
	"regexp"
	"strings"

	"github.com/go-faster/yaml"
	"go/token"

	"github.com/go-faster/errors"
	"go.uber.org/zap"

	"github.com/NefixEstrada/agen/asyncapi"
	"github.com/NefixEstrada/agen/asyncapi/parser"
	"github.com/NefixEstrada/agen/internal/ir"
	"github.com/NefixEstrada/agen/internal/lowering"
	"github.com/NefixEstrada/agen/internal/naming"
	"github.com/ogen-go/ogen/jsonschema"
)

// Feature is a code generation feature flag.
type Feature string

// Supported features.
const (
	// Publisher generates the client (publisher) for `send` operations.
	Publisher Feature = "publisher"
	// Subscriber generates handler interfaces, subscriber and dispatch code
	// for `receive` operations.
	Subscriber Feature = "subscriber"
	// Middleware generates middleware hooks for subscriber dispatch.
	Middleware Feature = "middleware"
	// Fakes generates a fake publisher and handler harness for tests.
	Fakes Feature = "fakes"
	// Unimplemented generates a handler that returns errors for all receive
	// operations.
	Unimplemented Feature = "unimplemented"
	// DebugExampleTests is reserved for future example-based tests.
	DebugExampleTests Feature = "debug_example_tests"
)

// DefaultFeatures are enabled unless overridden.
var DefaultFeatures = []Feature{Publisher, Subscriber, Middleware}

// AllFeatures lists every feature for validation.
var AllFeatures = []Feature{Publisher, Subscriber, Middleware, Fakes, Unimplemented, DebugExampleTests}

// Features is a feature set.
type Features map[Feature]struct{}

// Has returns true if the feature is enabled.
func (f Features) Has(p Feature) bool {
	_, ok := f[p]
	return ok
}

// BuildFeatures builds a feature set from enable/disable lists.
func BuildFeatures(enable, disable []Feature, disableAll bool) (f Features, err error) {
	if disableAll {
		return Features{}, nil
	}
	f = Features{}
	for _, p := range DefaultFeatures {
		f[p] = struct{}{}
	}
	for _, p := range enable {
		if !featureKnown(p) {
			return nil, errors.Errorf("unknown feature %q", p)
		}
		f[p] = struct{}{}
	}
	for _, p := range disable {
		if !featureKnown(p) {
			return nil, errors.Errorf("unknown feature %q", p)
		}
		delete(f, p)
	}
	return f, nil
}

func featureKnown(p Feature) bool {
	for _, known := range AllFeatures {
		if known == p {
			return true
		}
	}
	return false
}

// Filters select a subset of operations to generate.
type Filters struct {
	// OperationsRegex keeps only operations whose name matches.
	OperationsRegex *regexp.Regexp
	// Actions keeps only the given actions ("send"/"receive"); empty = all.
	Actions []string
}

// Options is the generator configuration.
type Options struct {
	Features          Features
	Filters           Filters
	IgnoreUnsupported []string
	// Initialisms applies the initialism naming rules (e.g. "Id" -> "ID")
	// to generated identifiers.
	Initialisms bool
	Logger      *zap.Logger
}

// Generator generates code for a parsed AsyncAPI document.
type Generator struct {
	opt      Options
	features Features
	log      *zap.Logger

	engine     *lowering.Engine
	tstorage   lowering.TStorageView
	operations []*ir.Operation
	channels   []*ir.Channel
	servers    []*ir.Server
	info       APIInfo
	imports    map[string]string
	// equalitySpecs are types requiring Equal() and Hash() methods for
	// complex uniqueItems validation.
	equalitySpecs []*ir.EqualityMethodSpec
	builtMsgs     map[*parser.Message]*ir.Message
	sumTypes      map[string]*ir.Type
	seenNames     map[string]struct{}
	refNames      map[string]string // schema ref ptr -> type name
	refNameUsed   map[string]struct{}

	// Redis binding defaults for the generated constructors (the draft redis
	// binding 0.2.0 fields, read from the `x-redis` extension).
	redisMode   string // "stream" or "pubsub" declared by the channels
	redisGroup  string // consumerGroup declared by the receive operations
	redisMaxLen int64  // maxLen declared by the send channels
}

// NewGenerator parses the API model into IR.
func NewGenerator(api *parser.API, opt Options) (*Generator, error) {
	if opt.Logger == nil {
		opt.Logger = zap.NewNop()
	}
	features := opt.Features
	if features == nil {
		var err error
		features, err = BuildFeatures(nil, nil, false)
		if err != nil {
			return nil, errors.Wrap(err, "build feature set")
		}
	}
	imports := lowering.DefaultImports()
	for path, alias := range map[string]string{
		"github.com/NefixEstrada/agen/runtime/broker":     "",
		"github.com/NefixEstrada/agen/runtime/agenerrors": "",
	} {
		imports[path] = alias
	}
	g := &Generator{
		opt:      opt,
		features: features,
		log:      opt.Logger,
		imports:  imports,
	}
	g.engine = lowering.NewEngine(lowering.EngineOptions{
		Fail:        g.fail,
		Logger:      opt.Logger,
		Initialisms: opt.Initialisms,
		NameRef:     g.nameSchemaRef,
	})
	g.builtMsgs = map[*parser.Message]*ir.Message{}
	g.sumTypes = map[string]*ir.Type{}
	g.seenNames = map[string]struct{}{}
	g.refNames = map[string]string{}
	g.refNameUsed = map[string]struct{}{}
	g.refNames = map[string]string{}
	g.refNameUsed = map[string]struct{}{}

	if err := g.build(api); err != nil {
		return nil, errors.Wrap(err, "build")
	}
	return g, nil
}

// fail applies the `ignore_unsupported` list to unsupported-feature errors.
func (g *Generator) fail(err error) error {
	if err == nil {
		return nil
	}
	var (
		ni        *lowering.ErrNotImplemented
		hasAll    = contains(g.opt.IgnoreUnsupported, "all")
		hasSchema = containsPrefix(g.opt.IgnoreUnsupported, "schema.")
	)
	if errors.As(err, &ni) {
		if hasAll ||
			contains(g.opt.IgnoreUnsupported, ni.Name) ||
			(hasSchema && contains(g.opt.IgnoreUnsupported, "schema."+ni.Name)) {
			g.log.Info("skipping unsupported feature",
				zap.String("feature", ni.Name),
			)
			return nil
		}
		return err
	}
	return err
}

func contains(s []string, v string) bool {
	for _, item := range s {
		if item == v {
			return true
		}
	}
	return false
}

func containsPrefix(s []string, prefix string) bool {
	for _, item := range s {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}

// knownProtocols are protocols agen understands; redis has a runtime backend,
// the rest generate protocol-agnostic code with a warning for now.
var knownProtocols = map[string]bool{
	"redis":        true,
	"kafka":        true,
	"kafka-secure": true,
	"mqtt":         true,
	"mqtt5":        true,
	"amqp":         true,
	"amqp1":        true,
	"nats":         true,
	"websockets":   true,
	"ws":           true,
	"googlepubsub": true,
	"pulsar":       true,
	"sns":          true,
	"sqs":          true,
	"ibmmq":        true,
	"solace":       true,
	"jms":          true,
	"mercure":      true,
	"stomp":        true,
	"http":         true,
}

// protocolsWithBackend lists protocols for which agen ships a runtime.
var protocolsWithBackend = map[string]bool{
	"redis": true,
}

// brokerBackend reports whether all spec servers share a single protocol with
// a shipped runtime backend, and that protocol. Server/client constructors
// wire the backend internally only in that case.
func brokerBackend(servers []*ir.Server) (wired bool, protocol string) {
	for _, s := range servers {
		if !protocolsWithBackend[s.Protocol] {
			return false, s.Protocol
		}
		switch protocol {
		case "":
			protocol = s.Protocol
		case s.Protocol:
		default:
			return false, protocol
		}
	}
	return protocol != "", protocol
}

func (g *Generator) build(api *parser.API) error {
	g.info = APIInfo{
		Title:       api.Info.Title,
		Version:     api.Info.Version,
		Description: api.Info.Description,
	}

	for _, name := range sortedServerKeys(api.Servers) {
		s := api.Servers[name]
		if !knownProtocols[s.Protocol] {
			err := &lowering.ErrNotImplemented{Name: "protocol " + s.Protocol}
			if g.fail(err) != nil {
				return errors.Wrapf(err, "server %q", name)
			}
			continue
		}
		if !protocolsWithBackend[s.Protocol] {
			g.log.Warn("no runtime backend for protocol yet; generated code is protocol-agnostic",
				zap.String("protocol", s.Protocol),
				zap.String("server", name),
			)
		}
		goName, err := g.pascalName(s.Name)
		if err != nil {
			return errors.Wrapf(err, "server name %q", name)
		}
		g.servers = append(g.servers, &ir.Server{
			Name:            s.Name,
			GoName:          goName,
			Host:            s.Host,
			Protocol:        s.Protocol,
			ProtocolVersion: s.ProtocolVersion,
			Description:     s.Description,
		})
	}

	// Server/client constructors wire the backend only when every server
	// shares one backend-wired protocol; generated code then imports it.
	if wired, _ := brokerBackend(g.servers); wired {
		g.imports["github.com/NefixEstrada/agen/runtime/redis"] = "redisruntime"
	}

	// Filter operations.
	var ops []*parser.Operation
	for _, op := range api.Operations {
		if r := g.opt.Filters.OperationsRegex; r != nil && !r.MatchString(op.Name) {
			continue
		}
		if len(g.opt.Filters.Actions) > 0 && !contains(g.opt.Filters.Actions, string(op.Action)) {
			continue
		}
		ops = append(ops, op)
	}

	seenChannels := map[*parser.Channel]*ir.Channel{}
	opNames := map[string]struct{}{}
	var synthetic []*parser.Operation
	for _, op := range ops {
		if op.Reply != nil && op.Action == parser.ReceiveAction {
			// The reply of a receive operation is a publish surface: model it
			// as a synthetic send operation ("<op> reply"), so the client
			// gets a typed publisher method for the reply channel.
			synthetic = append(synthetic, &parser.Operation{
				Name:        op.Name + "Reply",
				Action:      parser.SendAction,
				Channel:     op.Reply.Channel,
				Messages:    op.Reply.Messages,
				Title:       op.Reply.Title,
				Summary:     op.Reply.Summary,
				Description: op.Reply.Description,
				Common:      op.Reply.Common,
			})
		}
		irOp, err := g.buildOperation(op, seenChannels)
		if err != nil {
			return errors.Wrapf(err, "operation %q", op.Name)
		}
		if _, ok := opNames[irOp.Name]; ok {
			return errors.Errorf("name conflict: %q", irOp.Name)
		}
		opNames[irOp.Name] = struct{}{}
		g.operations = append(g.operations, irOp)
	}
	for _, op := range synthetic {
		irOp, err := g.buildOperation(op, seenChannels)
		if err != nil {
			return errors.Wrapf(err, "operation %q", op.Name)
		}
		if _, ok := opNames[irOp.Name]; ok {
			return errors.Errorf("name conflict: %q", irOp.Name)
		}
		opNames[irOp.Name] = struct{}{}
		g.operations = append(g.operations, irOp)
	}

	// Derive the redis wiring declared by the x-redis extensions (draft
	// binding 0.2.0). The spec is the single source of truth: every channel
	// must declare its type and every streams-mode receive operation its
	// consumerGroup — the generated code exposes no mode or group options,
	// so an undeclared value is an error, never a silent default.
	if wired, protocol := brokerBackend(g.servers); wired {
		effective := append(append([]*parser.Operation{}, ops...), synthetic...)
		mode, group, maxLen, err := redisBindingDefaults(effective)
		if err != nil {
			return errors.Wrap(err, "redis extensions")
		}
		g.redisMode, g.redisGroup, g.redisMaxLen = mode, group, maxLen
	} else if protocol != "" {
		for _, op := range ops {
			if op.Redis != nil || (op.Channel != nil && op.Channel.Redis != nil) {
				g.log.Warn("ignoring x-redis extensions: servers use a different protocol",
					zap.String("protocol", protocol),
					zap.String("operation", op.Name),
				)
				break
			}
		}
	}

	g.tstorage = g.engine.Types()

	// Collect types that need Equal() and Hash() methods for complex uniqueItems validation
	g.collectEqualitySpecs()

	return nil
}

func (g *Generator) buildOperation(op *parser.Operation, seenChannels map[*parser.Channel]*ir.Channel) (*ir.Operation, error) {
	goName, err := g.goNameOf(op.Common.Extensions, op.Name)
	if err != nil {
		return nil, errors.Wrap(err, "operation name")
	}
	g.reserveName(goName)

	// Build or reuse the channel IR.
	irCh, ok := seenChannels[op.Channel]
	if !ok {
		irCh, err = g.buildChannel(op.Channel)
		if err != nil {
			return nil, errors.Wrapf(err, "channel %q", op.Channel.Name)
		}
		seenChannels[op.Channel] = irCh
		g.channels = append(g.channels, irCh)
	}

	irOp := &ir.Operation{
		Name:        goName,
		OperationID: op.Name,
		Action:      string(op.Action),
		Description: op.Description,
		Channel:     irCh,
		GoDoc:       prettyDescription(op),
	}

	for _, msg := range op.Messages {
		irMsg, ok := g.builtMsgs[msg]
		if !ok {
			var err error
			irMsg, err = g.buildMessage(irOp, msg)
			if err != nil {
				return nil, errors.Wrapf(err, "message %q", msg.SpecName)
			}
			g.builtMsgs[msg] = irMsg
		}
		irOp.Messages = append(irOp.Messages, irMsg)
	}

	if len(irOp.Messages) > 1 {
		// Multi-message operation: generate a sum interface with
		// try-decode-in-order dispatch. Operations sharing the same message
		// set share the interface (envelope types carry one marker method).
		var key []string
		for _, m := range irOp.Messages {
			key = append(key, m.SpecName)
		}
		sortStrings(key)
		setKey := strings.Join(key, "\x00")
		iface, ok := g.sumTypes[setKey]
		if !ok {
			iface = ir.Interface(irCh.GoName + "Message")
			marker := lowerFirst(irCh.GoName) + "Message"
			iface.AddMethod(marker)
			for _, m := range irOp.Messages {
				m.Type.Implement(iface)
				m.SumMarker = marker
			}
			if err := g.engine.SaveType(iface); err != nil {
				return nil, errors.Wrap(err, "save sum type")
			}
			g.sumTypes[setKey] = iface
		}
		irOp.SumType = iface
	}

	return irOp, nil
}

// redisBindingDefaults derives the redis wiring declared by the x-redis
// extensions of the effective operations: the channel `type` selects the
// mapping mode, receive operations' `consumerGroup` the server group and the
// send channels' `maxLen` the stream trim length.
//
// The spec is the single source of truth — the generated code exposes no
// mode or group options — so every channel must declare its type and every
// streams-mode receive operation its consumerGroup: an undeclared value is an
// error, not a default. The runtime wires a single consumer and publisher per
// service, so declarations that disagree are errors too.
func redisBindingDefaults(ops []*parser.Operation) (mode, group string, maxLen int64, err error) {
	for _, op := range ops {
		ch := op.Channel
		if ch == nil {
			continue
		}
		if ch.Redis == nil {
			return "", "", 0, errors.Errorf("channel %q: x-redis.type is required (stream or pubsub)", ch.Name)
		}
		if mode == "" {
			mode = ch.Redis.Type
		} else if mode != ch.Redis.Type {
			return "", "", 0, errors.Errorf(
				"channel %q declares redis type %q but %q was declared elsewhere: mixing stream and pubsub channels needs separate services",
				ch.Name, ch.Redis.Type, mode,
			)
		}
		switch op.Action {
		case parser.ReceiveAction:
			if op.Redis == nil || op.Redis.ConsumerGroup == "" {
				if mode == parser.RedisTypeStream {
					return "", "", 0, errors.Errorf("operation %q: x-redis.consumerGroup is required in streams mode", op.Name)
				}
				break // pubsub receives need no group
			}
			if group == "" {
				group = op.Redis.ConsumerGroup
			} else if group != op.Redis.ConsumerGroup {
				return "", "", 0, errors.Errorf(
					"operation %q declares redis consumerGroup %q but %q was declared elsewhere: per-operation groups need separate services",
					op.Name, op.Redis.ConsumerGroup, group,
				)
			}
		case parser.SendAction:
			if ch.Redis.MaxLen == 0 {
				break
			}
			if maxLen == 0 {
				maxLen = ch.Redis.MaxLen
			} else if maxLen != ch.Redis.MaxLen {
				return "", "", 0, errors.Errorf(
					"channel %q declares redis maxLen %d but %d was declared elsewhere: per-channel maxLen is not supported yet",
					ch.Name, ch.Redis.MaxLen, maxLen,
				)
			}
		}
	}
	return mode, group, maxLen, nil
}

func prettyDescription(op *parser.Operation) []string {
	doc := op.Description
	if doc == "" {
		doc = op.Summary
	}
	if doc == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(doc, "\n"), "\n")
}

func (g *Generator) buildChannel(ch *parser.Channel) (*ir.Channel, error) {
	goName, err := g.goNameOf(ch.Common.Extensions, ch.Name)
	if err != nil {
		return nil, errors.Wrap(err, "channel name")
	}
	g.reserveName(goName)
	irCh := &ir.Channel{
		Name:        ch.Name,
		GoName:      goName,
		Address:     ch.Address,
		Description: ch.Description,
	}
	// Order parameters by appearance in the address.
	for _, token := range addressTokens(ch.Address) {
		p, ok := ch.Parameters[token]
		if !ok {
			continue // parser already validated this
		}
		goName := naming.Capitalize(p.Name)
		if g.opt.Initialisms {
			if n, err := lowering.PascalSpecialInitialisms(p.Name); err == nil {
				goName = n
			}
		}
		irCh.Params = append(irCh.Params, &ir.ChannelParam{
			Name:        p.Name,
			GoName:      goName,
			Enum:        p.Enum,
			Default:     p.Default,
			Description: p.Description,
		})
	}
	return irCh, nil
}

func (g *Generator) buildMessage(op *ir.Operation, msg *parser.Message) (*ir.Message, error) {
	name, err := g.goNameOf(msg.Common.Extensions, msg.Name)
	if err != nil {
		return nil, errors.Wrap(err, "message name")
	}

	g.reserveName(name)
	g.reserveName(name + "Payload")
	g.reserveName(name + "Headers")
	irMsg := &ir.Message{
		Name:        name,
		SpecName:    msg.SpecName,
		Title:       msg.Title,
		Description: msg.Description,
		ContentType: msg.ContentType,
	}

	doc := msg.Description
	if doc == "" {
		doc = msg.Summary
	}
	if doc == "" {
		doc = msg.Title
	}
	envelopeSchema := &jsonschema.Schema{
		Description: doc,
	}

	envelope := &ir.Type{
		Kind:   ir.KindStruct,
		Name:   name,
		Schema: envelopeSchema,
	}

	typeName := func(suffix string) string { return name + suffix }
	if msg.Payload != nil {
		payload, err := g.engine.Generate(typeName("Payload"), msg.Payload, false)
		if err != nil {
			return nil, errors.Wrap(err, "payload")
		}
		g.engine.AddFeature(payload, "json")
		irMsg.Payload = payload
		envelope.Fields = append(envelope.Fields, &ir.Field{
			Name: "Payload",
			Type: payload,
			Tag:  ir.Tag{JSON: "-"},
		})
	}
	if msg.Headers != nil {
		headers, err := g.engine.Generate(typeName("Headers"), msg.Headers, false)
		if err != nil {
			return nil, errors.Wrap(err, "headers")
		}
		g.engine.AddFeature(headers, "json")
		irMsg.Headers = headers
		envelope.Fields = append(envelope.Fields, &ir.Field{
			Name: "Headers",
			Type: headers,
			Tag:  ir.Tag{JSON: "-"},
		})
	}

	irMsg.Type = envelope
	if err := g.engine.SaveType(envelope); err != nil {
		return nil, errors.Wrap(err, "save envelope type")
	}
	return irMsg, nil
}

func addressTokens(address string) []string {
	var tokens []string
	for {
		start := strings.IndexByte(address, '{')
		if start < 0 {
			return tokens
		}
		end := strings.IndexByte(address[start:], '}')
		if end < 0 {
			return tokens
		}
		tokens = append(tokens, address[start+1:start+end])
		address = address[start+end+1:]
	}
}

func sortedServerKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

// nameSchemaRef derives a schema type name from its reference, keeping names
// unique across the document:
//
//   - #/components/schemas/<key> names by its key (the default rule);
//   - deeper pointers (cross-channel references in converted documents) name
//     by expanding tails of their path (payload -> message payload -> channel
//     message payload) until unique.
//
// Dotted segments (converter-era message keys) are joined without the dots.
func (g *Generator) nameSchemaRef(ref jsonschema.Ref) (string, error) {
	ptr := strings.TrimPrefix(ref.Ptr, "#/")
	if ref.Ptr != "" && ptr == ref.Ptr {
		// External reference: let the default rule handle it.
		return "", nil
	}
	if name, ok := g.refNames[ref.Ptr]; ok {
		return name, nil
	}
	segments := strings.Split(ptr, "/")
	for i, seg := range segments {
		segments[i] = strings.ReplaceAll(strings.ReplaceAll(seg, "~1", "/"), "~0", "~")
		segments[i] = strings.ReplaceAll(segments[i], ".", " ")
		segments[i] = strings.TrimSpace(segments[i])
	}
	var candidates [][]string
	if len(segments) >= 3 && segments[0] == "components" && segments[1] == "schemas" {
		candidates = append(candidates, segments[2:])
	} else {
		for n := 2; n <= len(segments); n++ {
			candidates = append(candidates, segments[len(segments)-n:])
		}
	}
	for _, tail := range candidates {
		if len(tail) == 0 {
			continue
		}
		name, err := g.pascalName(tail...)
		if err != nil {
			return "", err
		}
		if name == "" {
			continue
		}
		if _, used := g.refNameUsed[name]; used {
			continue
		}
		g.refNameUsed[name] = struct{}{}
		g.refNames[ref.Ptr] = name
		return name, nil
	}
	// All candidates collide (should not happen): fall back to the default.
	return "", nil
}

// reserveName marks a Go name as taken so ref-derived schema names avoid it.
func (g *Generator) reserveName(name string) {
	if name == "" {
		return
	}
	g.refNameUsed[name] = struct{}{}
}

func (g *Generator) pascalName(parts ...string) (string, error) {
	joined := strings.Join(parts, " ")
	if g.opt.Initialisms {
		return lowering.PascalSpecialInitialisms(joined)
	}
	return lowering.PascalSpecial(joined)
}

// goNameOf resolves a Go name from the x-agen-name (or x-ogen-name)
// extension, falling back to pascal-casing the spec name.
func (g *Generator) goNameOf(extensions asyncapi.Extensions, specName string) (string, error) {
	for _, key := range []string{"x-agen-name", "x-ogen-name"} {
		node, ok := extensions[key]
		if !ok || node.Kind != yaml.ScalarNode || node.Value == "" {
			continue
		}
		name := node.Value
		if !token.IsExported(name) {
			return "", errors.Errorf("extension %s: %q must be an exported Go identifier", key, name)
		}
		return name, nil
	}
	if g.opt.Initialisms {
		return lowering.PascalSpecialInitialisms(specName)
	}
	return lowering.PascalSpecial(specName)
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
