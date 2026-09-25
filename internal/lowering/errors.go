package lowering

import (
	"fmt"
	"strings"

	"github.com/go-faster/errors"

	"github.com/NefixEstrada/agen/internal/ir"
)

type unimplementedError interface {
	unimplemented()
	error
}

// ErrNotImplemented reports that feature is not implemented.
type ErrNotImplemented struct {
	Name string
}

func (e *ErrNotImplemented) unimplemented() {}

// Error implements error.
func (e *ErrNotImplemented) Error() string {
	return e.Name + " not implemented"
}

// ErrFieldsDiscriminatorInference reports fields discriminator inference failure.
type ErrFieldsDiscriminatorInference struct {
	Sum   *ir.Type
	Types []BadVariant
}

func (e *ErrFieldsDiscriminatorInference) unimplemented() {}

// Error implements error.
func (e *ErrFieldsDiscriminatorInference) Error() string {
	names := make([]string, len(e.Types))
	for i, typ := range e.Types {
		names[i] = typ.Type.Name
	}
	return fmt.Sprintf("can't infer fields discriminator: [%s]", strings.Join(names, ", "))
}

// BadVariant describes a sum type variant for what we unable to infer discriminator.
type BadVariant struct {
	Type   *ir.Type
	Fields map[string][]*ir.Type
}

var _ unimplementedError = (*ErrNotImplemented)(nil)
var _ unimplementedError = (*ErrFieldsDiscriminatorInference)(nil)
var _ = errors.Into[*ErrNotImplemented]
