// Package rb2go compiles a typed subset of Ruby to Go.
package rb2go

import (
	"context"
	_ "embed"

	"rb2go/internal/compiler"
)

//go:embed prelude.rb
var Prelude []byte

// Compile transpiles the Ruby source of mainName (with the embedded prelude)
// into a single Go file.
func Compile(ctx context.Context, mainName string, src []byte) ([]byte, []string, error) {
	return compiler.CompileWithWarnings(ctx, Prelude, mainName, src)
}
