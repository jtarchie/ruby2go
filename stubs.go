package rb2go

import (
	"context"

	"github.com/jtarchie/ruby2go/internal/compiler"
)

// StubsPath is where PreludeStubs' output goes, relative to the repo root: the only prelude/go file the embed pattern leaves out.
const StubsPath = "prelude/go/0_stubs.go"

// PreludeStubs is prelude/go/0_stubs.go's content (decision 154): what the generated program declares and prelude/go
// names, bodies replaced by panic(0), so the helpers type-check under `-tags rb2go_prelude`.
func PreludeStubs(ctx context.Context) ([]byte, error) {
	return compiler.PreludeStubs(ctx, Prelude) //nolint:wrapcheck // thin wrapper
}
