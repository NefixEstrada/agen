package gen

import (
	"embed"
	stdjson "encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/go-faster/errors"

	"github.com/NefixEstrada/agen/internal/ir"
	"github.com/NefixEstrada/agen/internal/lowering"
	"github.com/NefixEstrada/agen/internal/naming"
)

// DefaultElem is variable helper for setting default values.
type DefaultElem struct {
	// Type is type of this DefaultElem.
	Type *ir.Type
	// Var is decoding/encoding variable Go name (obj) or selector (obj.Field).
	Var string
	// Default is default value to set.
	Default ir.Default
	// Depth is the recursion depth, used to derive unique local variable names
	// when rendering nested array/struct/map default literals.
	Depth int
}

// LocalVar returns a unique local accumulator variable name for this depth.
func (d DefaultElem) LocalVar() string {
	return fmt.Sprintf("defaultVal%d", d.Depth)
}

// NextDepth returns the depth for a nested DefaultElem.
func (d DefaultElem) NextDepth() int {
	return d.Depth + 1
}

// DefaultStructField pairs a struct field with the value present for it in an
// object default.
type DefaultStructField struct {
	Field *ir.Field
	Value any
}

// DefaultMapEntry is a single key/value of a map default, in sorted-key order.
type DefaultMapEntry struct {
	Key   string
	Value any
}

func defaultSlice(d ir.Default) []any {
	if !d.Set {
		return nil
	}
	s, _ := d.Value.([]any)
	return s
}

func defaultStructFields(t *ir.Type, d ir.Default) []DefaultStructField {
	if !d.Set {
		return nil
	}
	m, _ := d.Value.(map[string]any)
	var out []DefaultStructField
	for _, f := range t.Fields {
		if f.Spec == nil {
			continue
		}
		if v, ok := m[f.Spec.Name]; ok {
			out = append(out, DefaultStructField{Field: f, Value: v})
		}
	}
	return out
}

func defaultMapEntries(d ir.Default) []DefaultMapEntry {
	if !d.Set {
		return nil
	}
	m, _ := d.Value.(map[string]any)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]DefaultMapEntry, 0, len(keys))
	for _, k := range keys {
		out = append(out, DefaultMapEntry{Key: k, Value: m[k]})
	}
	return out
}

