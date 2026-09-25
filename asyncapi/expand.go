package asyncapi

import (
	"strings"

	"github.com/go-faster/errors"
	"github.com/go-faster/yaml"

	"github.com/ogen-go/ogen/jsonpointer"
)

// Expand inlines every internal `$ref` of the document tree in place,
// producing a fully-dereferenced document suitable for dumping (the
// `expand.output` option).
//
// External references (other files or URLs) are left untouched, as are
// references that would expand into themselves (recursive schemas): both keep
// their `$ref` form so the document stays valid.
func Expand(root *yaml.Node) error {
	doc := documentNode(root)
	if doc == nil {
		return errors.New("expand: document is empty")
	}
	e := &expander{
		root: doc,
		seen: map[string]*yaml.Node{},
	}
	return e.expandMapping(doc)
}

type expander struct {
	// root is the document root mapping (document wrapper stripped).
	root *yaml.Node
	// seen maps the pointer currently being expanded to its target node,
	// detecting cycles.
	seen map[string]*yaml.Node
}

func documentNode(root *yaml.Node) *yaml.Node {
	if root == nil {
		return nil
	}
	if root.Kind == yaml.DocumentNode && len(root.Content) == 1 {
		return root.Content[0]
	}
	return root
}

func (e *expander) expandMapping(n *yaml.Node) error {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		value := n.Content[i+1]
		if value.Kind == yaml.MappingNode {
			// A mapping that is a reference (`{$ref: "#/..."}`) is replaced
			// entirely by its expanded target.
			if ref, ok := internalRef(value); ok {
				target, err := e.resolveRef(ref)
				if err != nil {
					return errors.Wrapf(err, "expand %q", ref)
				}
				if target != nil {
					n.Content[i+1] = target
					continue
				}
				// External or cyclic: keep the reference, do not descend.
				continue
			}
			if err := e.expandMapping(value); err != nil {
				return err
			}
			continue
		}
		if value.Kind == yaml.SequenceNode {
			for _, item := range value.Content {
				if err := e.expandMapping(item); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// internalRef returns the pointer when the mapping is an internal reference
// object (`{$ref: "#/..."}`, optionally with summary/description as JSON
// References allow).
func internalRef(n *yaml.Node) (string, bool) {
	var (
		ref     string
		hasRef  bool
		extraOK = true
	)
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := n.Content[i], n.Content[i+1]
		if key.Kind != yaml.ScalarNode {
			return "", false
		}
		switch key.Value {
		case "$ref":
			if value.Kind == yaml.ScalarNode && strings.HasPrefix(value.Value, "#") {
				ref, hasRef = value.Value, true
			}
		case "summary", "description":
			// allowed companions of $ref
		default:
			extraOK = false
		}
	}
	return ref, hasRef && extraOK
}

// resolveRef returns an inlined copy of the target of an internal reference,
// or nil when the reference must be kept (external or cyclic).
func (e *expander) resolveRef(ref string) (*yaml.Node, error) {
	if !strings.HasPrefix(ref, "#") {
		return nil, nil // external reference: keep as-is
	}
	if _, cycling := e.seen[ref]; cycling {
		return nil, nil // recursive definition: keep as-is
	}
	target, err := jsonpointer.Resolve(ref, e.root)
	if err != nil {
		return nil, err
	}

	e.seen[ref] = target
	defer delete(e.seen, ref)

	clone := deepCopy(target)
	if err := e.expandMapping(clone); err != nil {
		return nil, err
	}
	return clone, nil
}

func deepCopy(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	clone := &yaml.Node{
		Kind:        n.Kind,
		Style:       n.Style,
		Tag:         n.Tag,
		Value:       n.Value,
		Anchor:      "",
		Alias:       nil,
		HeadComment: n.HeadComment,
		LineComment: n.LineComment,
		FootComment: n.FootComment,
		Line:        n.Line,
		Column:      n.Column,
	}
	clone.Content = make([]*yaml.Node, len(n.Content))
	for i, c := range n.Content {
		clone.Content[i] = deepCopy(c)
	}
	return clone
}
