// Package rb2go compiles a typed subset of Ruby to Go.
package rb2go

import (
	"context"
	"embed"
	"fmt"

	"rb2go/internal/compiler"
)

// Prelude holds prelude.rb, its require_relatives, and prelude/go/*.go (pure-Go helpers embedded directly; see loadPreludeGo).
//go:embed prelude.rb prelude/*.rb prelude/go/*.go
var Prelude embed.FS

// Compile transpiles the Ruby source of mainName (with the embedded prelude)
// into a single Go file.
func Compile(ctx context.Context, mainName string, src []byte) ([]byte, []string, error) {
	out, warnings, err := compiler.CompileWithWarnings(ctx, Prelude, mainName, src)
	if err != nil {
		return nil, warnings, fmt.Errorf("rb2go: %w", err)
	}
	return out, warnings, nil
}
