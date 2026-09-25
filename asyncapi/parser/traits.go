package parser

import (
	"strings"

	"github.com/go-faster/errors"

	"github.com/ogen-go/ogen/jsonpointer"

	"github.com/NefixEstrada/agen/asyncapi"
)

// Trait merge rule (AsyncAPI 3.0): a trait MUST NOT override the value of a
// field that is already present on the object. Conflicts are errors.

func setString(dst *string, src string, field string) error {
	if src == "" {
		return nil
	}
	if *dst != "" {
		return errors.Errorf("trait overrides %q", field)
	}
	*dst = src
	return nil
}

func mergeSecurity(dst *[]asyncapi.SecurityRequirement, src []asyncapi.SecurityRequirement, field string) error {
	if len(src) == 0 {
		return nil
	}
	if len(*dst) > 0 {
		return errors.Errorf("trait overrides %q", field)
	}
	*dst = src
	return nil
}

func mergeTags(dst *asyncapi.Tags, src asyncapi.Tags, field string) error {
	if len(src) == 0 {
		return nil
	}
	if len(*dst) > 0 {
		return errors.Errorf("trait overrides %q", field)
	}
	*dst = src
	return nil
}

// mergeOperationTraits merges operation traits (resolving component refs).
func (p *parser) mergeOperationTraits(dst *Operation, raw *asyncapi.Operation, ctx *jsonpointer.ResolveCtx) error {
	for _, trait := range raw.Traits {
		if trait == nil {
			continue
		}
		resolved, err := p.resolveOperationTrait(trait, ctx)
		if err != nil {
			return err
		}
		if err := setString(&dst.Title, resolved.Title, "title"); err != nil {
			return err
		}
		if err := setString(&dst.Summary, resolved.Summary, "summary"); err != nil {
			return err
		}
		if err := setString(&dst.Description, resolved.Description, "description"); err != nil {
			return err
		}
		if err := mergeSecurity(&dst.Security, resolved.Security, "security"); err != nil {
			return err
		}
		if err := mergeBindingsOp(&dst.Bindings, resolved.Bindings); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) resolveOperationTrait(trait *asyncapi.OperationTrait, ctx *jsonpointer.ResolveCtx) (*asyncapi.OperationTrait, error) {
	if trait.Ref == "" {
		return trait, nil
	}
	key, err := ctx.Key(trait.Ref)
	if err != nil {
		return nil, errors.Wrapf(err, "resolve %q", trait.Ref)
	}
	if err := ctx.AddKey(key, p.file(ctx)); err != nil {
		return nil, errors.Wrapf(err, "resolve %q", trait.Ref)
	}
	defer ctx.Delete(key)

	node, err := jsonpointer.Resolve(key.Ptr, p.root)
	if err != nil {
		return nil, errors.Wrapf(err, "resolve %q", trait.Ref)
	}
	var out asyncapi.OperationTrait
	if err := node.Decode(&out); err != nil {
		return nil, errors.Wrapf(err, "decode operation trait %q", trait.Ref)
	}
	if out.Ref != "" {
		return nil, errors.Errorf("operation trait %q is a chained reference", trait.Ref)
	}
	return &out, nil
}

// mergeBindingsOp merges per-protocol bindings, erroring on protocol conflicts.
func mergeBindingsOp(dst *asyncapi.OperationBindings, src asyncapi.OperationBindings) error {
	for proto, b := range src {
		if b == nil {
			continue
		}
		if _, ok := (*dst)[proto]; ok {
			return errors.Errorf("trait overrides bindings.%s", proto)
		}
		if *dst == nil {
			*dst = asyncapi.OperationBindings{}
		}
		(*dst)[proto] = b
	}
	return nil
}

func mergeBindingsChannel(dst *asyncapi.ChannelBindings, src asyncapi.ChannelBindings) error {
	for proto, b := range src {
		if b == nil {
			continue
		}
		if _, ok := (*dst)[proto]; ok {
			return errors.Errorf("trait overrides bindings.%s", proto)
		}
		if *dst == nil {
			*dst = asyncapi.ChannelBindings{}
		}
		(*dst)[proto] = b
	}
	return nil
}

func mergeBindingsMessage(dst *asyncapi.MessageBindings, src asyncapi.MessageBindings) error {
	for proto, b := range src {
		if b == nil {
			continue
		}
		if _, ok := (*dst)[proto]; ok {
			return errors.Errorf("trait overrides bindings.%s", proto)
		}
		if *dst == nil {
			*dst = asyncapi.MessageBindings{}
		}
		(*dst)[proto] = b
	}
	return nil
}

// mergeChannelTraits merges channel traits (resolving component refs).
func (p *parser) mergeChannelTraits(dst *Channel, raw *asyncapi.Channel, ctx *jsonpointer.ResolveCtx) error {
	for _, traitRef := range rawChannelTraits(raw) {
		trait, err := p.resolveChannelTrait(traitRef, ctx)
		if err != nil {
			return err
		}
		if err := setString(&dst.Title, trait.Title, "title"); err != nil {
			return err
		}
		if err := setString(&dst.Summary, trait.Summary, "summary"); err != nil {
			return err
		}
		if err := setString(&dst.Description, trait.Description, "description"); err != nil {
			return err
		}
		if err := mergeBindingsChannel(&dst.Bindings, trait.Bindings); err != nil {
			return err
		}
		for _, s := range trait.Servers {
			if s == nil {
				continue
			}
			if err := ctx.AddKey(jsonpointer.RefKey{}, p.file(ctx)); err != nil {
				return err
			}
			dst.Servers = append(dst.Servers, ptrLastSegment(strings.TrimPrefix(s.Ref, "#/servers/")))
		}
		// Parameters: trait may declare parameters the channel does not have.
		for pname, param := range trait.Parameters {
			if param == nil {
				continue
			}
			if _, ok := dst.Parameters[pname]; ok {
				return errors.Errorf("trait overrides parameters.%s", pname)
			}
			if dst.Parameters == nil {
				dst.Parameters = map[string]*Parameter{}
			}
			if param.Ref != "" {
				resolved, err := p.parseParameterPointer(param.Ref, param.Common.Locator, ctx)
				if err != nil {
					return err
				}
				resolved.Name = pname
				dst.Parameters[pname] = resolved
				continue
			}
			dst.Parameters[pname] = p.parameterFrom(pname, &param.Parameter)
		}
	}
	return nil
}

// rawChannelTraits returns the channel's traits.
func rawChannelTraits(ch *asyncapi.Channel) []*asyncapi.ChannelTrait { return ch.Traits }

func (p *parser) resolveChannelTrait(trait *asyncapi.ChannelTrait, ctx *jsonpointer.ResolveCtx) (*asyncapi.ChannelTrait, error) {
	if trait.Ref == "" {
		return trait, nil
	}
	key, err := ctx.Key(trait.Ref)
	if err != nil {
		return nil, errors.Wrapf(err, "resolve %q", trait.Ref)
	}
	if err := ctx.AddKey(key, p.file(ctx)); err != nil {
		return nil, errors.Wrapf(err, "resolve %q", trait.Ref)
	}
	defer ctx.Delete(key)

	node, err := jsonpointer.Resolve(key.Ptr, p.root)
	if err != nil {
		return nil, errors.Wrapf(err, "resolve %q", trait.Ref)
	}
	var out asyncapi.ChannelTrait
	if err := node.Decode(&out); err != nil {
		return nil, errors.Wrapf(err, "decode channel trait %q", trait.Ref)
	}
	if out.Ref != "" {
		return nil, errors.Errorf("channel trait %q is a chained reference", trait.Ref)
	}
	return &out, nil
}

// mergeMessageTraits merges message traits (resolving component refs).
func (p *parser) mergeMessageTraits(dst *Message, raw *asyncapi.Message, ctx *jsonpointer.ResolveCtx) error {
	for _, trait := range raw.Traits {
		if trait == nil {
			continue
		}
		resolved, err := p.resolveMessageTrait(trait, ctx)
		if err != nil {
			return err
		}
		if err := setString(&dst.Name, resolved.Name, "name"); err != nil {
			return err
		}
		if err := setString(&dst.Title, resolved.Title, "title"); err != nil {
			return err
		}
		if err := setString(&dst.Summary, resolved.Summary, "summary"); err != nil {
			return err
		}
		if err := setString(&dst.Description, resolved.Description, "description"); err != nil {
			return err
		}
		if err := setString(&dst.ContentType, resolved.ContentType, "contentType"); err != nil {
			return err
		}
		if err := mergeBindingsMessage(&dst.Bindings, resolved.Bindings); err != nil {
			return err
		}
		if resolved.Headers != nil && raw.Headers != nil {
			return errors.New("trait overrides headers")
		}
		if resolved.CorrelationID != nil && dst.CorrelationID != nil {
			return errors.New("trait overrides correlationId")
		}
	}
	return nil
}

func (p *parser) resolveMessageTrait(trait *asyncapi.MessageTrait, ctx *jsonpointer.ResolveCtx) (*asyncapi.MessageTrait, error) {
	if trait.Ref == "" {
		return trait, nil
	}
	key, err := ctx.Key(trait.Ref)
	if err != nil {
		return nil, errors.Wrapf(err, "resolve %q", trait.Ref)
	}
	if err := ctx.AddKey(key, p.file(ctx)); err != nil {
		return nil, errors.Wrapf(err, "resolve %q", trait.Ref)
	}
	defer ctx.Delete(key)

	node, err := jsonpointer.Resolve(key.Ptr, p.root)
	if err != nil {
		return nil, errors.Wrapf(err, "resolve %q", trait.Ref)
	}
	var out asyncapi.MessageTrait
	if err := node.Decode(&out); err != nil {
		return nil, errors.Wrapf(err, "decode message trait %q", trait.Ref)
	}
	if out.Ref != "" {
		return nil, errors.Errorf("message trait %q is a chained reference", trait.Ref)
	}
	return &out, nil
}
