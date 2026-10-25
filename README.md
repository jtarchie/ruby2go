<p align="center">
  <img src="site/logo.svg" alt="rugaroo: a rougarou pup wearing a ruby, in front of the moon" width="160">
</p>

<h1 align="center">rugaroo</h1>

<p align="center"><em>Ruby by day. Go by night.</em><br>
Compile typed Ruby to a native Go binary.<br>
<a href="https://rugaroo.dev">rugaroo.dev</a> · <a href="https://play.rugaroo.dev">playground</a></p>

A [rougarou](https://en.wikipedia.org/wiki/Rougarou) is a shape-shifter from
Cajun folklore. This one turns Ruby into Go.

rugaroo reads a Ruby program whose methods carry
[rbs-inline](https://github.com/soutaro/rbs-inline) type annotations
(`#: (String) -> Integer` comments), translates it to a single Go file, and
builds it with the Go toolchain. The result is a static executable with no
Ruby interpreter and no runtime library: Ruby's core classes are compiled in
from a prelude that is itself written in Ruby.

The source stays plain Ruby. Every program rugaroo accepts also runs on MRI
(the standard `ruby`) unchanged, and the test suite holds rugaroo to that: for
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
$ rugaroo run greet.rb Go
1. hello, Go
2. hello, Go
3. hello, Go
["a", 3]

$ ruby greet.rb Go        # same output
```

The class becomes a Go struct, and its methods become ordinary typed Go
(excerpt of `rugaroo gen`, minus `//line` directives):

```go
type Greeter struct {
	name String
}

func Greeter_Greet(self *Greeter, times Integer) *Array[String] {
	return Range_Map[Integer, String](&Range[Integer]{b: 1, e: times}, func(i Integer) String {
		return Integer.ToS(i) + ". hello, " + self._Greeter().name
	})
}
```

## Install

You need **Go 1.26 or newer** on your `PATH`: rugaroo calls `go build` to
produce the binary. Ruby is *not* needed to compile programs (the Ruby parser,
Prism, is embedded as WebAssembly).

```sh
go install github.com/jtarchie/rugaroo/cmd/rugaroo@latest
```

Or from a clone: `go install ./cmd/rugaroo`.

## Usage

The CLI is shaped like `go build` / `go run` / `go test`:

```sh
rugaroo run main.rb [args...]        # compile, build, run; exits with the program's status
rugaroo build -o prog main.rb        # write a binary
rugaroo build -work main.rb          # keep the generated main.go; prints its directory as WORK=...
rugaroo gen main.rb > main.go        # print the generated Go (no go command needed)
rugaroo test test/                   # compile minitest files into one binary and run them
rugaroo test test/foo_test.rb --seed 1 -v   # minitest flags pass through
rugaroo web                          # playground at http://127.0.0.1:8080: Ruby in, Go out, Run button
rugaroo web -static dir -assets URL  # the same playground as static files, compiling in the browser (scripts/deploy-web.sh; live at play.rugaroo.dev)
```

`-race` and `-gcflags` pass through to `go build`. `--int-overflow=MODE`
chooses what an Integer past 64 bits does: `raise` (`RangeError`, the
default), `wrap` (two's-complement wraparound, for hashes and checksums) or
`promote` (a Bignum, as MRI); a `# int_overflow: promote` line in the file
does the same. Compile errors and warnings
point at `file:line` in your Ruby source, and runtime panics map back to Ruby
lines through `//line` directives.

A program is a Ruby file plus whatever it `require_relative`s: each file
loads once, where it is required, as in Ruby. The path must be a string
literal at a file's top level, since rugaroo reads the files when it compiles.
`require "x"` finds your own files in the directories given with `-I`, as
`ruby -I lib main.rb` does (`rugaroo run -I lib main.rb`), and otherwise loads a
supported standard library. Any other name is looked up as an installed gem
(through Bundler when a `Gemfile` sits beside the program), which `build`,
`run` and `gen` then compile in from source; that needs `ruby` on PATH at
build time, and only unmodified rack and Cuba are tested. Run-time
`$LOAD_PATH` changes are not supported.

Compiling a program that loads a gem *runs* it: `build`, `run` and `gen` execute
the program's load phase under `ruby` (and the `Gemfile` beside it, through
Bundler) with your full environment, as `ruby main.rb` would. There is no
sandbox, so don't point rugaroo at source you would not run. The run is bounded
(10 minutes, capped output). Programs without gems never start `ruby`.

## Writing Ruby for rugaroo

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
while running is a compile error instead ([example 119](examples/119_sorbet_sigs/main.rb)).

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
  `Comparable`, `Regexp`/`MatchData` (Ruby syntax over Go regexp engines), `Time`,
  `Random` (same sequence as MRI for a given seed), `Math`, `File`, `Dir`,
  `IO`, `ARGV`, `ENV`, `ARGF`, `$stdin`, `Kernel#system` and backticks,
  `Thread`, `Queue`, `Mutex`, `Ractor`, `Fiber`, `Marshal` (its own bytes, not
  MRI's).
- **Standard library:** `json`, `set`, `time`, `date`, `csv`, `stringio`,
  `strscan`, `digest`, `base64`, `zlib`, `securerandom`, `shellwords`, `uri`,
  `net/http`, `open-uri`, `webrick`, `socket`, `fileutils`, `find`, `pathname`,
  `tempfile`, `open3`, `logger`, `ipaddr`, `etc`, `timeout`, `tsort`, `abbrev`,
  `observer`, `forwardable`, `singleton`, `optparse`, `benchmark`, `cgi`,
  `fiddle` (C libraries without cgo, via purego), `sqlite3` (the gem's API over
  libsqlite3, from the pure-Ruby [gems/sqlite3-fiddle](gems/sqlite3-fiddle)),
  and `minitest` (including `Minitest::Spec`). Unmodified gems compile from
  source: `rack`/`rackup`, `cuba`, `kramdown` and `liquid` (examples
  [116](examples/116_cuba_web/main.rb), [123](examples/123_kramdown/main.rb),
  [124](examples/124_liquid/main.rb)).

[`examples/`](examples/) has one program per feature, from
[inheritance](examples/01_inheritance/main.rb) to a
[Rack-style web framework](examples/32_resty_reflective/main.rb) served over
`net/http`. Each is a normal Ruby script you can run with `ruby`.

The roadmap for the rest of the standard library is issue
[#1](https://github.com/jtarchie/rugaroo/issues/1); CLI work is
[#2](https://github.com/jtarchie/rugaroo/issues/2).

## What doesn't (by design, or not yet)

- **No `eval`, `define_method`, `instance_variable_get`, or runtime
  reopening.** rugaroo compiles a *closed world*: every class and method must be
  visible at compile time.
- **Strings are immutable.** `String` is a Go `string` value, so in-place
  mutation (`<<`, `upcase!`) is unsupported; build new strings or use
  `StringIO`.
- **No encoding tags.** Strings are UTF-8 bytes; `encode` works for UTF-8,
  ASCII-8BIT, US-ASCII, ISO-8859-1 and UTF-16/32, any other encoding is a
  compile error.
- **Integers are 64-bit by default.** Overflow raises `RangeError`;
  `--int-overflow=wrap` or `promote` (Bignum) change that, with limits
  (see the design record).
- **`Hash#[]` returns `V?`**, so typed callers nil-check or use `fetch`.
- **`return`/`break` out of a closure block** is a compile error unless the
  block can be inlined as a loop (e.g. `Array#each`).
- **Overloaded RBS signatures** are not supported.
- **Gems** compile from source with `ruby` on PATH at build time; only a few
  (rack, Cuba, kramdown, liquid, sqlite3) are tested.

Each trade-off has a numbered entry, with its reasoning, in the
[design record](docs/design.md).

## Performance and conformance

A compiled program does no interpretation. Over the workloads in
[`benchmarks/`](benchmarks/) rugaroo runs about 3x faster than MRI;
[BENCHMARKS.md](BENCHMARKS.md) has the table, the comparison columns (YJIT,
spinel, Crystal, hand-written Go) and where rugaroo trails.

Correctness is measured against MRI: every [example](examples/), minitest
suite and compile-error archive runs against `ruby` under `go test`. As a
wider probe, `cmd/rubyspec` runs [ruby/spec](https://github.com/ruby/spec)
under rugaroo; at the last run about 10,000 of its examples pass (roughly 70%
of `core` examples that run), and most of the rest are skipped for something
rugaroo leaves out by design (Bignum, String mutation, encoding tags, `eval`).
`scripts/rubyspec-coverage` and `scripts/spinel-rubyspec` hold the details and
the side-by-side with spinel.

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

Background: [docs/design.md](docs/design.md) (the design record), [docs/dynamic-dispatch.md](docs/dynamic-dispatch.md) (how
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
as minitest files that must pass on both MRI and rugaroo. A cold run can take
more than ten minutes; pass `-timeout 30m`.

```
cmd/rugaroo/          the CLI
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
