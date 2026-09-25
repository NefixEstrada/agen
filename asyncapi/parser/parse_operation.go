package parser

import (
	"strings"

	"github.com/go-faster/errors"

	"github.com/ogen-go/ogen/jsonpointer"
	"github.com/ogen-go/ogen/location"

	"github.com/NefixEstrada/agen/asyncapi"
)

// parseOperations parses the `operations` map, linking each operation to its
// channel and messages.
func (p *parser) parseOperations(api *API, ctx *jsonpointer.ResolveCtx) error {
	for _, name := range sortedKeys(p.spec.Operations) {
		op := p.spec.Operations[name]
		if op == nil {
			continue
		}
		if err := p.parseOperation(api, name, op, ctx); err != nil {
			return errors.Wrapf(err, "operation %q", name)
		}
	}
	return nil
}

func (p *parser) parseOperation(api *API, name string, op *asyncapi.Operation, ctx *jsonpointer.ResolveCtx) error {
	file := p.file(ctx)
	if op.Action != string(SendAction) && op.Action != string(ReceiveAction) {
		err := errors.Errorf("invalid action %q: must be %q or %q", op.Action, SendAction, ReceiveAction)
		return p.wrapField("action", file, op.Common.Locator, err)
	}

	semantic := &Operation{
		Name:        name,
		Action:      Action(op.Action),
		Title:       op.Title,
		Summary:     op.Summary,
		Description: op.Description,
		Security:    op.Security,
		Bindings:    op.Bindings,
		Tags:        op.Tags,
		Common:      op.Common,
	}

	// Merge operation traits; a trait MUST NOT override existing fields.
	if err := p.mergeOperationTraits(semantic, op, ctx); err != nil {
		return err
	}

	if op.Channel == nil {
		err := errors.New("channel is required")
		return p.wrapField("channel", file, op.Common.Locator, err)
	}
	ch, err := p.parseChannelRef(op.Channel, name, ctx)
	if err != nil {
		return p.wrapField("channel", file, op.Common.Locator, err)
	}
	semantic.Channel = ch

	// Operation-level messages win over channel-level messages.
	switch {
	case len(op.Messages) > 0:
		messages, err := p.parseMessages(op.Messages, ch.Name, ctx)
		if err != nil {
			return p.wrapField("messages", file, op.Common.Locator, err)
		}
		semantic.Messages = messages
	case len(ch.Messages) > 0:
		semantic.Messages = ch.Messages
	default:
		err := errors.New("no messages: define messages on the operation or on its channel")
		return p.wrapField("messages", file, op.Common.Locator, err)
	}

	if op.Reply != nil {
		reply, err := p.parseReply(op.Reply, ctx)
		if err != nil {
			return p.wrapField("reply", file, op.Common.Locator, err)
		}
		semantic.Reply = reply
	}

	api.Operations = append(api.Operations, semantic)
	return nil
}

func (p *parser) parseReply(reply *asyncapi.OperationReply, ctx *jsonpointer.ResolveCtx) (*Reply, error) {
	if reply.Channel == nil {
		err := errors.New("reply channel is required")
		return nil, p.wrapField("channel", p.file(ctx), reply.Common.Locator, err)
	}
	ch, err := p.parseChannelRef(reply.Channel, "reply", ctx)
	if err != nil {
		return nil, err
	}
	r := &Reply{
		Channel:     ch,
		Title:       reply.Title,
		Summary:     reply.Summary,
		Description: reply.Description,
		Common:      reply.Common,
	}
	switch {
	case len(reply.Messages) > 0:
		r.Messages, err = p.parseMessages(reply.Messages, ch.Name, ctx)
		if err != nil {
			return nil, err
		}
	case len(ch.Messages) > 0:
		r.Messages = ch.Messages
	default:
		err := errors.New("reply has no messages: define messages on the reply or on its channel")
		return nil, p.wrapField("messages", p.file(ctx), reply.Common.Locator, err)
	}
	return r, nil
}

