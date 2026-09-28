# CLAUDE.md

rb2go: transpiles a typed subset of Ruby (rbs-inline `#:` annotations) to a single Go file. README.md is the design record. Its numbered "Open decisions" list (1–36) is binding: read the relevant entry before changing behavior, and add or amend an entry when a new decision is made.

## Commands

```sh
bundle install                      # rbs, rbs-inline, webrick (needed by tests)
go test ./...                       # the oracle; see below
go test -run 'TestExamples/05_word_count' .   # one example
go test -run 'TestRun/^string_' .             # behaviour snippets by prefix
go test -run 'TestErrors/^regexp$' .          # one compile-error archive
RB2GO_RUN_SKIPPED=1 go test -run TestRun .    # also run `# skip:` known failures
RB2GO_NO_MRI_CACHE=1 go test ./...            # rerun MRI instead of its cached output (~/Library/Caches/rb2go-test/mri)
RB2GO_NO_PRUNE=1 go run ./cmd/rb2go build -work ...  # emit the whole prelude (debugging the pruner)
go test -run '^$' -fuzz FuzzCompile -fuzztime 60s .              # fuzz targets: FuzzCompile (.),
go test -run '^$' -fuzz FuzzParseType -fuzztime 30s ./internal/rbs # FuzzParseType/FuzzParseMethodType (rbs),
go test -run '^$' -fuzz FuzzTranslateRegexp ./internal/compiler    # FuzzGoMethodName/FuzzTranslateRegexp (compiler)
go test ./internal/rbs              # RBS parser unit tests
golangci-lint run ./...             # repo lint (.golangci.yml)
go run ./cmd/rb2go run examples/NN_x/main.rb        # transpile, build, run; warnings go to stderr
go run ./cmd/rb2go build -work -gcflags=-e examples/NN_x/main.rb  # keep main.go (WORK= path on stderr), show every Go error
```

`go test` fails fast unless `ruby` ≥ 4.0, `bundle install` has been run, and `golangci-lint` is on PATH. Gem executables are not on PATH on this machine, so use `bundle exec rbs-inline ...` for manual runs. Stderr lines like rdoc "already initialized constant" warnings are noise.

## What a passing example means

For each `examples/*/main.rb`, `TestExamples` (rb2go_test.go):
1. runs `rbs-inline` + `rbs validate`. A `require "x/y"` line adds `-r x-y`, so annotations must be valid RBS.
2. runs `Compile`, then `gofmt`, `go vet`, `golangci-lint` with `.golangci.generated.yml` (the generated Go must lint clean), and `go build`.
3. requires stdout **and** exit code to equal `ruby main.rb`.

Generated programs are built with `-race -trimpath` (`-trimpath` lets the build cache hit across temp dirs; the first run after a Go upgrade rebuilds the race std once). Examples are vetted and linted in one pass over a shared module after all of them pass. MRI output is cached by source, Ruby version and TZ.

Add one new example per new feature, numbered next in sequence. User code must run on MRI unchanged. There are no golden Go files; MRI's output is the only expectation.

Smaller cases go in `testdata/`:
- `testdata/run/<area>_*.rb`: `TestRun` gives each file the MRI stdout/exit-code comparison but skips rbs and lint, so each file costs one `go build` (the prelude is pruned to what the file reaches, decision 49; built with `-race -gcflags=-l`). File count, not size, drives suite time: add checks to an existing `<area>_bugs.rb`/`<area>_mid.rb` (renaming top-level defs, constants and locals that collide, decision 14) rather than a new file. Files that exit non-zero, call `exit`, or need a file-wide magic comment stay standalone.
- `testdata/errors/<area>.txtar`: each `-- name.rb --` is compiled as `main.rb`. `# error: text` lines must all appear in the compile error. `# warning: text` lines must each match a warning. A case with no `# error:` must compile.
- `# skip: reason` in either marks a known failure, which is skipped unless `RB2GO_RUN_SKIPPED=1`. When you fix the bug, remove the line.

## Architecture

- `rb2go.go`: public `Compile`. It embeds `prelude.rb` + `prelude/*.rb` + `prelude/go/*.go` via `//go:embed`. New `.rb` prelude files must be `require_relative`d from `prelude.rb`; `prelude/go/*.go` files are globbed.
- `internal/compiler`: one `Compiler` holds the closed world (prelude + one user file). Parsing uses Prism via `go-ruby-prism` (WASM/wazero). Pipeline in `compiler.go`:
  `loadPrelude` → `collect` (declare classes/methods/consts, `model.go`) → `link` (resolve supers, includes, signatures) → `discoverIvars` (dry-run bodies for ivar types) → `emitProgram` (`decls.go`) → std-import table + gofmt (`format.go`; no go command needed).
  - `gen.go`: per-function statement codegen (`fctx`). A "tail" (none/return/assign) threads the value-producing position through statements. Type inference works by running gen inside `probe` (output discarded, types recorded) and then running it again for real.
  - `dynamic.go`: generated `rbDynName` dispatchers + `DynName(...any) any` wrappers for calls on `untyped` values. There is no `reflect`: everything comes from the closed world.
  - `naming.go`: the operator → Go name table. It must stay injective against camel-cased names (decision 3).
  - Errors are `panic(compileError)` via `errorf`/`unsupported`, recovered in `compile`. Messages carry `file:line`. Warnings (`warn`) are deduped and printed by the CLI/test.
  - When the generated Go doesn't parse, the raw output is written to `$TMPDIR/rb2go-bad-output.go`.

### The prelude

`prelude/*.rb` is the core library, written in Ruby and compiled by the same transpiler. It reopens core classes, so it never runs on MRI.
- `%x{ ... }` is the Go escape hatch. `self` and parameter names bind to the Go receiver and args. The body is a Ruby xstring, so write `\\n` for a Go `\n` and `\#{` for a literal `#{`, and keep braces balanced (decision 16). A one-line body of a non-void method gets an implicit `return`. Imports are added by goimports.
- Methods with a `%x{}` body must carry a `#:` signature (the body is opaque). Pure-Ruby methods can infer.
- `# @go_type T` makes a class a named Go type (value semantics if frozen, e.g. `String`). Classes without it become structs, with an interface (`FooI`) and pointers.
- Package-level Go helpers (no `self`/param binding) live in `prelude/go/<name>.go`, next to the `prelude/<name>.rb` whose methods use them: real, gofmt-clean Go, no xstring escaping. `//go:build ignore` keeps them out of `go build ./...` (they reference generated types); `loadPreludeGo` in `compiler.go` globs them and appends each body (minus its `package` line) to `c.verbatim`, skipping Prism. Imports come from the `stdImports` table in `format.go`; a new std package must be added there. A top-level `%x{}` in a `.rb` still works, but new helpers go in `prelude/go/`. `runtime.rb` and `dynamic.rb` are now doc-only. `.golangci.generated.yml` has path-scoped exclusions for `prelude/go/*.go`: staticcheck's ST1003/ST1021 (names mirror Ruby, like decision 3) and nolintlint (gosec ignores `//line` remapping, so it can't correlate `//nolint:gosec` comments there even though the suppression itself works).

### Key representation rules (details in README decisions)

- `T?` → `*T` uniformly. `Opt()`/`Ref()` at `untyped`/optional boundaries.
- Methods on struct classes/modules → free funcs `Owner_Name[Self]` + forwarding methods on each concrete class (Go embedding would bind `self` wrongly). Module constraints are derived from what the module body calls on `self`.
- Generic methods (RBS `[X]`) are always free funcs.
- Blocks: a method is a Go iterator (`iter.Seq`) only when block and method both return void and the method only yields. Otherwise the block is a closure, and `return`/`break` inside it is a compile error.
- `raise` → `panic`, `rescue` → `defer`/`recover`. Uncaught exceptions print `msg (Class)` and exit 1, like MRI.
- `Hash` is insertion-ordered. `Hash#[]` returns `V?`. There is no default-value hash.

## Repo conventions

- Lint: `.golangci.yml` covers the repo (depguard, cyclop, wrapcheck, …). `.golangci.generated.yml` is the same set minus `unused`/`unparam`/`revive` naming, and it applies to transpiler output. Fix codegen instead of loosening it.
- Commit after each green milestone (e.g. a new example passing), in the style `rb2go: <feature> (example NN)`.
