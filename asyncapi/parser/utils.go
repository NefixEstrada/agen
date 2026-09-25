package parser

import (
	"sort"
	"strings"
)

// ptrLastSegment returns the last segment of a JSON pointer, e.g.
// "#/components/messages/lightMeasured" -> "lightMeasured".
func ptrLastSegment(ptr string) string {
	if i := strings.LastIndexByte(ptr, '/'); i >= 0 {
		return ptr[i+1:]
	}
	return ptr
}

// sortedKeys returns map keys in sorted order for deterministic output.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