// parseChannelRef resolves a channel (referenced or inline) to the semantic
// model. Parsed channels are cached by their JSON pointer so references and
// recursion behave.
func (p *parser) parseChannelRef(ref *asyncapi.ChannelRef, inlineName string, ctx *jsonpointer.ResolveCtx) (*Channel, error) {
	if ref.Ref != "" {
		return p.parseChannelPointer(ref.Ref, ref.Common.Locator, ctx)
	}
	// Inline channel: no caching, it exists only here.
	c, err := p.parseChannel(inlineName, &ref.Channel, ctx)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (p *parser) parseChannelPointer(ref string, loc location.Locator, ctx *jsonpointer.ResolveCtx) (*Channel, error) {
	key, err := ctx.Key(ref)
	if err != nil {
		return nil, p.wrapLocation(p.file(ctx), loc.Field("$ref"), errors.Wrapf(err, "resolve %q", ref))
	}
	if cached, ok := p.channels[key.Ptr]; ok {
		return cached, nil
	}
	if err := ctx.AddKey(key, p.file(ctx)); err != nil {
		return nil, p.wrapLocation(p.file(ctx), loc.Field("$ref"), errors.Wrapf(err, "resolve %q", ref))
	}
	defer ctx.Delete(key)

	node, err := jsonpointer.Resolve(key.Ptr, p.root)
	if err != nil {
		return nil, p.wrapLocation(p.file(ctx), loc.Field("$ref"), errors.Wrapf(err, "resolve %q", ref))
	}
	var raw asyncapi.Channel
	if err := node.Decode(&raw); err != nil {
		return nil, p.wrapLocation(p.file(ctx), loc.Field("$ref"), errors.Wrapf(err, "decode channel %q", ref))
	}

	name := ptrLastSegment(key.Ptr)
	semantic, err := p.parseChannel(name, &raw, ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "channel %q", name)
	}
	p.channels[key.Ptr] = semantic
	return semantic, nil
}

func (p *parser) parseChannel(name string, raw *asyncapi.Channel, ctx *jsonpointer.ResolveCtx) (_ *Channel, rerr error) {
	file := p.file(ctx)
	defer func() {
		if rerr != nil {
			rerr = p.wrapLocation(file, raw.Common.Locator, rerr)
		}
	}()

	if raw.Address == "" {
		err := errors.New("channel.address is required")
		return nil, p.wrapField("address", file, raw.Common.Locator, err)
	}
	semantic := &Channel{
		Name:        name,
		Address:     raw.Address,
		Title:       raw.Title,
		Summary:     raw.Summary,
		Description: raw.Description,
		Bindings:    raw.Bindings,
		Tags:        raw.Tags,
		Common:      raw.Common,
	}

	// Resolve channel servers.
	for _, s := range raw.Servers {
		if s == nil {
			continue
		}
		serverName, err := p.parseServerRef(s, ctx)
		if err != nil {
			return nil, p.wrapField("servers", file, raw.Common.Locator, err)
		}
		semantic.Servers = append(semantic.Servers, serverName)
	}

	// Merge channel traits (from components.channelTraits or inline).
	if err := p.mergeChannelTraits(semantic, raw, ctx); err != nil {
		return nil, p.wrapField("traits", file, raw.Common.Locator, err)
	}

	// Resolve parameters.
	semantic.Parameters = map[string]*Parameter{}
	for _, pname := range sortedKeys(raw.Parameters) {
		pref := raw.Parameters[pname]
		if pref == nil {
			continue
		}
		var param *Parameter
		if pref.Ref != "" {
			var err error
			param, err = p.parseParameterPointer(pref.Ref, pref.Common.Locator, ctx)
			if err != nil {
				return nil, p.wrapField("parameters", file, raw.Common.Locator, err)
			}
			// The map key wins over the component name.
			param.Name = pname
		} else {
			param = p.parameterFrom(pname, &pref.Parameter)
		}
		semantic.Parameters[pname] = param
	}

	// Parse channel-level messages (lazily; operations may override).
	if len(raw.Messages) > 0 {
		messages, err := p.parseMessages(raw.Messages, name, ctx)
		if err != nil {
			return nil, p.wrapField("messages", file, raw.Common.Locator, err)
		}
		semantic.Messages = messages
	}

	// Validate that every address template parameter is declared.
	if err := validateAddressParams(semantic); err != nil {
		return nil, p.wrapField("address", file, raw.Common.Locator, err)
	}
	return semantic, nil
}

