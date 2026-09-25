// Package agenerrors provides typed errors for generated message decoding and
// validation, decoupled from net/http (unlike ogen's ogenerrors).
package agenerrors

import (
	"fmt"

	"github.com/go-faster/errors"
)

// DecodeMessageError reports that a broker message could not be decoded.
type DecodeMessageError struct {
	// Operation is the operation name, e.g. "receiveLightMeasurement".
	Operation string
	// Message is the message name being decoded, when known.
	Message string
	// Err is the underlying error.
	Err error
}

// Unwrap implements errors.Wrapper.
func (e *DecodeMessageError) Unwrap() error { return e.Err }

// FormatError implements errors.Formatter.
func (e *DecodeMessageError) FormatError(p errors.Printer) (next error) {
	if e.Message != "" {
		p.Printf("decode %s message of %s", e.Message, e.Operation)
	} else {
		p.Printf("decode message of %s", e.Operation)
	}
	return e.Err
}

// Error implements error.
func (e *DecodeMessageError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("decode %s message of %s: %s", e.Message, e.Operation, e.Err)
	}
	return fmt.Sprintf("decode message of %s: %s", e.Operation, e.Err)
}

var _ error = (*DecodeMessageError)(nil)

// ValidateMessageError reports that a decoded message failed validation.
type ValidateMessageError struct {
	// Operation is the operation name.
	Operation string
	// Message is the message name.
	Message string
	// Err is the underlying validation error.
	Err error
}

// Unwrap implements errors.Wrapper.
func (e *ValidateMessageError) Unwrap() error { return e.Err }

// Error implements error.
func (e *ValidateMessageError) Error() string {
	return fmt.Sprintf("validate %s message of %s: %s", e.Message, e.Operation, e.Err)
}

// NotDispatchedError reports that a message matched none of the operation's
// message shapes (multi-message dispatch).
type NotDispatchedError struct {
	// Operation is the operation name.
	Operation string
	// ContentType of the incoming message.
	ContentType string
}

// Error implements error.
func (e *NotDispatchedError) Error() string {
	return fmt.Sprintf("message does not match any shape of %s (contentType %q)", e.Operation, e.ContentType)
}

// ErrNotImplemented is returned by unimplemented generated handlers.
var ErrNotImplemented = errors.New("not implemented")

// EncodeMessageError reports that a message could not be encoded for
// publishing.
type EncodeMessageError struct {
	// Operation is the operation name.
	Operation string
	// Message is the message name.
	Message string
	// Err is the underlying error.
	Err error
}

// Unwrap implements errors.Wrapper.
func (e *EncodeMessageError) Unwrap() error { return e.Err }

// Error implements error.
func (e *EncodeMessageError) Error() string {
	return fmt.Sprintf("encode %s message of %s: %s", e.Message, e.Operation, e.Err)
}
