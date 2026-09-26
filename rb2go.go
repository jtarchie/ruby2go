// Package rb2go compiles a typed subset of Ruby to Go.
package rb2go

import (
	"context"
	"embed"

	"rb2go/internal/compiler"
)

// Prelude holds prelude.rb and the files it require_relatives.
//
//go:embed prelude.rb prelude/*.rb
var Prelude embed.FS

// Compile transpiles the Ruby source of mainName (with the embedded prelude)
// into a single Go file.
func Compile(ctx context.Context, mainName string, src []byte) ([]byte, []string, error) {
	return compiler.CompileWithWarnings(ctx, Prelude, mainName, src)
}
