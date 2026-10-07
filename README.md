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
supported standard library. Gems and run-time `$LOAD_PATH` changes are not
supported.

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
  and `minitest` (including `Minitest::Spec`).

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
  encoding is a compile error.
- **Integers are 64-bit.** There is no Bignum; where MRI would promote,
  rb2go raises `RangeError` rather than silently wrapping.
- **No hash defaults.** `Hash.new(0)` is rejected; use `tally`,
  `h[k] = (h[k] || 0) + 1`, or `fetch`. `Hash#[]` returns `V?`.
- **`return`/`break` inside a closure block** is a compile error unless the
  block can be inlined as a loop (e.g. `each` on an `Array`).
- **Overloaded RBS signatures** (`(Integer) -> T | () -> T`) are not
  supported; prelude methods pick one shape.
- **One source file**, no gems.

Every trade-off above has a numbered entry, with its reasoning, in the
[design record](docs/design.md).

## Performance and conformance

A compiled program does no interpretation, so the interpreter's
per-operation overhead is gone. On one Apple-silicon machine (MRI 4.0.7;
`scripts/benchmark`, median of 7 runs over [`benchmarks/`](benchmarks/)):

| workload | MRI | rb2go | vs MRI |
|---|---:|---:|---:|
| `fib(32)` | 0.29s | 0.02s | 17x faster |
| `Hash` counting, 500k | 0.13s | 0.02s | 7x faster |
| `Array#map`/`select`/`sum`, 1M | 0.17s | 0.03s | 6x faster |
| string build/split, 100k | 0.13s | 0.03s | 5x faster |
| `Math.sin`/`cos`, 200k | 0.11s | 0.01s | 10.8x faster |

`Math` calls Go's `math` package directly, so a printed transcendental can
differ from MRI in the last digit — Go and MRI's libm are both within one
ulp and neither is uniquely correct ([decision 43](docs/design.md)).
Ahead-of-time compilation is the other trade: `rb2go build` is ~0.25s warm
on hello-world, but the resulting binary starts in under 10ms (MRI ~70ms).

Correctness is measured against MRI, not a specification: 107
[examples](examples/), 29 minitest suites and 22 compile-error cases all
run against `ruby` under `go test`. As a wider probe, the prelude defines
a method named by **73.6%** of ruby/spec's 1,855 `core/*` method spec
files (`scripts/rubyspec-coverage`, a name-level heuristic;
[#49](https://github.com/jtarchie/ruby2go/issues/49)).

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
bundle install          # rbs, rbs-inline, webrick
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
