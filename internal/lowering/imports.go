package lowering

// DefaultImports returns a map of default imports for the generated code.
// The keys are the import paths, and the values are the aliases (empty string means no alias).
func DefaultImports() map[string]string { return defaultImports() }

// defaultImports returns a map of default imports for the generated code.
// The keys are the import paths, and the values are the aliases (empty string means no alias).
func defaultImports() map[string]string {
	return map[string]string{
		"bytes":     "",
		"context":   "",
		"fmt":       "",
		"io":        "",
		"math":      "",
		"math/big":  "",
		"math/bits": "",
		"mime":      "",
		"net":       "",
		"net/http":  "",
		"net/netip": "",
		"net/url":   "",
		"regexp":    "",
		"sort":      "",
		"strconv":   "",
		"strings":   "",
		"sync":      "",
		"time":      "",

		"github.com/go-faster/errors":   "",
		"github.com/go-faster/jx":       "",
		"github.com/google/uuid":        "",
		"github.com/shopspring/decimal": "",

		"github.com/NefixEstrada/agen/runtime/conv":      "conv",
		"github.com/NefixEstrada/agen/runtime/codec":     "json",
		"github.com/NefixEstrada/agen/runtime/ogenregex": "ogenregex",
		"github.com/NefixEstrada/agen/runtime/validate":  "validate",
	}
}
