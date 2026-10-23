# CLAUDE.md

rb2go: transpiles a typed subset of Ruby (rbs-inline `#:` annotations) to a single Go file. README.md is the user-facing intro; docs/design.md is the design record. Its numbered "Open decisions" list is binding: read the relevant entry before changing behavior, and add or amend an entry when a new decision is made. New decisions are appended at the END of design.md with the next number (last: 346); when one amends an older entry, also add a "see also" pointer inside the old entry. The index table near the top of design.md is generated: `scripts/design-index` rewrites the block between its `design-index` markers (run it after adding or retitling a decision).

## Commands

```sh
git submodule update --init         # gems/*: vendored gem sources; the build embeds their mirror in vendored/ (scripts/vendor-gems; decision 328)
bundle install                      # rbs, rbs-inline, webrick (needed by tests)
go test ./...                       # the oracle; see below. ~7 min with warm caches (2026-10-22); ~23 min the first run after a prelude/go edit, which misses the Go build cache for most programs
go test -run 'TestExamples/each/05_word_count' .   # one example
go test -run 'TestRun/^string_' .             # behaviour snippets by prefix
go test -run 'TestErrors/^regexp$' .          # one compile-error archive
go test -run 'TestMinitest/^string$' .        # one minitest file (testdata/test/string_test.rb), vs MRI
go run ./cmd/rb2go test testdata/test --seed 1  # the whole suite as one program, as `minitest testdata/test` (slow to build: decision 81)
go run ./cmd/rb2go test testdata/test/array_test.rb --seed 1 -v   # one test file (any minitest flags); a dir runs them all as one program
RB2GO_RUN_SKIPPED=1 go test -run TestRun .    # also run `# skip:` known failures
RB2GO_NO_MRI_CACHE=1 go test ./...            # rerun MRI instead of its cached output (`os.UserCacheDir()/rb2go-test/mri`; key: Ruby description, TZ, checkout dir, sha256 of Gemfile.lock, extra args, sources; decision 272)
RB2GO_NO_PRUNE=1 go run ./cmd/rb2go build -work ...  # emit the whole prelude (debugging the pruner)
RB2GO_INFER_DEBUG=1 go run ./cmd/rb2go run x.rb    # what each parameter-inference round typed and could not compile (decision 146)
RB2GO_PRUNE_DEBUG=1 go test -run TestBootAutoload .  # gem programs: why each method name was reached, live gem classes (decision 169)
RB2GO_DISPATCH_DEBUG=1 go run ./cmd/rb2go build x.rb   # why a Kernel/Object name keeps a Go method instead of a type-switch dispatcher (decisions 212-213); =all for every owner
RB2GO_GEM_WARNINGS=1 go run ./cmd/rb2go gen ...       # also print gem files' warnings (skipped statements, nil constants: decision 205)
RB2GO_GEM_WARNINGS=1 RB2GO_UNTYPED_WHY=1 go run ./cmd/rb2go gen ...  # each dynamic-call warning names the root of its untyped receiver (sig untyped, widened local, unannotated return, ...)
RB2GO_BACKTRACE=1 ./prog                       # a built program reports an uncaught exception as MRI does, file:line and from lines (decision 178)
RB2GO_GO_STACK=1 ./prog                        # a Go runtime panic turned into a Ruby exception carries its Go stack (decision 324)
RB2GO_COST=1 go test -run TestCompileCost -v .    # compiled functions + compile time/memory of 05_word_count, 64_observable, 116_cuba_web (#96); or a comma list of examples
RB2GO_CPUPROFILE=gen.prof go run ./cmd/rb2go gen x.rb > /dev/null  # CPU profile of the compile itself (go tool pprof -top gen.prof)
go -C scripts/census run . /path/main.go           # what a generated main.go spends its functions on (forwarders, Dyn wrappers, stubs); ./inst walks generic instantiations
go test -run '^$' -fuzz FuzzCompile -fuzztime 60s .              # fuzz targets: FuzzCompile (.),
go test -run '^$' -fuzz FuzzParseType -fuzztime 30s ./internal/rbs # FuzzParseType/FuzzParseMethodType (rbs),
go test -run '^$' -fuzz FuzzTranslateRegexp ./internal/compiler    # FuzzGoMethodName/FuzzTranslateRegexp (compiler)
go run ./cmd/rb2go stubs && go vet -tags rb2go_prelude ./prelude/go   # type-check prelude/go helpers against generated stubs (decision 154; TestPreludeGo does the same)
go test ./internal/rbs              # RBS parser unit tests
golangci-lint run ./...             # repo lint (.golangci.yml)
go run ./cmd/rb2go run examples/NN_x/main.rb        # transpile, build, run; warnings go to stderr
go run ./cmd/rb2go build -work -gcflags=-e examples/NN_x/main.rb  # keep main.go (WORK= path on stderr), show every Go error
scripts/rubyspec-coverage --missing   # ruby/spec core method files the prelude names, per class (#49; clones ruby/spec to the cache)
go run ./cmd/rubyspec -reasons 20 core/integer   # run ruby/spec directories under rb2go (decision 127); `core` runs each core/* dir, -v lists every skip; the pinned checkout lives in `os.UserCacheDir()/rb2go-rubyspec`
bundle exec tyda path/to/file.rb        # inferred RBS for unannotated Ruby (dev aid, see below); --diagnostics for JSON Lines
```

`go test` fails fast unless `ruby` ≥ 4.0, `bundle install` has been run, and `golangci-lint` is on PATH. Gem executables are not on PATH on this machine, so use `bundle exec rbs-inline ...` for manual runs. Stderr lines like rdoc "already initialized constant" warnings are noise.

## When to use tyda

`tyda` (Gemfile, development group) infers RBS from unannotated Ruby and prints it. It is not part of the build or tests: rb2go's own inference (decisions 36, 146, 183) is the authority, and the two disagree by design (tyda emits `untyped` where unsure and keeps literal types like `1 | "one"`). Reach for it when:
- annotating a legacy file or a vendored gem (decision 110/160): run it, keep the concrete signatures, widen literals, fill the `untyped` ones by hand.
- a program fails inference and you want a second opinion on a parameter or return type before adding a `#:`.
- an rb2go inference result looks wrong: if tyda agrees with MRI's behaviour and rb2go does not, that is a bug to file.
Never feed its output into the compiler automatically, and never loosen rb2go's typing to match it.