func validateAddressParams(ch *Channel) error {
	for _, token := range addressTokens(ch.Address) {
		if _, ok := ch.Parameters[token]; !ok {
			return errors.Errorf("address parameter %q is not declared in channel parameters", token)
		}
	}
	return nil
}

// addressTokens extracts `{token}` template parameters from a channel address.
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

func (p *parser) parseServerRef(ref *asyncapi.ServerRef, ctx *jsonpointer.ResolveCtx) (string, error) {
	// Channel servers are references like `{$ref: '#/servers/production'}`.
	key, err := ctx.Key(ref.Ref)
	if err != nil {
		return "", errors.Wrapf(err, "resolve %q", ref.Ref)
	}
	if err := ctx.AddKey(key, p.file(ctx)); err != nil {
		return "", errors.Wrapf(err, "resolve %q", ref.Ref)
	}
	defer ctx.Delete(key)

	if _, err := jsonpointer.Resolve(key.Ptr, p.root); err != nil {
		return "", errors.Wrapf(err, "resolve %q", ref.Ref)
	}
	return ptrLastSegment(key.Ptr), nil
}

func (p *parser) parseParameterPointer(ref string, loc location.Locator, ctx *jsonpointer.ResolveCtx) (*Parameter, error) {
	key, err := ctx.Key(ref)
	if err != nil {
		return nil, p.wrapLocation(p.file(ctx), loc.Field("$ref"), errors.Wrapf(err, "resolve %q", ref))
	}
	if cached, ok := p.parameters[key.Ptr]; ok {
		return cached, nil
	}
	if err := ctx.AddKey(key, p.file(ctx)); err != nil {
		return nil, p.wrapLocation(p.file(ctx), loc.Field("$ref"), errors.Wrapf(err, "resolve %q", ref))
	}
	defer ctx.Delete(key)

	node, err := jsonpointer.Resolve(key.Ptr, p.root)
	if err != nil {
		return nil, p.wrapLocation(p.file(ctx), loc.Field("$ref"), errors.Wrapf(err, "resolve %q", ref))
	}
	var raw asyncapi.Parameter
	if err := node.Decode(&raw); err != nil {
		return nil, p.wrapLocation(p.file(ctx), loc.Field("$ref"), errors.Wrapf(err, "decode parameter %q", ref))
	}
	semantic := p.parameterFrom(ptrLastSegment(key.Ptr), &raw)
	p.parameters[key.Ptr] = semantic
	return semantic, nil
}

func (p *parser) parameterFrom(name string, raw *asyncapi.Parameter) *Parameter {
	return &Parameter{
		Name:        name,
		Enum:        raw.Enum,
		Default:     raw.Default,
		Description: raw.Description,
		Location:    raw.Location,
		Common:      raw.Common,
	}
}

// parseMessages parses a Messages map (name → message ref or inline).
func (p *parser) parseMessages(messages asyncapi.Messages, channelName string, ctx *jsonpointer.ResolveCtx) ([]*Message, error) {
	var out []*Message
	for _, name := range sortedKeys(messages) {
		ref := messages[name]
		if ref == nil {
			continue
		}
		msg, err := p.parseMessageRef(ref, ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "message %q", name)
		}
		// The messages map key identifies the message when the message
		// object itself has no name and is not a components ref.
		if msg.SpecName == "" {
			msg.SpecName = name
			if msg.Name == "" {
				msg.Name = name
			}
		}
		out = append(out, msg)
	}
	return out, nil
}