func defaultJSON(v any) (string, error) {
	b, err := stdjson.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Elem is variable helper for recursive array or object encoding or decoding.
type Elem struct {
	// Sub whether this Elem has parent Elem.
	Sub bool
	// Type is type of this Elem.
	Type *ir.Type
	// Var is decoding/encoding variable Go name (obj) or selector (obj.Field).
	Var string
	// Tag contains info about field tags, if any.
	Tag ir.Tag
	// First whether this field is first.
	First bool
}

// NextVar returns name of variable for decoding recursive call.
//
// Needed to make variable names unique.
func (e Elem) NextVar() string {
	if !e.Sub {
		// No recursion, returning default name.
		return "elem"
	}
	return e.Var + "Elem"
}

// OpMsgElem pairs an operation with one of its messages.
type OpMsgElem struct {
	Op      *ir.Operation
	Msg     *ir.Message
	Variant bool // true when the message is one of several (sum dispatch)
}

// EncodeFn returns the name of the generated encode function.
func (e OpMsgElem) EncodeFn() string {
	if e.Variant {
		return "encode" + e.Op.Name + e.Msg.Name
	}
	return "encode" + e.Op.Name + "Message"
}

// DecodeFn returns the name of the generated decode function.
func (e OpMsgElem) DecodeFn() string {
	return "decode" + e.Op.Name + e.Msg.Name
}

// NeedsValidation returns true if the envelope type has a generated Validate
// method.
func (e OpMsgElem) NeedsValidation() bool {
	return e.Msg.Type.NeedValidation()
}

// OperationElem is variable name for generating per-operation functions.
type OperationElem struct {
	// Operation is the operation.
	Operation *ir.Operation
	// Config is the template configuration.
	Config TemplateConfig
}

// templateFunctions returns functions which used in templates.
func templateFunctions() template.FuncMap {
	return template.FuncMap{
		"errorf": func(format string, args ...any) (any, error) {
			return nil, errors.Errorf(format, args...)
		},
		"pascalSpecial": pascalSpecial,
		"camelSpecial":  camelSpecial,
		"capitalize":    naming.Capitalize,
		"upper":         strings.ToUpper,

		// Helpers for recursive encoding and decoding.
		"elem": func(t *ir.Type, v string) Elem {
			return Elem{
				Type: t,
				Var:  v,
			}
		},
		"pointer_elem": func(parent Elem) Elem {
			return Elem{
				Type: parent.Type.PointerTo,
				Sub:  true,
				Var:  parent.NextVar(),
			}
		},
		// Recursive array element (e.g. array of arrays).
		"sub_array_elem": func(parent Elem, t *ir.Type) Elem {
			return Elem{
				Type: t,
				Sub:  true,
				Var:  parent.NextVar(),
			}
		},
		// Initial array element.
		"array_elem": func(t *ir.Type) Elem {
			return Elem{
				Type: t,
				Sub:  true,
				Var:  "elem",
			}
		},
		"map_elem": func(t *ir.Type) Elem {
			return Elem{
				Type: t,
				Sub:  true,
				Var:  "elem",
			}
		},
		// Field of structure.
		"field_elem": func(f *ir.Field) Elem {
			return Elem{
				Type: f.Type,
				Var:  fmt.Sprintf("s.%s", f.Name),
				Tag:  f.Tag,
			}
		},
		"first_field_elem": func(f *ir.Field) Elem {
			return Elem{
				Type:  f.Type,
				Var:   fmt.Sprintf("s.%s", f.Name),
				Tag:   f.Tag,
				First: true,
			}
		},
		"default_elem": func(t *ir.Type, v string, value ir.Default) DefaultElem {
			return DefaultElem{
				Type:    t,
				Var:     v,
				Default: value,
			}
		},
		"sub_default_elem": func(t *ir.Type, v string, val any, depth int) DefaultElem {
			return DefaultElem{
				Type: t,
				Var:  v,
				Default: ir.Default{
					Value: val,
					Set:   true,
				},
				Depth: depth,
			}
		},
		"default_slice":         defaultSlice,
		"default_struct_fields": defaultStructFields,
		"default_map_entries":   defaultMapEntries,
		"default_json":          defaultJSON,
		"op_msg": func(op *ir.Operation, msg *ir.Message) OpMsgElem {
			return OpMsgElem{Op: op, Msg: msg}
		},
		"op_msg_variant": func(op *ir.Operation, msg *ir.Message) OpMsgElem {
			return OpMsgElem{Op: op, Msg: msg, Variant: true}
		},
		"splitLines": func(s string) []string {
			s = strings.TrimRight(s, "\n")
			if s == "" {
				return nil
			}
			return strings.Split(s, "\n")
		},
		"join": strings.Join,
		"replaceParamsSprintf": func(address string) string {
			// streetlights.{id}.light -> "streetlights.%s.light"
			var b strings.Builder
			for {
				start := strings.IndexByte(address, '{')
				if start < 0 {
					b.WriteString(address)
					return b.String()
				}
				end := strings.IndexByte(address[start:], '}')
				if end < 0 {
					b.WriteString(address)
					return b.String()
				}
				b.WriteString(address[:start])
				b.WriteString("%s")
				address = address[start+end+1:]
			}
		},
		"op_elem": func(op *ir.Operation, cfg TemplateConfig) OperationElem {
			return OperationElem{
				Operation: op,
				Config:    cfg,
			}
		},
		"print_go": ir.PrintGoValue,
		// We use any to prevent template type matching errors
		// for type aliases (e.g. for quoting ir.ContentType).
		"quote": func(v any) string {
			// Fast path for string.
			if s, ok := v.(string); ok {
				return strconv.Quote(s)
			}
			return fmt.Sprintf("%q", v)
		},
		"backquote": func(v any) string {
			// Fast path for string.
			if s, ok := v.(string); ok && strconv.CanBackquote(s) {
				return "`" + s + "`"
			}
			return fmt.Sprintf("%#q", v)
		},
		"times": func(n int) []struct{} {
			return make([]struct{}, n)
		},
		"add": func(a, b int) int {
			return a + b
		},
		"div": func(a, b int) int {
			return a / b
		},
		"mod": func(a, b int) int {
			return a % b
		},
		"dedupeVariantsByType":             dedupeVariantsByType,
		"needsArrayElementDiscrimination":  needsArrayElementDiscrimination,
		"dedupeVariantsByArrayElementType": dedupeVariantsByArrayElementType,
	}
}

//go:embed _template/*.tmpl
var templates embed.FS

//go:embed _template/*/*.tmpl
var templatesSub embed.FS

var _templates struct {
	sync.Once
	val *template.Template
}

// vendoredTemplates parses and returns vendored code generation templates.
func vendoredTemplates() *template.Template {
	_templates.Do(func() {
		tmpl := template.New("templates").Funcs(templateFunctions())
		tmpl = template.Must(tmpl.ParseFS(templates, "_template/*.tmpl"))
		tmpl = template.Must(tmpl.ParseFS(templatesSub, "_template/*/*.tmpl"))
		_templates.val = tmpl
	})
	return _templates.val
}

// pascalSpecial converts a string to PascalCase, allowing special characters.
func pascalSpecial(strs ...string) (string, error) { return lowering.PascalSpecial(strs...) }

// camelSpecial converts a string to camelCase, allowing special characters.
func camelSpecial(s ...string) (string, error) { return lowering.CamelSpecial(s...) }

const jxTypeArray = "jx.Array"

// dedupeVariantsByType deduplicates variants by their FieldType to avoid
// duplicate type checks. When multiple variants have the same field type (or
// no type discrimination), keep only unique entries.
func dedupeVariantsByType(variants []ir.UniqueFieldVariant) []ir.UniqueFieldVariant {
	if len(variants) == 0 {
		return variants
	}

	seen := make(map[string]bool)
	result := make([]ir.UniqueFieldVariant, 0, len(variants))

	for _, v := range variants {
		// If FieldType is empty (no type discrimination), include all variants.
		if v.FieldType == "" || !seen[v.FieldType] {
			if v.FieldType != "" {
				seen[v.FieldType] = true
			}
			result = append(result, v)
		}
	}

	return result
}

// needsArrayElementDiscrimination checks if all variants have the same
// jx.Array FieldType but different ArrayElementTypes, requiring element-level
// discrimination.
func needsArrayElementDiscrimination(variants []ir.UniqueFieldVariant) bool {
	if len(variants) < 2 {
		return false
	}

	// All variants must be arrays.
	for _, v := range variants {
		if v.FieldType != jxTypeArray {
			return false
		}
	}

	// Count unique element types.
	uniqueElemTypes := make(map[string]bool)
	for _, v := range variants {
		if v.ArrayElementType != "" {
			uniqueElemTypes[v.ArrayElementType] = true
		}
	}

	return len(uniqueElemTypes) > 1
}

// dedupeVariantsByArrayElementType deduplicates array variants by their
// ArrayElementType. Used when all variants are arrays that need element-level
// discrimination.
func dedupeVariantsByArrayElementType(variants []ir.UniqueFieldVariant) []ir.UniqueFieldVariant {
	if len(variants) == 0 {
		return variants
	}

	seen := make(map[string]bool)
	result := make([]ir.UniqueFieldVariant, 0, len(variants))

	for _, v := range variants {
		// If ArrayElementType is empty, include the variant.
		if v.ArrayElementType == "" || !seen[v.ArrayElementType] {
			if v.ArrayElementType != "" {
				seen[v.ArrayElementType] = true
			}
			result = append(result, v)
		}
	}

	return result
}