## What a passing example means

For each `examples/*/main.rb`, `TestExamples` (rb2go_test.go):
1. runs `rbs-inline` + `rbs validate`. A `require "x/y"` line adds `-r x-y`, so annotations must be valid RBS.
2. runs `Compile`, then `gofmt`, `go vet`, `golangci-lint` with `.golangci.generated.yml` (the generated Go must lint clean), and `go build`.
3. requires stdout **and** exit code to equal `ruby main.rb`.

Generated programs are built with `-trimpath`, plus `-race` when they start goroutines (decision 88) (`-trimpath` lets the build cache hit across temp dirs; the first run after a Go upgrade rebuilds the race std once). Examples are vetted and linted in one pass over a shared module after all of them pass. MRI output is cached by source, Ruby version, TZ and checkout directory (so worktrees do not share entries holding `__dir__`).

Add one new example per new feature, numbered next in sequence: a plain program that shows the feature and prints what it computes (examples are scripts, as in a Ruby project's `examples/`; behaviour checks go in `testdata/test`). User code must run on MRI unchanged. There are no golden Go files; MRI's output is the only expectation.

Smaller cases go in `testdata/`:
- `testdata/test/<area>_test.rb`: minitest files (decision 81), preferred for new behaviour checks. `TestMinitest` runs each with `--seed 1`; MRI must pass (else the test is wrong), then rb2go's output must equal MRI's apart from timings, and every assertion's operands must equal MRI's (the trace oracle, decision 105: `RB2GO_MT_TRACE` on both sides, MRI preloading `testdata/mt_trace.rb`; the failure names the test and both operands). So a message rb2go words differently by design (decision 20's `TypeError`) is checked with `begin`/`rescue` and a class check, not `assert_raises`, which logs the message. Tests must not print. One file per area (`array`, `control`, `number`, ...); all files load together into one program (decision 84), as Ruby loads a suite into one process, so each file keeps its classes and constants inside a `module <Area>Tests` namespace, prefixes its top-level helper defs (`array_ident`: a top-level def is a private method on every object), and reopens core classes only to add methods. Don't name a top-level method `describe` (minitest's Kernel#describe; decision 83). Check the suite loads in one MRI process: `ruby $(ruby -e 'print Gem.bin_path("minitest","minitest")') testdata/test --seed 1`.
- `testdata/run/<area>_*.rb` (print-and-compare; keep for what tests output itself: `puts`/`p`/`print` formatting, exit status, crashes, stdin): `TestRun` gives each file the MRI stdout/exit-code comparison but skips rbs and lint, so each file costs one `go build` (the prelude is pruned to what the file reaches, decision 49; built with `-race -gcflags=-l`, prelude `//line`s pinned to `:1` and no DWARF so the build cache only grows for what an edit reaches, decision 88). File count, not size, drives suite time: add checks to the area's `<area>_output.rb` (renaming top-level defs, constants and locals that collide, decision 14) rather than a new file. Files that exit non-zero, call `exit`, or need a file-wide magic comment stay standalone.
- `testdata/errors/<area>.txtar`: each `-- name.rb --` is compiled as `main.rb`. `# error: text` lines must all appear in the compile error. `# warning: text` lines must each match a warning. Every case needs at least one of them (a case with only `# warning:` lines must compile); what must compile cleanly belongs in `testdata/test`.
- `# args: a b`, `# env: K=V` and `# stdin: "Go-quoted\n"` (repeatable) feed both MRI and the binary; `# load_path: dir` (repeatable, relative to the file) is `-I dir` for both `ruby` and the compile; `# stderr: match` also compares stderr (only where MRI prints no backtrace).
- `# skip: reason` in either marks a known failure, which is skipped unless `RB2GO_RUN_SKIPPED=1`. When you fix the bug, remove the line.

## Architecture

- `rb2go.go`: public `Compile`. It embeds `prelude.rb` + `prelude/*.rb` + `prelude/go/*.go` via `//go:embed`. `prelude.rb` `require_relative`s the core (what MRI defines without a require); a library file is instead mapped from its require name in `internal/compiler/libs.go` and loads only when user code has a literal `require` for it (decision 155), `require_relative`-ing what MRI's own file loads. A new `.rb` prelude file goes in one of those two places; `prelude/go/*.go` files are globbed and always loaded, so a core helper may name a lib's type only inside a single-type `case` (the pruner drops it).
- `internal/compiler`: one `Compiler` holds the closed world (prelude + one user file). Parsing uses Prism via `go-ruby-prism` (WASM/wazero; its compilation cache is `os.UserCacheDir()/rb2go/wazero`, off with `RB2GO_NO_WASM_CACHE=1`). Pipeline in `compiler.go`:
  `loadPrelude` → `collect` (declare classes/methods/consts: `collect.go`, types in `model.go`) → `link` (resolve supers, includes, signatures: `link.go`) → `discoverIvars` (dry-run bodies for ivar types: `gen_ivars.go`) → `emitProgram` (`decls.go`, with the emitters in `emit_*.go`) → std-import table + gofmt (`format.go`; no go command needed).
  - `gen.go` and `gen_*.go`: per-function statement codegen (`fctx` in `gen.go`; loops, case, rescue, locals, conditions, analysis, method and main emission each in their own `gen_*.go`; expressions in `expr*.go`). A "tail" (none/return/assign) threads the value-producing position through statements. Type inference works by running gen inside `probe` (output discarded, types recorded) and then running it again for real.
  - `dynamic.go` (and `dynamic_*.go`: wrappers, blocks, send, the runtime text): generated `rbDynName` dispatchers + `DynName(...any) any` wrappers for calls on `untyped` values. There is no `reflect`: everything comes from the closed world.
  - `naming.go`: the operator → Go name table. It must stay injective against camel-cased names (decision 3).
  - Errors are `panic(compileError)` via `errorf`/`unsupported`, recovered in `compile`. Messages carry `file:line`. Warnings (`warn`) are deduped and printed by the CLI/test.
  - When the generated Go doesn't parse, the raw output is written to `$TMPDIR/rb2go-bad-output.go`.

- `prelude/`: the core library in Ruby; its conventions (`%x{}`, `@go_type`, `prelude/go/` helpers) are in `prelude/CLAUDE.md`.
- `gems/<name>/`: vendored pure-Ruby gems as git submodules, mirrored into `vendored/<name>/` by `scripts/vendor-gems` (committed; `TestVendoredMirror` fails on drift) so the embed works from the module proxy; they run on MRI unchanged and are embedded and compiled as user code when required (`vendoredLibs` in `libs.go`, decision 328); `TestVendoredGemsMRI` runs each one's `test/*_test.rb` under MRI with its lib on the load path. sqlite3-fiddle (github.com/jtarchie/sqlite3-fiddle) is the first; its CI also compiles its test with rb2go.


### Key representation rules (details in docs/design.md decisions)

- `T?` → `*T` uniformly. `Opt()`/`Ref()` at `untyped`/optional boundaries.
- Methods on struct classes/modules → free funcs `Owner_Name[Self]` + forwarding methods on each concrete class (Go embedding would bind `self` wrongly); a class body whose signature names no `Self` takes `self OwnerI` instead (decision 210). Module constraints are derived from what the module body calls on `self`.
- Generic methods (RBS `[X]`) are always free funcs.
- Blocks: a method is a Go iterator (`iter.Seq`) only when block and method both return void and the method only yields. Otherwise the block is a closure; a valueless `break` out of a call statement unwinds with a sentinel panic (`rbBreak`, so `ensure` runs; decision 241), and a gem method's block `return` does the same through a per-call `rbReturn` token (decision 247). In user code a non-local `return` from a closure block is still a compile error (decision 4), as is `break v` or a `break` whose call value is used.
- `raise` → `panic`, `rescue` → `defer`/`recover`. Uncaught exceptions print `msg (Class)` and exit 1, like MRI.
- `Hash` is insertion-ordered. `Hash#[]` returns `V?`. `Hash.new(default)` and `Hash.new { |h, k| ... }` are supported (decision 5), but `[]` stays `V?` so a typed caller still nil-checks or uses `fetch`.

## Repo conventions

- Lint: `.golangci.yml` covers the repo (depguard, cyclop, wrapcheck, …). `.golangci.generated.yml` is the same set minus `unused`/`unparam`/`revive` naming, and it applies to transpiler output. Fix codegen instead of loosening it.
- Commit after each green milestone (e.g. a new example passing), in the style `rb2go: <feature> (example NN)`.
- GitHub issues use three sections: `## Why` (the problem, and what Ruby code fails today), `## What` (a bullet list of the behaviour to build, ending with which example or testdata file proves it against MRI), and `## Notes` (pointers to existing helpers, gotchas, and relevant docs/design.md decisions). Write for a junior PM and a junior engineer: plain words, a short explanation of any Ruby or Go term, and no assumed context. Before filing, check the roadmap issues (#1 stdlib, #2 CLI) so you don't duplicate them, and link #1 or #2 from new issues.