func (p *parser) parseMessageRef(ref *asyncapi.MessageRef, ctx *jsonpointer.ResolveCtx) (*Message, error) {
	if ref.Ref != "" {
		return p.parseMessagePointer(ref.Ref, ref.Common.Locator, ctx)
	}
	msg, err := p.parseMessage("", &ref.Message, ctx)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (p *parser) parseMessagePointer(ref string, loc location.Locator, ctx *jsonpointer.ResolveCtx) (*Message, error) {
	key, err := ctx.Key(ref)
	if err != nil {
		return nil, p.wrapLocation(p.file(ctx), loc.Field("$ref"), errors.Wrapf(err, "resolve %q", ref))
	}
	if cached, ok := p.messages[key.Ptr]; ok {
		return cached, nil
	}
	if err := ctx.AddKey(key, p.file(ctx)); err != nil {
		return nil, p.wrapLocation(p.file(ctx), loc.Field("$ref"), errors.Wrapf(err, "resolve %q", ref))
	}
	defer ctx.Delete(key)

	node, err := jsonpointer.Resolve(key.Ptr, p.root)
	if err != nil {
		return nil, p.wrapLocation(p.file(ctx), loc.Field("$ref"), errors.Wrapf(err, "resolve %q", ref))
	}
	var raw asyncapi.MessageRef
	if err := node.Decode(&raw); err != nil {
		return nil, p.wrapLocation(p.file(ctx), loc.Field("$ref"), errors.Wrapf(err, "decode message %q", ref))
	}
	if raw.Ref != "" {
		return nil, p.wrapLocation(p.file(ctx), loc.Field("$ref"), errors.Errorf("message %q is a chained reference", ref))
	}

	semantic, err := p.parseMessage(ptrLastSegment(key.Ptr), &raw.Message, ctx)
	if err != nil {
		return nil, err
	}
	p.messages[key.Ptr] = semantic
	return semantic, nil
}

func (p *parser) parseMessage(name string, raw *asyncapi.Message, ctx *jsonpointer.ResolveCtx) (_ *Message, rerr error) {
	file := p.file(ctx)
	defer func() {
		if rerr != nil {
			rerr = p.wrapLocation(file, raw.Common.Locator, rerr)
		}
	}()

	if err := validateSchemaFormat(raw.SchemaFormat, raw.Common.Locator, file); err != nil {
		return nil, p.wrapField("schemaFormat", file, raw.Common.Locator, err)
	}

	semantic := &Message{
		SpecName:      name,
		ContentType:   raw.ContentType,
		Name:          raw.Name,
		Title:         raw.Title,
		Summary:       raw.Summary,
		Description:   raw.Description,
		CorrelationID: raw.CorrelationID,
		Bindings:      raw.Bindings,
		Tags:          raw.Tags,
		Common:        raw.Common,
	}
	if semantic.Name == "" {
		semantic.Name = semantic.SpecName
	}

	// Merge message traits before parsing schemas so trait headers can be used.
	if err := p.mergeMessageTraits(semantic, raw, ctx); err != nil {
		return nil, p.wrapField("traits", file, raw.Common.Locator, err)
	}

	if raw.Payload != nil {
		s, err := p.schemaParser.Parse(raw.Payload, ctx)
		if err != nil {
			return nil, p.wrapField("payload", file, raw.Common.Locator, err)
		}
		semantic.Payload = s
	}
	if raw.Headers != nil {
		s, err := p.schemaParser.Parse(raw.Headers, ctx)
		if err != nil {
			return nil, p.wrapField("headers", file, raw.Common.Locator, err)
		}
		semantic.Headers = s
	}

	contentType := semantic.ContentType
	if contentType == "" {
		contentType = defaultContentType(p.spec)
	}
	if err := validateContentType(contentType); err != nil {
		return nil, p.wrapField("contentType", file, raw.Common.Locator, err)
	}
	semantic.ContentType = contentType

	return semantic, nil
}

// validateSchemaFormat errors clearly on non-default schema formats until
// support for them lands.
func validateSchemaFormat(format string, loc location.Locator, file location.File) error {
	switch format {
	case "", "application/vnd.aai.asyncapi", "application/vnd.aai.asyncapi+json",
		"application/vnd.aai.asyncapi;version=3.0.0", "application/vnd.aai.asyncapi+json;version=3.0.0",
		"application/vnd.aai.asyncapi;version=3.1.0", "application/vnd.aai.asyncapi+json;version=3.1.0":
		return nil
	default:
		if strings.Contains(format, "avro") {
			return errors.Errorf("schema format %q (Avro) is not supported yet", format)
		}
		return errors.Errorf("schema format %q is not supported", format)
	}
}

// validateContentType checks the content type against the known codecs table.
func validateContentType(ct string) error {
	if ct == "application/json" || strings.HasSuffix(ct, "+json") {
		return nil
	}
	return errors.Errorf("unsupported content type %q: only JSON codecs are generated", ct)
}
