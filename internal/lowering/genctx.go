package lowering

import (
	"github.com/NefixEstrada/agen/internal/ir"
	"github.com/ogen-go/ogen/jsonschema"
)

// genctx is a generation context.
type genctx struct {
	global *tstorage // readonly
	local  *tstorage
}

func (g *genctx) saveType(t *ir.Type) error {
	return g.local.saveType(t)
}

func (g *genctx) saveRef(ref jsonschema.Ref, e ir.Encoding, t *ir.Type) error {
	return g.local.saveRef(ref, e, t)
}

func (g *genctx) lookupRef(ref jsonschema.Ref, e ir.Encoding) (*ir.Type, bool) {
	key := schemaKey{ref, e}
	if t, ok := g.global.refs[key]; ok {
		return t, true
	}
	if t, ok := g.local.refs[key]; ok {
		return t, true
	}
	return nil, false
}
