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

`-race` and `-gcflags` pass through to `go build`. `--int-overflow=MODE`
chooses what an Integer past 64 bits does: `raise` (`RangeError`, the
default), `wrap` (two's-complement wraparound, for hashes and checksums) or
`promote` (a Bignum, as MRI); a `# int_overflow: promote` line in the file
does the same ([decision 372](docs/decisions/372.md)). Compile errors and warnings
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
- **Integers are 64-bit by default.** Where MRI would grow an Integer past
  64 bits, rb2go raises `RangeError` rather than silently wrapping.
  `--int-overflow=promote` makes a local that grows in a loop, a `**` result
  and a literal past 64 bits a Bignum (example 126); overflow through a typed
  parameter, ivar or container element still raises
  ([decision 372](docs/decisions/372.md)).
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
per-operation overhead is gone. Over the 18 workloads in
[`benchmarks/`](benchmarks/) (Hash, Array, string, regexp, JSON, exceptions,
allocation, Bignum, and three unmodified gems), rb2go runs in 1.87s where MRI
4.0.7 takes 5.58s: 3.0x MRI, against 1.2x for YJIT, 1.4x for spinel and Crystal and
5.3x for hand-written Go. [BENCHMARKS.md](BENCHMARKS.md) has the full table,
how each column is built, where rb2go trails the native ports and why, and
what the Integer overflow checks cost (nothing measurable).

Correctness is measured against MRI, not a specification: 127
[examples](examples/), 44 minitest suites and 25 compile-error archives all
run against `ruby` under `go test`. As a wider probe, `cmd/rubyspec` runs
ruby/spec under rb2go (decision 127). Measured 2026-10-25 in the default mode, unchanged by decisions 371-372 (the runner walks
nested spec folders since 2026-10-21, decision 336, so the totals cover 2,151
`core` and 1,517 `library` spec files, not the 1,855 and 715 it read before):

| ruby/spec | examples | pass | fail | error | skipped |
|---|---:|---:|---:|---:|---:|
| `core` | 21,709 | **8,916** | 1,307 | 2,478 | 9,008 |
| `library` | 5,304 | **1,116** | 192 | 1,536 | 2,460 |
| total | 27,013 | **10,032** | 1,499 | 4,014 | 11,468 |

Under `--int-overflow=promote` ([decision 372](docs/decisions/372.md)), where
the runner also defines mspec's `bignum_value`, `core` passes **9,077** (+161:
`core/integer` 180 to 269, `core/array/pack` 1,136 to 1,185) and `library`
**1,120** (+4). Most Bignum specs still skip or raise there: they pass a
Bignum through a typed parameter or call a method Bignum does not define.

"Fail" means an assertion did not hold. "Error" means the example raised
instead. "Skipped" means the example uses something rb2go can't compile; the
largest groups are Bignum, String mutation, String encoding tags and
`eval`/code loading, which rb2go leaves out by design. Of the examples that
run, 70.2% of `core` and 39.2% of `library` pass: down from 82.1% and 81.7% on 2026-10-23 although passes rose (8,582 to 8,916 and 1,110 to 1,116), because decisions 362-370 compile examples rb2go used to skip, and many of those now run and raise (a constant a skipped fixture would have defined, `library/matrix`'s 289 without Matrix). `core/array/pack` and
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

Side by side with spinel ([#114](https://github.com/jtarchie/ruby2go/issues/114)),
on spinel's own ruby/spec harness (`tools/rubyspec`: its 22 enrolled suites
at its ruby/spec pin `79e2dee`, each `it` block one standalone program,
counting only the 7,838 programs MRI 4.0.7 passes), run for both compilers
with [`scripts/spinel-rubyspec`](scripts/spinel-rubyspec/compare) on 2026-10-25:

| suites | examples | spinel | rb2go |
|---|---:|---:|---:|
| `language` | 1,903 | 1,408 | 1,028 |
| 21 `core` classes | 5,935 | 4,333 | 2,954 |
| total | 7,838 | **5,741** (73%) | **3,982** (51%) |

rb2go passes 313 of them that spinel does not, and spinel 2,072 that rb2go
does not (from 3,106 before decisions 362-370). The largest remaining causes
are Bignum ([#113](https://github.com/jtarchie/ruby2go/issues/113)),
`define_method`, specs that pass a wrong type expecting a run-time
`TypeError` (decision 20) and in-place String mutation (decision 58).

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
