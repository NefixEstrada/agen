package asyncapi

import (
	"strings"

	"github.com/go-faster/yaml"
)

// agenExtensionPrefix is agen's specification-extension prefix; it mirrors
// ogen's `x-ogen-` family for muscle memory.
const agenExtensionPrefix = "x-agen-"

// AliasXExtensions rewrites `x-agen-*` mapping keys of the document tree into
// their `x-ogen-*` equivalents, so the vendored ogen engine (schema naming,
// custom types, properties, time formats, validate hooks) honors them.
//
// Call it on the root yaml.Node BEFORE decoding the document into Spec. The
// semantic model also reads `x-agen-name` / `x-ogen-name` on operations,
// channels and messages for Go name overrides.
func AliasXExtensions(root *yaml.Node) {
	walkRenameExtensions(root)
}

func walkRenameExtensions(nodes ...*yaml.Node) {
	for _, n := range nodes {
		if n == nil {
			continue
		}
		switch n.Kind {
		case yaml.DocumentNode:
			if len(n.Content) == 1 {
				walkRenameExtensions(n.Content[0])
			}
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				key, value := n.Content[i], n.Content[i+1]
				if key.Kind == yaml.ScalarNode && strings.HasPrefix(key.Value, agenExtensionPrefix) {
					key.Value = "x-ogen-" + strings.TrimPrefix(key.Value, agenExtensionPrefix)
				}
				walkRenameExtensions(value)
			}
		case yaml.SequenceNode:
			walkRenameExtensions(n.Content...)
		}
	}
}
