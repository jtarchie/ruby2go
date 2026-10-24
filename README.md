# rb2go

Compile typed Ruby to a native Go binary.

rb2go reads a Ruby program whose methods carry
[rbs-inline](https://github.com/soutaro/rbs-inline) type annotations
(`#: (String) -> Integer` comments), translates it to a single Go file, and
builds it with the Go toolchain. The result is a static executable with no
Ruby interpreter and no runtime library: Ruby's core classes are compiled in
from a prelude that is itself written in Ruby.

The source stays plain Ruby. Every program rb2go accepts also runs on MRI
(the standard `ruby`) unchanged, and the test suite holds rb2go to that: for
each example, the compiled binary's stdout and exit code must match
`ruby main.rb` byte for byte.

> **Status:** experimental. It compiles a large, useful subset of Ruby and a
> good slice of the standard library (see [What works](#what-works)), but it
> is not a drop-in replacement for `ruby`. Expect to hit gaps.

## Example

```ruby
# rbs_inline: enabled

class Greeter
  #: (String) -> void
  def initialize(name)
    @name = name #: String
  end

  #: (Integer) -> Array[String]
  def greet(times)
    (1..times).map { |i| "#{i}. hello, #{@name}" }
  end
end

Greeter.new(ARGV.first || "world").greet(3).each { puts it }
counts = %w[a b a c a b].tally
p counts.max_by { |_, n| n }
```

```console
$ rb2go run greet.rb Go
1. hello, Go
2. hello, Go
3. hello, Go
["a", 3]

$ ruby greet.rb Go        # same output
```

The class becomes a Go struct, and its methods become ordinary typed Go:

```go
type Greeter struct {
	name String
}

func Greeter_Greet[Self GreeterI](self Self, times Integer) *Array[String] {
	return Enumerable_Map[*Range[Integer], Integer, String]((&Range[Integer]{b: 1, e: times}), func(i Integer) String {
		return (Integer.ToS(i) + ". hello, " + self._Greeter().name)
	})
}
```

## Install

You need **Go 1.26 or newer** on your `PATH`: rb2go calls `go build` to
produce the binary. Ruby is *not* needed to compile programs (the Ruby parser,
Prism, is embedded as WebAssembly).

```sh
go install github.com/jtarchie/ruby2go/cmd/rb2go@latest
```

Or from a clone: `go install ./cmd/rb2go`.

## Usage

The CLI is shaped like `go build` / `go run` / `go test`:

```sh
rb2go run main.rb [args...]        # compile, build, run; exits with the program's status
rb2go build -o prog main.rb        # write a binary
rb2go build -work main.rb          # keep the generated main.go; prints its directory as WORK=...
rb2go gen main.rb > main.go        # print the generated Go (no go command needed)
rb2go test test/                   # compile minitest files into one binary and run them
rb2go test test/foo_test.rb --seed 1 -v   # minitest flags pass through
rb2go web                          # playground at http://127.0.0.1:8080: Ruby in, Go out, Run button
rb2go web -static dir -assets URL  # the same playground as static files, compiling in the browser (scripts/deploy-web.sh)
```

`-race` and `-gcflags` pass through to `go build`. Compile errors and warnings
point at `file:line` in your Ruby source, and runtime panics map back to Ruby
lines through `//line` directives.

A program is a Ruby file plus whatever it `require_relative`s: each file
loads once, where it is required, as in Ruby. The path must be a string
literal at a file's top level, since rb2go reads the files when it compiles.
`require "x"` finds your own files in the directories given with `-I`, as
`ruby -I lib main.rb` does (`rb2go run -I lib main.rb`), and otherwise loads a
supported standard library. Any other name is looked up as an installed gem
(through Bundler when a `Gemfile` sits beside the program), which `build`,
`run` and `gen` then compile in from source; that needs `ruby` on PATH at
build time, and only unmodified rack and Cuba are tested. Run-time
`$LOAD_PATH` changes are not supported.

Compiling a program that loads a gem *runs* it: `build`, `run` and `gen` execute
the program's load phase under `ruby` (and the `Gemfile` beside it, through
Bundler) with your full environment, as `ruby main.rb` would. There is no
sandbox, so don't point rb2go at source you would not run. The run is bounded
(10 minutes, capped output). Programs without gems never start `ruby`.

## Writing Ruby for rb2go

Put `# rbs_inline: enabled` at the top and annotate method signatures with
`#:` comments. Locals, block parameters and most return types are inferred;
annotate where inference has nothing to go on:

```ruby
#: (String, ?Integer) -> Array[String]     # method signature (? = optional arg)
def split_words(text, limit = 10) = text.split.first(limit)

@cache = {} #: Hash[String, Integer]       # an empty literal needs its type (in initialize)
attr_reader :name #: String                # attribute type
name = nil #: String?                      # T? means "T or nil"
```

Sorbet-typed code works too: with `require "sorbet-runtime"`, `sig { params(...).returns(...) }`,
`T.let`, `T.cast` and `T.must` are read as the same types, and what sorbet-runtime would raise
while running is a compile error instead (decision 196, [example 119](examples/119_sorbet_sigs/main.rb)).

How Ruby types land in Go:

| Ruby | Go |
|---|---|
| `Integer`, `Float`, `String`, `Symbol` | named Go types over `int`, `float64`, `string` (values, with methods) |
| `true` / `false` | one `Boolean` type |
| `T?` (maybe `nil`) | `*T`; `if x` narrows it |
| `A \| B` (one of a few classes) | Go `any`; each call is a type switch with a typed call per member, and `is_a?`/`case` narrow it |
| `Array[T]`, `Hash[K, V]` | generic Go types; `Hash` keeps insertion order like Ruby |
| a class | a struct, plus an interface so subclasses dispatch correctly |
| a module | generic free functions constrained by what the module calls on `self` |
| `raise` / `rescue` / `ensure` | `panic` / `recover` / `defer` |
| `Thread`, `Queue`, `Mutex` | goroutines, channels, `sync` |
| `Ractor`, `Ractor::Port` | a goroutine and a queue; messages are deep-copied, isolation is checked at compile time |
| `untyped` | Go `any`, with calls dispatched through generated switches (the compiler warns at each one) |

`untyped` is the escape hatch for code the type system can't follow: `send`,
`method_missing`, `const_get` and `respond_to?` work on it, generated from the
whole program at compile time rather than looked up by reflection.

## What works

- **Language:** classes, modules, `include`/`extend`, inheritance and `super`,
  blocks, procs and lambdas, `Method` objects (`&method(:name)`), iterators,
  `case`/`when`, exceptions with custom hierarchies, `catch`/`throw`, `Struct`
  and `Data`, optional and splat arguments, multiple assignment, `||=`, Ruby 4.0 syntax (`it`, leading `&&`),
  `at_exit`, signal traps.
- **Core classes:** `String`, `Symbol`, `Numeric` (`Integer`, `Float`, `Rational`,
  `Complex`, mixing as MRI coerces), `Array`, `Hash`, `Range`, `Set`, `Enumerable`, `Enumerator`
  (with `next`/`peek`, `Enumerator.new` generators and `Enumerator::Lazy`),
  `Comparable`, `Regexp`/`MatchData` (Ruby syntax on Go's RE2), `Time`,
  `Random` (same sequence as MRI for a given seed), `Math`, `File`, `Dir`,
  `IO`, `ARGV`, `ENV`, `ARGF`, `$stdin`, `Kernel#system` and backticks,
  `Thread`, `Queue`, `Mutex`, `Ractor`, `Fiber`, `Marshal` (its own bytes, not
  MRI's).
- **Standard library:** `json`, `set`, `time`, `date`, `csv`, `stringio`,
  `strscan`, `digest`, `base64`, `zlib`, `securerandom`, `shellwords`, `uri`,
  `net/http`, `open-uri`, `webrick`, `socket`, `fileutils`, `find`, `pathname`,
  `tempfile`, `open3`, `logger`, `ipaddr`, `etc`, `timeout`, `tsort`, `abbrev`,
  `observer`, `forwardable`, `singleton`, `optparse`, `benchmark`, `cgi`,
  `fiddle` (calls C libraries without cgo, through purego;
  [example 121](examples/121_fiddle/main.rb)),
  `sqlite3` (the sqlite3 gem's API over the system's libsqlite3 through
  Fiddle, from the vendored pure-Ruby gem
  [gems/sqlite3-fiddle](gems/sqlite3-fiddle), which also runs on MRI;
  [example 122](examples/122_sqlite3/main.rb)),
  and `minitest` (including `Minitest::Spec`); from gems, the unmodified
  `rackup` gem's WEBrick handler serves a Rack app, and the unmodified `cuba`
  gem on `rack` serves routes with captures, query parameters, POST forms and
  404s
  ([example 116](examples/116_cuba_web/main.rb)), and the unmodified `kramdown`
  gem turns Markdown (lists, tables, footnotes, code blocks, inline HTML) into
  HTML, compiled with `rb2go build -trace` because it requires its converters
  by computed name ([example 123](examples/123_kramdown/main.rb)), and the
  unmodified `liquid` gem (Jekyll's template language: loops, conditionals,
  filters, whitespace control) renders templates
  ([example 124](examples/124_liquid/main.rb)).

[`examples/`](examples/) has one program per feature, from
[inheritance](examples/01_inheritance/main.rb) to a
[Rack-style web framework](examples/32_resty_reflective/main.rb) served over
`net/http`. Each is a normal Ruby script you can run with `ruby`.

The roadmap for the rest of the standard library is issue
[#1](https://github.com/jtarchie/ruby2go/issues/1); CLI work is
[#2](https://github.com/jtarchie/ruby2go/issues/2).

## What doesn't (by design, or not yet)

- **No `eval`, `define_method`, `instance_variable_get`, or runtime
  reopening.** rb2go compiles a *closed world*: every class and method must be
  visible at compile time.
- **Strings are frozen.** `String` is a Go `string` value, so in-place
  mutation (`<<`, `upcase!`, `+"..."`) is unsupported; build new strings
  instead, or use `StringIO`.
- **Strings carry no encoding.** Every `String` is UTF-8 bytes; `encoding`
  reports UTF-8 for valid UTF-8 and ASCII-8BIT otherwise. `encode`, `scrub`,
  `unicode_normalize` and `File.open(path, "r:ISO-8859-1:UTF-8")` work for
  UTF-8, ASCII-8BIT, US-ASCII, ISO-8859-1 and UTF-16/32; naming any other
  encoding is a compile error. A String the compiler can see is binary
  (`.b`, `File.binread`, `force_encoding("BINARY")`, `read(n)`) counts,
  slices and matches bytes even when they are valid UTF-8; passed through a
  method parameter, ivar or collection it reads as UTF-8 again
  ([decision 352](docs/decisions/352.md)).
- **Integers are 64-bit.** There is no Bignum; where MRI would promote,
  rb2go raises `RangeError` rather than silently wrapping.
- **No hash defaults.** `Hash.new(0)` is rejected; use `tally`,
  `h[k] = (h[k] || 0) + 1`, or `fetch`. `Hash#[]` returns `V?`.
- **`return`/`break` inside a closure block** is a compile error unless the
  block can be inlined as a loop (e.g. `each` on an `Array`).
- **Overloaded RBS signatures** (`(Integer) -> T | () -> T`) are not
  supported; prelude methods pick one shape.
- **Gems** compile from source with `ruby` on PATH at build time; rack and
  Cuba are the ones tested.

Every trade-off above has a numbered entry, with its reasoning, in the
[design record](docs/design.md).

## Performance and conformance

A compiled program does no interpretation, so the interpreter's
per-operation overhead is gone. On one Apple-silicon machine (MRI 4.0.7,
Crystal 1.22.1, Go 1.27.3, spinel `baea3e28`; `RUNS=9 MARKDOWN=1 scripts/benchmark`,
median of 9 runs over [`benchmarks/`](benchmarks/)):

| workload | MRI | MRI+YJIT | rb2go | spinel | Crystal | Go |
|---|---:|---:|---:|---:|---:|---:|
| `fib(32)` | 0.27s | 0.095s | 0.019s | 0.016s | 0.018s | 0.017s |
| `Hash` counting, 500k | 0.11s | 0.10s | 0.018s | 0.009s | 0.014s | 0.011s |
| `Array#map`/`select`/`sum`, 1M | 0.15s | 0.14s | 0.023s | 0.013s | 0.014s | 0.013s |
| string build/split, 100k | 0.11s | 0.11s | 0.031s | 0.018s | 0.025s | 0.017s |
| `Math.sin`/`cos`, 200k | 0.090s | 0.077s | 0.012s | 0.008s | 0.009s | 0.009s |
| classes, polymorphic calls, Comparable | 0.27s | 0.17s | 0.037s | - | 0.015s | 0.014s |
| binary trees (allocation), depth 10 | 0.083s | 0.070s | 0.012s | 0.010s | 0.012s | 0.009s |
| splay tree | 0.22s | 0.12s | 0.014s | 0.019s | 0.018s | 0.012s |
| blocks, procs, lambdas, enumerator chains | 0.35s | 0.34s | 0.065s | 0.14s | 0.038s | 0.013s |
| raise/rescue/retry, catch/throw | 0.28s | 0.25s | 0.12s | 0.078s | 2.52s | 0.020s |
| regexp match/gsub/scan/tr, StringScanner | 0.76s | 0.73s | 0.21s | 0.84s | 0.21s | 0.28s |
| sort_by/group_by/tally, Set, Struct | 0.54s | 0.53s | 0.12s | 1.12s | 0.054s | 0.068s |
| format, StringIO, case mapping | 0.50s | 0.47s | 0.24s | 0.21s | 0.14s | 0.11s |
| JSON generate + parse | 0.14s | 0.16s | 0.12s | 0.24s | 0.13s | 0.25s |
| pairing_heap gem: Dijkstra + heapsort | 0.67s | 0.34s | 0.21s | - | 0.039s | 0.035s |
| liquid gem: a template rendered 400 times | 0.39s | 0.31s | 0.17s | - | - | - |
| kramdown gem: a 20-section document to HTML, 30 times | 0.44s | 0.56s | 0.39s | - | - | - |
| total (rows each one ran) | 5.37s | 4.56s | 1.81s | 2.73s | 3.26s | 0.87s |
| vs MRI on those rows | 1.0x | 1.2x | 3.0x | 1.3x | 1.4x | 5.2x |

What the columns are:
- **rb2go** and **spinel** ([matz/spinel](https://github.com/matz/spinel), a
  Ruby-to-C compiler) compile the same unmodified `.rb` file. A "-" means it
  did not compile it or printed something other than MRI: spinel cannot load
  the gems or one `objects.rb` construct.
- **Crystal** and **Go** are hand-written ports in
  [`benchmarks/crystal`](benchmarks/crystal) and [`benchmarks/go`](benchmarks/go):
  the same algorithm and sizes, written as a programmer in that language would,
  stdlib only, printing exactly what MRI prints. The deviations are noted at the
  top of each file: for example, Go uses error values instead of exceptions and a
  value struct for `objects`' vectors. kramdown and liquid have no port, since a
  port would be a different markdown or template library.
- Each "vs MRI" compares a column only on the rows it ran. That is why spinel's
  ratio is low: its `collections` row (2x slower than MRI) weighs on a smaller
  total.

Where rb2go trails the native columns, the cost is mostly in its Ruby
semantics: a frozen check on every mutation, a Hash
that keeps insertion order, and `T?` still boxed for a class with subclasses or type parameters (decisions 359-360 made
other leaf classes plain pointers with a plain `nil`, which put splay ahead
of spinel and Crystal). It is ahead of spinel on
regexp, collections, blocks and JSON, where spinel's runtime library is
thinner. Against Crystal it wins on exceptions, which Crystal unwinds with a
backtrace per raise.

The closest rows to MRI are JSON and kramdown. MRI's `json` is a C extension; rb2go's
generator writes into one buffer instead of a String per nested value
(decision 346), and its parser builds Hashes on an entry array (decision 350). kramdown spends its regex time partly on
patterns RE2 cannot match (lookbehinds, atomic groups, backreferences), which
run on the regexp2 backtracker (decisions 242, 332, 347).

`Math` calls Go's `math` package directly, so a printed transcendental can
differ from MRI in the last digit — Go and MRI's libm are both within one
ulp and neither is uniquely correct ([decision 43](docs/decisions/043.md)).
Ahead-of-time compilation is the other trade: `rb2go build` is ~0.3s warm
on hello-world, but the resulting binary starts in under 10ms (MRI ~60ms).

Correctness is measured against MRI, not a specification: 126
[examples](examples/), 43 minitest suites and 25 compile-error archives all
run against `ruby` under `go test`. As a wider probe, `cmd/rubyspec` runs
ruby/spec under rb2go (decision 127). Measured 2026-10-23 (the runner walks
nested spec folders since 2026-10-21, decision 336, so the totals cover 2,151
`core` and 1,517 `library` spec files, not the 1,855 and 715 it read before):

| ruby/spec | examples | pass | fail | error | skipped |
|---|---:|---:|---:|---:|---:|
| `core` | 21,731 | **8,561** | 1,094 | 772 | 11,304 |
| `library` | 5,307 | **1,102** | 152 | 95 | 3,958 |
| total | 27,038 | **9,663** | 1,246 | 867 | 15,262 |

"Fail" means an assertion did not hold. "Error" means the example raised
instead. "Skipped" means the example uses something rb2go can't compile; the
largest groups are Bignum, String mutation, String encoding tags and
`eval`/code loading, which rb2go leaves out by design. Of the examples that
run, 82.1% of `core` and 81.7% of `library` pass. `core/array/pack` and
`core/string/unpack` (3,177 examples, 2,543 passing) read as 11% passing in
the first run that reached them because the runner did not set mspec's
`@method` for the specs' format helpers; it does now
([#110](https://github.com/jtarchie/ruby2go/issues/110)). What is left there is
mostly by design: no Bignum, no encoding tags, no C pointers (`P`, `p`), and
mock objects that answer `to_int`/`to_str`. Not counted: `library/etc` and
`library/erb/defmethod`, which stop on a syntax error in the program the
runner builds, and `language`, which stops on an internal compiler
error. A looser name-level metric,
[`scripts/rubyspec-coverage`](scripts/rubyspec-coverage), finds 81.2% of
core's method spec files name a method the prelude or the compiler defines
([#49](https://github.com/jtarchie/ruby2go/issues/49)).

## How it works

1. Parse the prelude and the program with Prism (the official Ruby parser,
   run as WebAssembly via [wazero](https://wazero.io)).
2. Collect every class, module, method and constant into one closed world;
   resolve superclasses, mixins and RBS signatures.
3. Infer local, instance-variable and return types by dry-running code
   generation.
4. Emit Go: generics for containers and mixins, interfaces for virtual
   dispatch, iterators (`iter.Seq`) for yielding methods.
5. Prune everything the program can't reach, `gofmt`, and hand the file to
   `go build`.

The core library is [`prelude/`](prelude/): Ruby source that reopens core
classes, with leaf primitives dropping into Go through `%x{ ... }` bodies.
Generated programs use only the Go standard library and never `reflect`.

Background: [docs/design.md](docs/design.md) (the design record and every
decision), [docs/dynamic-dispatch.md](docs/dynamic-dispatch.md) (how
`untyped` calls work without a runtime).

## Development

Running the test suite needs Ruby ≥ 4.0 (to produce the expected output),
Bundler and [golangci-lint](https://golangci-lint.run) on `PATH`:

```sh
bundle install          # rbs, rbs-inline, webrick, rackup
go test ./...           # every example and test file, compared against MRI
go test -run 'TestExamples/each/05_word_count' .    # one example
```

There are no golden files: MRI's output is the only expectation. Each
`examples/*/main.rb` must pass `rbs validate`, the generated Go must pass
`gofmt`, `go vet` and `golangci-lint`, and the binary's stdout and exit code
must equal `ruby main.rb`. Behaviour checks live in `testdata/test/*_test.rb`
as minitest files that must pass on both MRI and rb2go. A cold run can take
more than ten minutes; pass `-timeout 30m`.

```
cmd/rb2go/          the CLI
internal/compiler/  Ruby → Go: declarations, type inference, codegen
internal/rbs/       the RBS type-syntax subset the compiler understands
prelude.rb, prelude/  the core and standard library, in Ruby (+ Go helpers in prelude/go/)
examples/           one program per feature, each checked against MRI
testdata/           minitest suites, print-and-compare snippets, compile-error cases
docs/               design record and deep dives
```

[CLAUDE.md](CLAUDE.md) has the full command list and contributor conventions.

## Prior art

[Opal](https://opalrb.com) (Ruby → JavaScript, core library in Ruby with
`%x{}` escapes), [Crystal](https://crystal-lang.org) (Ruby-like syntax,
static types, same 64-bit Integer trade-off), and TruffleRuby/Rubinius (core
written in Ruby over primitives).

## License

[MIT](LICENSE). The minitest port in `prelude/minitest.rb` keeps minitest's
own MIT license ([prelude/minitest.LICENSE](prelude/minitest.LICENSE)).
