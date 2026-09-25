package lowering

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/ogen-go/ogen/location"
)

func unreachable(v any) string {
	return fmt.Sprintf("unreachable: %v", v)
}

type position interface {
	Position() (location.Position, bool)
	File() location.File
}

func zapPosition(l position) zap.Field {
	if l == nil {
		return zap.Skip()
	}
	loc, ok := l.Position()
	if !ok {
		return zap.Skip()
	}
	file := l.File()
	return zap.String("at", loc.WithFilename(file.Name))
}
