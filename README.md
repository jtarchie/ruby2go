# Ruby → Go transpiler

I want to compile a subset of Ruby to Golang. The runtime of Golang supports
enough that I believe it to be possible.

Requirements:

- use this type syntax (https://github.com/soutaro/rbs-inline,
  https://github.com/soutaro/rbs-inline/wiki/Syntax-guide)
- we should be able to support `class`, `module`, `includes`, and inheritance

Anti-goals:

- we don't need to support eval or `define_method`. Class-based virtual
  dispatch on `self` **is** required — inheritance doesn't work without it
  (see [01_inheritance](examples/01_inheritance/)).
- *Amended:* reflective dispatch (`send`, `method_missing`, `const_get`,
  `constants`) is supported, generated from the closed world rather than
  looked up at run time: typed code never pays for it, calls on `untyped`
  values do, and the compiler warns at each one (decisions 28–33,
  [docs/dynamic-dispatch.md](docs/dynamic-dispatch.md)).

I want to support all the native types of Ruby as Golang primitives, but with
methods. These are ideas, and not limited to or the strict implementation.

## Layout

```
rb2go.go              public API: embeds the prelude, calls the compiler
cmd/rb2go/            CLI: rb2go build|run main.rb and rb2go test [paths], like go build|run|test
internal/compiler/    Ruby → Go: declarations, types, codegen
internal/rbs/         the RBS type-syntax subset the compiler understands
prelude.rb            core library entry point; require_relatives prelude/*.rb
prelude/              core library, written in Ruby, compiled by the same transpiler;
                      includes net_http.rb and webrick.rb over Go's net/http
examples/NN_*/main.rb  one feature per program; runs on MRI unchanged
testdata/run/*.rb     smaller behaviour cases, same MRI oracle, no rbs/lint gate
testdata/test/*_test.rb  minitest behaviour checks: MRI must pass them, rb2go must match
testdata/errors/*.txtar  compile-error and warning cases
rb2go_test.go         the integration suite (below)
Gemfile               rbs, rbs-inline, and webrick (the HTTP examples' MRI server)
.golangci.yml         lint config for this repo
.golangci.generated.yml  lint config the tests apply to the generated Go
```

`go test ./...` is the oracle. It fails fast unless `ruby` ≥ 4.0, `bundle
install` has been run, and `golangci-lint` is on PATH; then for every
`examples/*/main.rb`:

1. `rbs-inline --output` succeeds and `rbs validate` passes on its output;
2. the transpiled Go passes `gofmt`, `go vet`, `go build` and
   `golangci-lint` (with `.golangci.generated.yml`);
3. its stdout and exit code equal `ruby main.rb`.

Examples that `require "net/http"` (or any library) get the matching
`rbs -r net-http` flags, so their annotations validate against the
library's signatures.

Each example covers one feature; add one whenever the transpiler grows
something the others don't exercise. There are no golden Go files: MRI's
output is the only expectation, and the generated Go is inspected with
`go run ./cmd/rb2go build -work main.rb` when needed (it keeps `main.go`).

`TestRun` holds `testdata/run/*.rb` to the same MRI comparison but skips the
rbs and lint gates, so each file costs one `go build`. `TestMinitest` runs
`testdata/test/*_test.rb` with `--seed 1`: MRI must pass each file (else the
test is wrong), and rb2go's output must equal MRI's apart from timings
(decision 81). `TestErrors` compiles
each case in `testdata/errors/*.txtar` and checks the `# error: text` /
`# warning: text` lines it declares. A `# skip: reason` line marks a known
failure in either; `RB2GO_RUN_SKIPPED=1` runs them anyway.

## Simplest case

```ruby
"hello, world".upcase
```

```go
type String string
func (s String) Upcase() String { return String(strings.ToUpper(string(s))) }
```

A named Go type over the primitive, methods attached. Zero-cost.

## The class hierarchy

`String` doesn't stand alone: `String.ancestors` is
`[String, Comparable, Object, Kernel, BasicObject]`, and ordinary calls resolve
to every level of it:

```ruby
s = "hello"
s.upcase             # String       → "HELLO"
s < "world"          # Comparable   → true   (String defines <=>, not <)
s.clamp("a", "c")    # Comparable   → "c"
s.then { _1 * 2 }    # Kernel       → "hellohello"
s.equal?(s)          # BasicObject  → true
```

(`Object` defines no methods of its own in MRI — `Object.instance_methods(false)`
is `[]` — everything comes from `Kernel`.)

Example: [00_string_hierarchy](examples/00_string_hierarchy/) —
[main.rb](examples/00_string_hierarchy/main.rb).

### The prelude idea

The real question: can the whole object model be *written in Ruby*, with the
transpiler only providing syntax sugar? Yes — [prelude.rb](prelude.rb) is the
core library, compiled by the same transpiler as user code. Only leaf
primitives drop into Go through an escape hatch.

Prior art: Opal (Ruby→JS, corelib in Ruby, raw JS in `%x{}`), Crystal's prelude
(`@[Primitive]`), TruffleRuby/Rubinius (core in Ruby, `Primitive` leaves).

The prelude is transpiler input only — it reopens core classes, so it never
runs on MRI.

Rules the prelude relies on:

- **`%x{}` escape hatch** — valid Ruby (Prism, rbs-inline, Steep still parse
  the prelude), Opal precedent. Inside, `self` and parameter names bind to the
  Go receiver/args. No `.value` wrapper.
- **Primitives must be annotated** — the Go body is opaque, so the RBS
  signature is the only source of the return type. Pure-Ruby methods can infer.
- **`@go_type`** — tells the transpiler `String` is a named Go `string`, not a
  struct. Classes without it become structs (and are passed as pointers).
  A `@go_type` class has no struct to embed, so subclassing one is a
  compile error.
- **Mixins are just Ruby** — `Comparable#<` and `#clamp` are written once;
  `String` only supplies `<=>`. Compiles to `Comparable_Op_lt[T Comparable_Self[T]]`.
  The constraint interface is *derived from the module body*: whatever the
  module calls on `self` is what an includer must provide. Including
  `Comparable` without `<=>` fails at `go build`, which is the right place.
- **Generics pick the Go shape** — a method with type params (`then`'s `[X]`)
  must be a free func (`Kernel_Then[Self, X]`), since Go methods can't be
  generic. Otherwise emit a method. Derived from the RBS, not hand-picked.
- **Inherited/included methods → free funcs + forwarders.** `Owner_Name[Self]`
  free func, generic over the receiver; each concrete class gets a thin
  forwarding method for the non-generic ones so it satisfies Go interfaces.
  Generic ones (`then`) have no forwarder; call sites call the free func.
- **Bootstrapping** — `Comparable#<` calls `Integer#<`, which must be a
  primitive. Every call chain must bottom out in a `%x{}` leaf. With static
  dispatch, the transpiler can verify this at compile time.
- **No separate runtime** — helpers that aren't methods (e.g. `identical`) go
  in a top-level `%x{}` in the prelude and are emitted verbatim. One source of
  truth, one `//line` map. Output is run through `goimports`, so `%x{}` bodies
  don't declare imports.
- **Testing** — the prelude can't run on MRI, so it's tested by transpiling and
  running the Go. `examples/00_string_hierarchy/main.rb` is the prelude's
  first test.
- **Error mapping** — `%x{}` bodies aren't type-checked until `go build`. Emit
  `//line prelude.rb:N` directives so Go errors point back to Ruby source.
- **Frozen strings.** `# frozen_string_literal: true` semantics only; no
  `upcase!`/`<<`. That's what lets `String` be a Go `string` value. `equal?`
  (identity) on a value type compares the backing pointer — a hack that only
  works because strings are immutable. Go hands back the input when there
  is nothing to do (`strip`, `sub`, `Repeat(s, 1)`, a lone split part),
  where MRI makes a new String, so a method whose result is its receiver's
  own bytes copies them (`rbNewStr`); only that case pays for a copy, and
  `"#{x}"`, `Symbol#to_s` and `dup` always copy. Other shared bytes still
  read as one object: two equal slices of one string (`s.split("b")[0]`
  twice), `strconv`'s small numbers (`5.to_s`), a `""` whose address Go
  drops when boxing it. Fixing those takes a boxed `String`. `frozen?` uses
  the same identity: a string is frozen if it shares a literal's bytes (the
  compiler emits a table of every literal, `rbStringLits`) or was passed to
  `freeze`; strings built at run time are not, as in MRI. Other values:
  immediates, `Regexp` and `Data` are frozen; objects, `Array`, `Hash` and
  `Struct` are not.
- **Literals need wrapping only for interface targets** — Go's untyped
  constants convert to `String` when the parameter is `String`
  (`s.Op_lt("world")` compiles), but become Go `string` when the parameter is
  `any`. Always emit `String("...")` when the target type is an interface or
  `untyped` (see [06_puts](examples/06_puts/)), and when the literal is a
  receiver or bound with `:=` (`(-1).abs`, `case 3`, `false && x`), where Go
  would infer `int`/`string`/`bool`.

## Examples

Each `main.rb` produces identical output under `ruby` and as transpiled Go.
User code is plain Ruby that runs on MRI, so the transpiler is tested by
diffing against MRI output. The design notes below describe the Go shape
each feature compiles to.

### 01 — Inheritance, `super`, overriding

[main.rb](examples/01_inheritance/main.rb)

`describe` is defined on `Shape` but must call the subclass's `area`/`name`.

- **Per-class interface.** Every class gets a Go interface of its full method
  set (own + inherited). Methods defined on a superclass take that interface as
  `self`. Go resolves the vtable.
- **Embedding alone is wrong.** Go embedding promotes methods but binds `self`
  to the embedded field: `Square#describe` would print `Rect`. Forwarders are
  re-emitted on every subclass — same rule as the `String` forwarders.
- **`super`** in `initialize` → call the parent's `_Initialize` on the embedded
  field. `new` → generated `NewRect` = allocate + `initialize`.
- **Struct classes are pointers** (`*Rect`). Value semantics only for
  `@go_type` frozen primitives.
- **`Float#to_s`** is a primitive trap: Go prints `6`, Ruby `6.0`. Expect many
  of these in the prelude.

### 02 — Enumerable, blocks, generic containers

[main.rb](examples/02_enumerable/main.rb)

`Enumerable` written once in the prelude against `each`; Go generics, with
type arguments spelled out from the Ruby types (Go would infer `*Foo` or an
untyped constant from the arguments).

- **`each` → `iter.Seq[E]`** (Go 1.23+ range-over-func). A Ruby
  `def each; ...; yield x; ...; end` body compiles to a push iterator:
  `yield x` → `if !yield(x) { return }`. This is real compiler work, not a
  mapping table.
- **Constraint derived from the module body.** `Enumerable` calls `self.each` →
  `Enumerable_Self[E]{ Each() iter.Seq[E] }`.
- **Go infers `E`** from the constraint method, so call sites don't spell type
  args.
- **Generic methods → free funcs.** `map`'s `[U]` means
  `Enumerable_Map(nums, ...)` at every call site. Verbose, unavoidable.
- **Mutable types are pointers.** `b = a; b << 1` mutates `a` in Ruby →
  `Array` is always `*Array[E]`. Opposite of frozen `String`. Rule: frozen
  `@go_type` → value; mutable → pointer.
- **`&:even?`** resolves statically to a closure. No `Symbol#to_proc` at
  runtime.
- **Non-local return is the hidden problem.** `arr.each { |x| return x if x > 2 }`
  is everyday Ruby; a Go closure can't return from the enclosing func.
  Options: (a) when the receiver is a known `Array`/`Hash`/`Range`, inline the
  block as a `for` loop — no closure, `return` just works; (b) otherwise panic
  with a per-call sentinel and `recover` at the method boundary. Do (a) where
  possible, (b) as fallback.

### 03 — `nil` and optional types

[main.rb](examples/03_nil/main.rb)

`T?`, `&.`, `||`, truthiness narrowing. The biggest cross-cutting decision: it
affects `Hash#[]`, `find`, `first`, and every `if x`.

- **`T?` → `*T`** for `@go_type` values. `nil` = nil pointer. Struct classes
  are already pointers, so `Rect?` is still `*Rect` — one representation.
- **Narrowing is free.** `if name` → `if name != nil`; inside, `name.Size()`
  works because Go auto-derefs `*String` for value-receiver methods.
- **`&.` and `||` become statements.** Short-circuit expressions need
  temporaries in Go; the transpiler lifts expressions to statement sequences.
  Union `String? | String` collapses to `String`.
- **Truthiness rule:** only `nil` and `false` are falsy. `Boolean` → native
  `!`; `T?` → `!= nil`, except `Boolean?`, which can hold `false` (a
  `Hash[String, bool]` lookup): `rbTruthyOpt(x)`, i.e. `x != nil && bool(*x)`;
  everything else → constant true (warn).
- **Methods on `nil`.** `x.inspect` where `x: String?` → static branch:
  nil → `NilClass_Inspect()`, else `x.Inspect()`. No runtime `NilClass`
  class (`nil.class` is a bare class object, decision 19).
- **RBS core lies about `Hash#[]`.** Core signatures say `(K) -> V` (because
  of defaults); the prelude should declare `(K) -> V?`. `Hash.new(0)` +
  `h[k] += 1` then fails to typecheck — see 05.
- **Alternative rejected:** `Option[T]` struct `{v T; ok bool}`. Uniform, no
  pointer escape, but `if x` → `x.ok` everywhere and doubles every prelude
  signature. `*T` wins on simplicity; revisit only if escapes show in profiles.

### 04 — Exceptions

[main.rb](examples/04_exceptions/main.rb)

`raise`/`rescue`/`ensure`, user-defined hierarchies. Feasibility proof more than
a design driver.

- **`raise` → `panic`, `rescue` → `defer`/`recover`.** Alternative is Go
  `error` returns: idiomatic, but changes every signature and needs
  raise-analysis. `panic` keeps signatures and matches Ruby unwinding. Slow,
  which is fine — exceptions are exceptional.
- **Class matching via marker methods.** `rescue AppError` must match
  `NotFound`. `r.(*AppError)` fails on `*NotFound`;
  `r.(interface{ isAppError() })` succeeds because embedding promotes the
  marker. Embedding works here (not in 01) because markers never touch `self`.
- **Catch-all `rescue`** also catches Go runtime panics (nil deref, index out
  of range). Aligns with Ruby (`NoMethodError`, `IndexError` are
  `StandardError`s). Wrap Go runtime errors into prelude exception types in the
  recover path.
- **`ensure`** → outer `defer`; LIFO gives rescue-then-ensure. `return` inside
  `rescue` needs a named result. Go's `return`/`break`/`continue` stop at the
  `func(){}` wrapper, so a `return`, `break`, `next` (or an iterator's stopped
  `yield`) that leaves it sets the wrapper's flag and is re-issued after the call.
- **Not shown:** `retry`, bare `raise` re-raise, `backtrace`, `rescue` in
  blocks. `retry` → loop around the `func(){}` wrapper.

### 05 — A real program: word count

[main.rb](examples/05_word_count/main.rb)

Three mechanism tests prove feasibility; one small real program shows what a
user would actually write, and surfaces what the toys don't.

- **Hash ordering.** Go maps are unordered *and randomized per run*; Ruby
  preserves insertion order. This example happens not to depend on it (the full
  sort key decides), but any program that prints or iterates an unsorted hash
  would have nondeterministic output — which also kills MRI-diff testing.
  Recommended: ordered `Hash` as `map[K]int` index into `[]entry` with
  tombstones (Python-dict style, ~40 lines, O(1) ops). Omitted here
  for brevity.
- **`tally` instead of `Hash.new(0)` + `+= 1`.** The default-value idiom needs
  the transpiler to track hash defaults through types (or accept RBS's unsound
  `V`). `tally` types cleanly. Prelude should still support `Hash.new(default)`
  eventually — it's common.
- **`[-n, w]` sort key is an RBS tuple `[Integer, String]`** → `Tuple2[A,B]`,
  lexicographic `<=>`. Because Go constraints are satisfied by *methods*, each
  tuple instantiation used as a `Comparable` needs a named wrapper type
  (`Tuple2IS`) — generated on demand.
- **`sort_by`** = compute keys once, then sort. Ruby's `sort_by` is unstable;
  `slices.SortFunc` is too. Same contract.
- **`|w, n|` block destructuring** of a pair → `p.F0, p.F1`. Arity-based
  auto-splat resolved from the element type.
- **`first(3)`** exercises early termination: `break` in the `range` loop makes
  `yield` return false and stops the upstream iterator. Lazy chains are cheap.
- **What this surfaced that the toys didn't:** hash ordering, tuples-as-keys,
  wrapper types for generic constraints, interpolation calling `to_s` on every
  part. Expect every real program to add 2–3 prelude items.

### 06 — `puts`

[main.rb](examples/06_puts/main.rb) ·
[prelude `Kernel#puts`](prelude.rb)

`puts` is not `fmt.Println`: `nil` → blank line, arrays flatten recursively,
no doubled trailing `\n`, bare `puts` → newline, `to_s` dispatch on user
classes. It's written in the prelude as plain Ruby; only `__write` is a
primitive.

- **`case a when nil / when Array` → Go type switch.** `untyped` → `any`.
  First example that needs a runtime type test — everything before was static.
- **Any other `case` → `switch {}` over `cond === subj`.** A class is
  `is_a?`; a condition whose class defines `===` (`Regexp`, a user class)
  calls it, dynamically when the condition is `untyped` or `T?`; anything
  else is `==`, `Object#===`'s default, so typed `when 0` stays a typed `==`.
- **`when Array` vs. generics.** A Go type switch can't match `*Array[E]` for
  unknown `E`. Every `Array` instantiation implements a non-generic
  `Array_Any{ ToAAny() []any }`; the switch matches that. General rule for
  `when GenericClass`: match a non-generic interface the class implements.
- **`a.to_s` on `untyped`** → `a.(Kernel_ToS).ToS()`. Safe because `Kernel`
  guarantees `to_s` on every object; the prelude must give every `@go_type`
  and struct class a `ToS`.
- **`T?` into `any` needs `Opt()`.** A nil `*String` stored in `any` is a
  *typed* nil, so `case nil` misses it. The transpiler knows the static type
  is `String?` and emits `Opt(words.Index(5))`, converting to untyped nil.
- **Untyped literals into `any` become Go `string`**, not `String` — the
  first run of this example panicked on exactly that. Emit `String("...")`
  for interface-typed targets.
- **Splats into a rest param** → one fresh slice: `f(1, *a)` →
  `f(slices.Concat[[]T]([]T{1}, *a)...)`. Go spreads only a lone slice, and
  Ruby's rest param is a new array, so the callee must not alias the caller's.
  Elements convert (`rbSplat`) when the Go types differ (`Array[Integer]` into `*untyped`).
- **Output is buffered** (`bufio.Writer`), flushed by `defer` in `main` —
  runs on return and on panic, so partial output before a crash matches Ruby.
  `os.Exit` skips defers: `Kernel#exit` must flush first. A terminal
  stdout flushes on every write, and SIGINT/SIGTERM flush (decision 60).
  Interleaving with `$stderr` on a pipe would need `$stdout.sync`-style
  flushing; not handled yet.
- **Return type `nil` → no Go return value.** `puts`'s result is rarely
  used; when it is (`x || puts(...)`, `puts(...).inspect`), the call runs as
  a statement and the value is `nil`. A block whose value is nil
  (`map { |n| puts n }`) binds its type variable to `untyped`.
- **`//line` directive granularity.** The panic trace mapped back to
  `prelude.rb`/`main.rb`, but to the wrong line inside `puts` — one directive
  per function isn't enough; emit one per statement.

### 07–24 — one feature each

Ruby 4.0 syntax (a line starting with `&&`, `it`), control flow, struct
classes, strings, hashes, optionals, exception flow, an uncaught exception,
namespaces and constants, class methods and class objects, symbols and
braceless hash arguments, regexps, JSON, `untyped` and `is_a?`, `||=`,
multiple assignment, threads, and a WEBrick server with a `Net::HTTP`
client in one process.

### 26–31 — reflection, and the Ruby resty leans on

Class objects and constant reflection (26), `extend` and `&block` (27),
`Struct.new` and `Data.define` (28), `method_missing` and `respond_to?` on
typed receivers (29), dynamic `send` on untyped values (30), and everyday
Ruby semantics such as `rescue` modifiers, `return` in `ensure`, and `&&`
returning values (31).

### 25 and 32 — resty: a web framework over `net/http`

[main.rb](examples/25_resty/main.rb) is a typed port of
[jtarchie/resty](https://github.com/jtarchie/resty), a Rack framework that
forces RESTful conventions. The program mounts it on WEBrick, then drives
it with `Net::HTTP`, replaying resty's own integration specs over a real
socket. On Go, WEBrick and `Net::HTTP` are the prelude's classes over
`net/http`'s `http.Server` and `http.Client`; the printed transcript is
byte-for-byte MRI's.

What survives from resty is its architecture: a request picks its action
class and its format class by asking each candidate *class* `matches?`
(`Actions::ALL.detect { |a| a.matches?(request) }`), then instantiates the
winner through `singleton(Actions::Base)`, and actions find the app's
controller action by `self.class.name`. What had to change is exactly the
README's anti-goal list:

- `"#{ns}::#{path.camelize}Controller".constantize` and
  `const_get(action_name)` become a typed registry,
  `Hash[String, singleton(Resty::Action)]`;
- `Actions.constants.map { const_get }` becomes an explicit `ALL` list;
- `NullController`'s `method_missing` becomes a `NullAction` class;
- `Rack::Request` becomes `Resty::Request`, built from WEBrick's request;
  ActiveRecord becomes an in-memory `Resty::Model`;
- idioms whose types are unions (`path =~ re || …` as a `bool`,
  `match(...)[1]` on a possible nil) are spelled with `match?` and a check.

[Example 32](examples/32_resty_reflective/main.rb) then compiles resty's
own `lib/` code as written, reflection and all: `constantize`,
`const_get`, `constants.map { const_get }`, `NullAction`'s
`method_missing`, `Struct.new ... do`, `extend Enumerable`. It adds type
annotations and changes two lines (marked `rb2go:`); `Rack::Request`,
ActiveSupport's `camelize`/`constantize` and ActiveRecord are small shims.
Its transcript is the same as example 25's. The compiler warns at each of
its dynamic calls, which is where the program's types run out.

## Open decisions

Decisions taken while building `rb2go` are recorded under the item they
resolve; anything not listed is still open.

1. Ordered `Hash`: **decided, ordered.** `Hash[K, V]` is
   `struct { keys []K; vals map[K]V }` behind `@go_type`; iteration follows
   insertion order, so output is deterministic and MRI-diffable. Deletion is
   O(n) over the key list (no tombstones yet; add them if a profile says so).
   *Revised:* the struct gained `iter int`, a count of running iterators
   (MRI's `iter_lev`), so mutation during `each` behaves as in MRI: `[]=` of
   a new key raises `RuntimeError`, and `delete` copies the key list rather
   than shifting the one being ranged over, whose deleted keys are skipped.
   `Array#each` likewise loops by index over the live length.
   *Revised:* keys match by Ruby's `eql?`/`hash`, not Go `==`, which keyed
   arrays, hashes, Regexps, Structs/Data and `T?` boxes by pointer. The
   struct gained `idx`, hash buckets of the keys that match by value (those
   defining `eql?` and `hash`, and `T?` boxes by what they point at); a
   lookup first maps its key to the stored `eql?` one. Keys whose Go `==`
   is `eql?` (`String`, `Integer`, `Symbol`, `Float`, `Boolean`, tuples of
   them) skip the index on a flag set once per Hash, so typed hashes over
   them cost about what they did. `uniq`
   uses the same index; `tally`/`group_by` are Hash-based. Mutating a key
   after inserting it is not detected (MRI needs `rehash` there too).
2. `TrueClass`/`FalseClass` vs. `Boolean`: **decided, one `Boolean`** (a Go
   `bool`). `true`/`false` literals are untyped constants that convert to
   `Boolean`, and get wrapped (`Boolean(true)`) only when the target is
   `untyped`. `inspect`/`to_s` live on `Boolean`.
   *Amended:* `TrueClass`, `FalseClass` and `NilClass` exist as marker
   classes with no instances: `.class` of a Boolean picks one by value,
   and `is_a?`/`case`-`when` against them test the value (so `when
   TrueClass` leaves the Go type switch). nil's `to_a`/`to_h`/`to_i`/`to_f`
   work on a nil literal and on an untyped nil.
3. Operator name table: **decided** — `==`→`Op_eq`, `!=`→`Op_ne`,
   `<=>`→`Op_cmp`, `<`→`Op_lt`, `<=`→`Op_le`, `>`→`Op_gt`, `>=`→`Op_ge`,
   `+`→`Op_plus`, `-`→`Op_minus`, `*`→`Op_mul`, `/`→`Op_div`, `%`→`Op_mod`,
   `**`→`Op_pow`, unary `-`→`Op_neg`, `+@`→`Op_pos`, `!`→`Op_not`,
   `~`→`Op_inv`, `<<`→`Op_shl`, `>>`→`Op_shr`, `&`→`Op_bitAnd`,
   `|`→`Op_bitOr`, `^`→`Op_bitXor`, `=~`→`Op_eqTilde`, `!~`→`Op_notTilde`,
   `===`→`Op_eqq`, `[]`→`Op_idx`, `[]=`→`Op_idxSet`, `` ` ``→`Op_backtick`.
   Everything else camel-cases with `?`→`Q`, `!`→`Bang`, `=`→`Set`; leading
   underscores are kept (`__write`→`__Write`). The table has to stay
   injective against camel-cased names too. Camel-casing upper-cases the
   letter after every `_`, so an `_` before a lowercase letter marks a name
   it cannot produce: every operator carries one, and a suffix-less name
   whose last word is `q`, `bang` or `set` keeps it as written
   (`empty_q`→`Empty_q`, where `empty?` is `EmptyQ`). What camel-casing still
   merges, capitals (`foo_bar`/`fooBar`) and a digit after `_`
   (`utf_8`/`utf8`), is a compile error when both names reach one class or
   are both called dynamically. *(Revised: the table used to be `Eq`,
   `Plus`, `Div`, `Pos`, `Idx`, …, which `eq`, `plus`, `Integer#div`,
   `IO#pos` and `idx` camel-case to as well, so a class defining both got a
   duplicate Go method or `rbDyn` dispatcher and `go build` failed; backtick
   was missing.)*
   *Revised:* a `<=>` declared `-> Integer?` is Go's `cmpNil` (lowercase,
   so no camel-cased name reaches it), and the compiler gives its class an
   `Op_cmp(T) Integer` adapter that raises MRI's `ArgumentError: comparison
   of X with Y failed` where `cmpNil` answers nil (MRI's `rb_cmpint`).
   `Comparable_Self` and `rbCmp` call `Op_cmp`, so they still need an
   Integer. Float's `<=>` is one: it answers nil for NaN, as MRI's does,
   so a typed `a <=> b` on Floats is `Integer?` and needs a nil check
   before arithmetic, while `between?`, `clamp`, `sort`, `min` and `max`
   raise on NaN. `between?` and `clamp` now compare with `<=>`, as MRI's
   do, since Float's own `<` answers false for NaN instead of raising.
4. Non-local `return`/`break`/`next` in blocks: **decided, inline loops
   only.** A method whose block returns `void` compiles to a Go iterator
   (`iter.Seq`/`iter.Seq2`) and every call site with a block becomes a
   `for range` loop, so `return`, `break` and `next` are plain Go. Blocks
   passed to value-returning methods (`map`, `select`, `then`, …) are Go
   closures; `next` is `return` of nil (`false` in a `bool` block), and
   `return`/`break` inside them is a compile error. The sentinel-panic
   fallback is not implemented.
   A method is an iterator only when the block *and* the method return
   nothing, the block is only yielded to, and nothing rescues around the
   yield (Go forbids a range function from recovering a panic raised in
   the loop body). A `%x{}` leaf is an iterator only if its Go builds one
   (`func(yield ...`). Everything else takes a closure: `Thread.new { }`
   returns a Thread, `mount_proc(path) { }` stores its block.
   A closure that overrides an iterator (an `each` that rescues around
   `yield`, under `include Enumerable`) moves to `Each_blk`, and an
   `iter.Seq` adapter (`rbSeq`) keeps `Each` for the iterator's callers.
   A loop that stops early unwinds the closure with a sentinel panic, so
   `ensure` runs as on MRI's `break`; an exception from the loop body that
   the closure rescues aborts the Go program instead, since a range
   function cannot recover one. *(Revised: such a class failed `go build`,
   and an unannotated override inherited iterator-ness despite its rescue.)*
5. `Hash.new(default)` / `Hash#[]` typing: **decided, `Hash#[]` is
   `(K) -> V?`** and there is no default value. `Hash#fetch(k, default)`
   covers the common case; `tally`/`group_by` are written with `||`.
   `Hash.new(0)` + `h[k] += 1` is not supported.
6. Prelude coverage: **done.** Every example compiles against `prelude/`
   alone; the subsets that used to be inlined in `examples/*/main.go` are
   in `prelude/{object,integer,float,string,enumerable,array,hash,exception}.rb`.
   The hand-written `main.go` files were later dropped; MRI is the oracle.
7. `T?` representation: **decided, uniformly `*T`** — including for struct
   classes, whose non-optional representation is already an interface
   (`Rect` is `RectI`, `Rect?` is `*RectI`). The README's "one
   representation" shortcut would make `E?` inside a generic container mean
   something different from `Rect?` outside it; uniform boxing keeps
   generics honest at the cost of a `Ref`/`Opt` at the boundary.
   A consequence: `Array[Integer]` is `[]Integer` and cannot hold nil, so
   `a[i] = v` past the end pads the gap with the zero value (`0`, `""`,
   `false`) where MRI pads with nil. That is right whenever the gap is
   filled before it is read (`out[perm[i]] = x`); a program that reads the
   holes needs an element type that holds nil (`Array[Integer?]`, or an
   unannotated `[]`). Raising instead would reject the fill-later programs
   MRI runs, and tracking holes would cost every typed read.
   Generic code holding `E = T?` passes the box itself to `any`-typed
   helpers (`inspect`, `==`, `<=>`, `to_json`, `compact`, untyped views),
   where a nil `*T` is a non-nil interface and a non-nil one has the wrong
   method set. A generated `rbUnbox` type switch over every concrete `T?`
   the program renders opens it there; `rbCmp` tries the typed `Op_cmp` first
   and only falls back to the box path when that fails.
   Ruby has one nil, so `T??` is `T?` and `untyped?` is `untyped`: a
   generic `E?` result instantiated with `E = T?` (Go `**T`, e.g.
   `Array[Integer?]#[]`) or `E = untyped` (`*any`) is flattened at the
   call site. *(Revised: the nested box used to reach user code, so a nil
   element read as non-nil.)*
8. Dispatch shape: struct classes get an interface (`ShapeI`) of their full
   method set plus `_Shape() *Shape` accessors for every struct in the
   chain (ivar access from free functions, and the marker `rescue` matches
   on). Methods defined on struct classes and modules are free functions
   generic over `Self`, forwarded by a Go method on every concrete class.
   A struct class's forwarder instantiates `Self` with the interface its
   own signature uses for `self` (`Comparable_Op_lt[VersionI]`, not
   `[*Version]`): `*Version`'s methods take `VersionI`, so it can't satisfy
   `Comparable_Self[*Version]` or pass a `(self)` argument on.
   `BasicObject`, `Object` and `Kernel` are "universal": their `Self` is
   `any`, since primitives inherit from them too.
   Default arguments: a plain literal default is filled in at the call
   site. Any other (`b = a.size`, `x = @x`, `g = helper`, a constant) runs
   in the callee, as in MRI: the method's Go func takes `rbArgc int` (the
   count of positional args given) first, callers pass zero values for the
   rest, and the body evaluates the missing defaults in order. Every def
   of that name along an ancestor chain takes `rbArgc` too, so overrides
   keep one Go signature and run their own defaults. *(Revised: all
   defaults used to be evaluated at the call site, in the caller's scope.)*
   A subclass interface must hold every ancestor's method with the same Go
   signature, so an override whose signature differs from its parent's
   (another arity, a narrower return such as `-> Sub` for `-> Base`) gets
   its own Go name (`F_ofSub`), and the class answers to the parent's name
   through an adapter: it converts arguments and result (up a struct
   hierarchy, or through `untyped`), or raises MRI's ArgumentError for an
   argument count the override cannot take. Calls typed as the subclass
   reach the override directly. An override the adapter cannot bridge (an
   unrelated return type, a block, a parent's rest parameter) is a compile
   error naming both signatures. *(Revised: every such override failed
   `go build`, "wrong type for method".)*
9. Module constraints are derived from the module body, as the README
   says: `Comparable_Self[Self]` lists what `Comparable`'s methods call on
   `self` (including `self.X(` inside `%x{}`), not every module method.
   Primitive classes get forwarders only for those; everything else is
   called through the free function. This is also what avoids Go's
   "instantiation cycle": a forwarder such as `Hash[K,V].Tally` would
   instantiate `Hash[[K,V], Integer]`, whose forwarders instantiate the
   next size up, forever. Inside a module method `self` is some includer,
   not the module: `Object` methods called on it (`to_s`, `inspect`,
   `"#{self}"`) join the constraint so the includer's overrides run,
   `self.class` is a `_ClassObj()` accessor (typed `Class`) the constraint
   lists, and `is_a?(C)`/`respond_to?(:m)` ask the includer at run time.
   *(Revised: `self` used to be typed as the module, so `self.class` was
   the module and `is_a?`/`respond_to?` folded to the module's answer.)*
   A `super` the module cannot resolve itself (its target is the
   includer's superclass or a later module, different per includer) is a
   call to `self._Super_M_name(...)`: the constraint requires it, and
   every includer implements it by calling the definition that follows
   `M#name` in its own ancestors, or raising MRI's `NoMethodError`.
   *(Revised: this was a compile error.)*
10. Type parameters are all constrained `comparable`. Every generated Go
    type satisfies it (strings, ints, pointers, interfaces, tuples of
    those), and it is what `map[K]` and `tally` need; deriving the
    constraint per parameter bought nothing.
11. `raise` is an intrinsic: `raise Klass, msg` ≡ `raise Klass.new(msg)`,
    `raise "msg"` ≡ `RuntimeError.new`. Uncaught exceptions flush stdout,
    print `message (Class)` to stderr and exit 1, like MRI. Go runtime
    panics (`index out of range`, divide by zero) are wrapped into
    `IndexError`/`ZeroDivisionError`/`StandardError` on the way into a
    `rescue`. *Revised:* `Kernel#exit` raises `SystemExit` instead of
    calling `os.Exit`, so `ensure` blocks and `rescue Exception` run as in
    MRI; uncaught, it exits silently with its status. In a thread it still
    exits on the spot, skipping the main thread's `ensure`s.
12. Overloads (`#|`) are not supported, so `first`/`take` require a count
    (`arr.first(3)`; use `arr[0]` for the head) and `Array#[]` takes one
    Integer. `split` takes an optional separator through `?String?`.
    *Revised:* `Hash#fetch` needs MRI's second overload, since a nil
    default is returned rather than meaning "no default" (`fetch(k, nil)`
    is `nil` for a missing key). A `fetch(k, d)` whose `d` may be nil
    (`nil`, `T?` or `untyped`, as in every dynamic call) goes to
    `Hash#__fetch_opt`, `(K, V?) -> V?`; any other default keeps
    `(K, ?V?) -> V`, so `h.fetch(k, 0) + 1` stays an Integer.
    *Revised:* Integer and Float mix anyway, as MRI's `coerce` does. When
    an Integer or Float method's numeric argument is the other class, the
    compiler widens the Integer side and calls Float's method (`a / 2.0` is
    `Float(a).Op_div(2.0)`, still unboxed); an operator Float lacks (`%`) is a
    compile error. Comparable's methods take both as `rbNum`, one Go type,
    because `2.5.clamp(1, 2)` returns the bound `2` itself, so the result is
    `untyped`. A local assigned both joins to `untyped`
    (`total = 0; total += 1.5`), so its later arithmetic is dynamic, and
    dynamic wrappers widen the same way. A Float where an Integer is
    expected is a compile error, not Go's silent constant truncation.
    *Revised:* overloads by convention. A call with an argument count the
    method cannot take goes to the receiver class's `__<name>_<count>`, and
    one whose sole argument is of class `C` to `__<name>_<c>` (`C`
    snake-cased; an operator is named by its Go name minus `Op_`, `?` and
    `!` are spelt `_q` and `_bang`, so
    `[]` with a Range is `__idx_range` and `Time - Time` is
    `__minus_time`), when defined. So `arr.first` (`E?`), `arr.last(2)`,
    `arr[1, 2]`, `arr[1..]` and `str[0...-1]` work; everything else still
    takes one signature.
13. Empty `[]`/`{}` literals without an annotation are `Array[untyped]` /
    `Hash[untyped, untyped]`, which is what Ruby's are; any other missing
    type is an error, and an unannotated override inherits the parent's
    signature (never `untyped`).
    *Amended:* a local first assigned an unannotated `[]`/`{}` is typed by
    the join of what the same method later puts in it (`x << v`, `push`,
    `unshift`, `x[i] = v`, which adds nil for the gap, `h[k] = v`,
    `store`), re-running local analysis until nothing new is typed, since
    one typed literal can type the next. It stays untyped when that join
    is or holds `untyped`, when the typed version fails to compile, or when
    it adds an `rbAs` conversion: a typed array converted to another
    instantiation is a copy, and Ruby's aliasing would be lost. This is
    unification local to one method body; types never flow between
    methods ([example 33](examples/33_empty_literals/main.rb)).
14. Locals are inferred from their assignments (joined across branches:
    `nil` + `String` → `String?`, and `x = nil` then `x ||= v` counts as
    assigning `v`) and hoisted to a `var` at the top of
    their Ruby scope (the method, or the block's Go body) when Go's block
    scoping would otherwise hide them. Scopes are resolved as prism does:
    block params, `|x; y|` block locals and locals first assigned in a
    block belong to that block, fresh on every call and typed apart from
    same-named locals elsewhere in the method. `rescue => e` binds a local
    of the enclosing scope, nil after the `begin` when nothing was rescued;
    when `e` was assigned before, it keeps its one type and the clause's
    binding shadows it instead. *(Revised: locals used to be keyed by name
    for the whole method, so sibling blocks shared one type and block
    locals were hoisted to the function, keeping their value across
    iterations.)* A local that some path reads before any assignment ran
    (assigned in one branch only, in a loop body, or in a `begin` body a
    raise can cut short) is `nil` there, as in Ruby, so it is typed `T?`; an
    assignment of a non-nil value makes it read as `T` for the rest of that
    branch or loop body. A definite-assignment pass decides this: `return`,
    `break`, `next` and `raise` end a path, `while true` leaves only by
    `break`, a block's assignments never count outside it, an `ensure`
    assumes nothing of its `begin`, and a `rescue` assumes what the body
    assigned before each explicit `raise`. A raise from a callee counts as
    coming after the body's assignments, and an annotated local (`#: T`)
    keeps `T`: a skipped assignment then reads Go's zero value (`nil` for
    objects, `0` for an Integer). *(Revised: every such local was hoisted as
    `T`, so a skipped assignment read `0`, `""` or `false`.)*
    Lifted temporaries for `&.`, `||`, ternaries, `case`-expressions and the
    value of an attribute write used as a value (`r = (o.x = v)` is `v`, not
    the setter's result) are computed before the statement they belong to,
    so their side effects run slightly earlier than MRI would run them.
15. Instance variables are typed from `attr_*` annotations, `# @rbs @x: T`,
    or a dry run of the class's method bodies (`initialize` first); an ivar
    that is only ever assigned `nil` needs an annotation.
    *Amended:* an ivar first assigned an unannotated `[]`/`{}` is typed
    as decision 13 types locals, from what the class's methods put in it,
    re-discovering until nothing new is typed. Evidence that is untyped
    only because it reads the container itself (`h[k] = (h[k] || 0) + 1`)
    is skipped. Every typing is checked by a dry-run emission of the
    classes that see the ivar: one that fails to compile or adds a
    conversion is dropped, trying all, then all but one, then one at a
    time ([example 35](examples/35_ivar_containers/main.rb)).
16. `%x{}` bodies are Ruby xstrings, so Ruby escape processing applies to
    the Go inside them: write `\\n` for a Go `\n`, `\#{` for a literal `#{`,
    and keep braces balanced (no `"{"` in Go strings). A one-line body of a
    non-void method gets `return` prepended.
17. Namespaces: a class's Go name joins its constant path with `_`
    (`Resty::Actions::Show` → `Resty_Actions_Show`); a generated map gives
    messages and `inspect` the Ruby name back. A user class or constant
    whose Go name, or a name generated from it (`NewX`, `XI`, `X_Meta`,
    `X_<method>`), is already taken by a runtime helper (`Opt`, `Ref`) or an
    earlier class (`User`'s `NewUser`, `Foo::Bar`'s `Foo_Bar`) gains a
    trailing `_`; prelude names never change, since `%x{}` spells them.
    Locals the compiler introduces (`t1_`, `r_`, `ret_`, `rest_`) end in a
    single `_`, and a Ruby local ending in `_` gets another, as a Go
    keyword or builtin already did (`len` → `len_`). A subclass struct embeds its parent
    through an alias (`super_Calc_`), so a method named like the parent
    (`Calc#calc`) does not meet the embedded field. Constants resolve as Ruby
    does: the lexical scope innermost-out (`Module.nesting`), then the
    innermost class's ancestors, then top level; `class A::B` compact form
    does not put `A` in scope. RBS names in annotations resolve the same way.
    *(Revised: names used to be taken as written, so a class named `Opt` or
    `NewUser`, `Foo_Bar` beside `Foo::Bar`, or `Calc#calc` with a subclass
    failed `go build`, and a local named `r`, `p`, `t1` or `ret_` met a
    generated one.)*
18. Constants are Go package variables, typed by `#: T` or by their
    initializer, and assigned in `main` in source order (prelude first), as
    MRI evaluates them. *(Revised: they used to initialize before `main` in
    Go's dependency order.)* That is where main.rb's class bodies run: an
    initializer inside a class or module has the class object as `self`
    (`FREEZING = of(0)`), and a user-defined `inherited` is called where a
    class is first opened, `included`/`extended` at the `include`/`extend`,
    in the same source order. *(Revised: initializers ran with `main` as
    `self`, and hooks were compiled but never called.)* A constant that
    code can read before its assignment runs (one assigned after a
    statement, a hook, or an initializer that may call a main.rb method)
    gets a flag, and its reads raise `NameError` while it is unset, as in
    MRI. Constants assigned before any such code, the usual case, are read
    directly. *(Revised: such reads saw the Go zero value.)*
19. Class methods: every class and module (except `BasicObject` and
    `Kernel`) gets a metaclass — a struct class holding the
    class methods, inheriting from the parent's metaclass — and one
    instance of it is the class object. Class methods therefore inherit and
    dispatch virtually like instance methods; `singleton(C)` is the
    metaclass's interface; `self.class` is a per-class accessor. Each
    metaclass gets generated `new`, `name`, `to_s` and `inspect`, except
    `name`/`to_s`/`inspect` that a user's `def self.` on it or an ancestor
    already defines, so they inherit as in MRI. `new` is
    kept off the shared interface because subclasses may change
    `initialize`: `klass.new(...)` through `singleton(Base)` type-asserts
    for a matching `New`, failing at run time where Ruby would raise
    `ArgumentError`. `Foo.new` on a constant stays a direct constructor
    call. `class << self` is not supported. Metaclasses inherit from the
    parent's metaclass, else from prelude `Class` (modules: `Module`), so
    a value typed `Module` can hold any class object. `x.class` whose
    class is known only at run time (`untyped`, `Object`, a module type,
    `nil`, `T?`) is a generated type switch over the `@go_type` classes
    and struct hierarchies, typed `Class`; a module's `self` asks the
    includer instead (decision 9). nil's class object is a bare `Class`
    named `NilClass`, not a constant code can name. *(Revised: these were
    a build error, `NoMethodError`, or the module itself.)*
20. `T?` where `T` is expected is a compile error (check it first:
    `if x`, `return unless x`, `x ||= …`, `&.`). `untyped?` is untyped:
    passing it on checks the type at run time, raising MRI's `TypeError`
    ("no implicit conversion of Integer into String") as a dynamic call's
    arguments do (decision 32), except to a `T | untyped` parameter (the
    only union besides `T | nil`): typed arguments are checked against
    `T`, untyped ones pass unasserted and the method handles them.
    *Amended:* where a Boolean is expected (a predicate block's result, a
    `bool` parameter) `T?` is its truthiness, as `untyped` already was:
    `xs.reject { |x| seen.add?(x) }`. Regexp
    subjects use it, so a literal `nil` is an error but an untyped `nil`
    or Symbol matches as in MRI. *Calling a method* on `T?` raises
    `NoMethodError` when it is nil, as in Ruby, and the compiler warns;
    methods `NilClass` defines (`to_s`, `inspect`, `==`, `!`, `to_i`,
    `to_f`, `to_a`, `to_h`, `=~`) give nil's answer instead (`0`, `0.0`,
    `[]`, `{}`, `nil`) when `T`'s method returns a type that holds it.
    *(Revised: `to_i`/`to_f`/`to_a`/`to_h`/`=~` raised, so an unmatched
    group's `m[2].to_i` failed where MRI gives 0.)*
    Narrowing follows `if x`, `if x.is_a?(C)`, `&&`, `case x` (a one-class
    `when C` arm, and the `else` after `when nil`), and early-exit guards
    (`return … unless cond`, `return if x.nil?`) for the rest of the
    block; attribute reads on `self` narrow like locals; reassigning drops
    the narrowings. Likewise a value of class `C` where `T` is expected
    (an argument, element, key, block result or ivar write) is a compile
    error unless `C` is `T` or inherits/includes it, with type arguments
    checked the same way. Go would convert a literal silently, so only
    an Integer literal where a Float is expected passes (Float's
    operators take Integers); `"a"` is not a `Symbol` (decision 23).
21. `is_a?`/`kind_of?` is a constant when static types decide it and a Go
    type assertion otherwise. There is no runtime record of included
    modules, so `is_a?(SomeModule)` (and `when SomeModule`) on an untyped
    value, or on a class with a subclass including it, is a compile error.
    Narrowing an untyped local to `Array` views it as `Array[untyped]`.
    *Revised:* Go instantiations are invariant, so the view of a typed
    `Array[Integer]` is a copy (`_to_any`) and writes through it were lost.
    Block-less calls on the narrowed local are now sent to the value itself
    through dynamic dispatch (decision 32), so `x << 1` and `x[k] = v`
    reach the caller's container (an element it cannot hold raises
    `TypeError`), and passing it on as `untyped` passes the value; calls
    with a block or type parameters (`each`, `map`) and typed uses read a
    fresh copy. Elsewhere a container where another instantiation is
    expected (`Array[Integer]` for `Array[untyped]`, or `Array[untyped]`
    or an untyped value for `Array[Integer]`; `Hash` alike, dynamic
    arguments too) is converted: a copy with each element checked, so
    writes through it do not reach the original. `T?` elements are not
    converted.
22. Unannotated literals infer by joining their parts; when parts share no
    type the element type is `untyped`. A 2–3 element mixed array with
    nothing expected of it is a tuple (sort keys, multiple returns). A
    literal nested in one that `untyped` is expected of is untyped-expected
    too, so it is an Array. A tuple that reaches `untyped` is converted to
    an `Array[untyped]` copy, so untyped code sees an Array (`is_a?`, `when
    Array`, `puts`, `==`, dynamic calls); writes to the copy are not seen by
    the tuple, and like any Array it keys a Hash by value (decision 1). A
    tuple still inside a typed container answers as an Array to
    `is_a?`/`when`/`.class` but not to dynamic calls.
    The way back is checked: a dynamic call converts an Array of the right
    size and element types into a tuple parameter, else raises `TypeError`;
    no other untyped value converts to a tuple. *(Revised: tuples used to
    reach untyped as Go structs, so `is_a?(Array)` was false and `puts`
    printed their inspect.)*
23. Symbols are a named Go string distinct from `String`. `f(a: 1)` on a
    method without keyword parameters passes a Hash, as Ruby 3 does;
    keyword parameters themselves are not supported.
24. Regexps are Ruby syntax on Go's RE2. Every pattern gets `(?m)` (Ruby's
    `^`/`$` are line anchors), Ruby `/m` and inline `(?m)` become `(?s)`,
    `\h` is expanded. Onigmo syntax RE2 reads differently is rewritten: `\s`
    includes `\v`; POSIX brackets are Unicode (Go's tables); nested classes
    and `&&` are computed as rune ranges; `{,n}` is `{0,n}`; `X{n}?` is
    `(?:X{n})?`; `\u`/`\e` become `\x{...}`; `\Q` is a literal `Q`; plain
    groups don't capture once one is named. *(Revised: these passed through
    and silently matched RE2's meaning.)* Lookaround, backreferences, `\Z`
    and `/x` are rejected with `file:line` at transpile time. Static patterns compile once into package
    variables; interpolated ones compile at run time and raise
    `RegexpError` (with `/o`, only until one compiles; it is kept). An
    interpolated pattern is translated whole at run time (the compiler's
    translator is emitted into every program), so values get the same
    rewrites, an embedded Regexp's `(?i-mx:...)` composes, and each value is
    evaluated once; its static parts are still checked at transpile time.
    *(Revised: static parts were translated one at a time and values were
    inserted untranslated.)* Matching semantics no rewrite can express are
    handled at match time: Onigmo's `^` never matches at the end after a
    final newline, and its `\b` counts non-ASCII letters as word characters
    (`\w` stays ASCII). A pattern using either also gets a backtracker (Go's
    own algorithm, leftmost-first) with Onigmo's assertions; it runs only
    for subjects where RE2's answer can differ (a non-ASCII word character,
    or an RE2 match ending at a final newline), so other matches stay on
    RE2. Under `/i`, a literal also matches Unicode's multi-character case
    folds (`STRASSE` matches `straße`, `FF` matches `ﬀ`), spelled as
    alternations. As in Onigmo, a fold doesn't span a group or a quantified
    atom; unlike it, nor a one-character class (`/[s]s/i` misses `ß`), and
    a literal is expanded in pieces of about 8 characters, so a fold
    straddling two pieces is missed. *(Revised: these matched RE2's
    meaning.)* `$~`/`$1` are not supported; use `match`.
25. JSON matches the json gem: escapes (quotes, backslash, control
    characters; `/` and non-ASCII as-is) and floats (its `fpconv` rules,
    e.g. `1e+20`, `0.0000123`) are ported.
    `to_json(opts)` and `JSON.generate(obj, opts)` take the generator
    options `indent`, `space`, `space_before`, `object_nl`, `array_nl`,
    `depth`, `script_safe` (`escape_slash`), `ascii_only` and
    `allow_nan`; `sort_keys`, `strict` and `as_json` raise
    `NotImplementedError`, other keys are ignored as the gem ignores
    unknown ones, and `max_nesting` is not checked. `JSON.pretty_generate`
    is `generate` with the gem's `PRETTY_STATE_PROTOTYPE` (two-space
    indent, one value per line) and `JSON.dump` is `generate`. Like the
    gem, the generator calls every value's `to_json` with one (opaque)
    state argument: a user `to_json` declared other than
    `(*untyped) -> String` is reached through its dynamic wrapper, so
    `(?untyped)` gets the state and `()` raises `ArgumentError`.
    *(Revised: options were ignored, and such a `to_json` was skipped
    for the JSON of its `to_s`.)*

    `JSON.parse`/`.load` walk the token stream directly (not
    `json.Unmarshal` into a Go map) so object keys keep insertion order
    in rb2go's `Hash`: an object becomes `Hash[String, untyped]`
    (`Hash[Symbol, untyped]` with `symbolize_names: true`, keys chosen
    once for the whole document), an array `Array[untyped]`, and
    string/true/false/null map to themselves. Numbers are decoded with
    `json.Decoder.UseNumber`, so a literal with no `.`/`e`/`E` is
    Integer and any other is Float, as the gem. A duplicate object key
    keeps its first position and takes the last value, as Ruby Hash
    assignment. An integer literal that does not fit in 64 bits raises
    `RangeError` rather than the gem's Bignum (decision 35: no Bignum),
    matching `String#to_i`'s and a literal's own overflow. Malformed
    input raises `JSON::ParserError`; message text is Go's decode error,
    not ported from the gem's parser. `JSON.load` is `.parse`: the
    gem's docs call `.load` unsafe for arbitrary Ruby objects, but
    rb2go's closed, untyped-only world has no such objects to inject.
26. Threads are goroutines. An exception ends only its thread (reported on
    stderr) and `join` re-raises it. There is no GVL: stdout writes are
    locked, other shared state is the program's problem. The set of
    containers mid-`inspect` (what prints a self-reference as `[...]` or
    `{...}`) is one locked set, not MRI's per-thread one: Go exposes no
    goroutine id, so two threads inspecting one container at once may
    see `[...]`.
27. The `net/http` prelude is WEBrick's and `Net::HTTP`'s API on Go's
    `http.Server` and `http.Client`, keeping what programs can observe:
    no sniffed `Content-Type`, form bodies parsed into `query` only for
    form content types, `mount_proc` refusing methods other than GET, HEAD,
    POST and PUT, relative `Location` made absolute, `HTTPStatus`
    exceptions becoming their status, no redirect following on the client.
    Servers listen in `new` (so `Port: 0` works with `config[:Port]`),
    `start` blocks until `shutdown`, and each request gets its own
    goroutine and servlet instance. *Revised:* `Net::HTTPResponse` dropped
    `@go_type` and became a plain ivar-based class so it can have real
    subclasses (a `@go_type` class has no struct to embed, so subclassing
    one is a compile error, per the `@go_type` note above) — one per MRI's
    `Net::HTTPResponse::CODE_TO_OBJ` (checked against
    `ruby -rnet/http -e 'p Net::HTTPResponse::CODE_TO_OBJ'`), under
    category classes (`HTTPInformation`/`HTTPSuccess`/`HTTPRedirection`/
    `HTTPClientError`/`HTTPServerError`) picked by status digit, falling
    back to `HTTPUnknownResponse`, so `case res when Net::HTTPSuccess` and
    `res.is_a?(Net::HTTPNotFound)` work like MRI's. Request objects
    (`Net::HTTP::Get.new(path, initheader = nil)` and the other five verbs,
    `req["H"] = v`, `#body=`, `#basic_auth`, `#set_form_data`) and
    `http.request(req)` share the same sender as `get`/`post`/etc; header
    names are folded to lowercase everywhere (`Net::HTTPHeader`'s and
    WEBrick's own convention), not Go's `Title-Case` canonical form.
    `use_ssl=` switches the request URL to `https://`; `open_timeout=`/
    `read_timeout=` (Float seconds) wrap `http.Transport.DialContext` and
    `http.Client.Timeout` respectively, raising `Net::OpenTimeout`/
    `Net::ReadTimeout` (`Timeout::Error < RuntimeError`, matching MRI's
    hierarchy) by tagging the dial-phase error to tell it apart from a
    client-wide timeout. *Ponytail: MRI times each response chunk
    separately; this times the whole remaining round trip as one span.*
    `Net::HTTP.get`/`get_response`/`post_form` take a `URI::HTTP`/`HTTPS`
    (decision 63), not a `String` — overloads aren't supported (decision
    12). `Net::HTTP.start(host, port) { |http| ... }` yields but doesn't
    thread the block's return value out. On the server side, `WEBrick::
    HTTPStatus` gained `BadRequest`/`Unauthorized`/`Forbidden`/
    `InternalServerError` and a `Redirect` category
    (`MovedPermanently`/`Found`); `HTTPResponse#set_redirect(status, url)`
    sets body/`Location` then raises `status` like MRI's does, reusing
    `mount`'s `singleton(C).New` type-assertion idiom (decision 19) since
    raising a dynamically-typed class value has no other path yet. A
    raised `HTTPStatus` now keeps whatever body/header the handler already
    set instead of wiping them to a bare 500 — only a non-`HTTPStatus`
    panic still resets to 500, matching WEBrick's own default error page.
    `HTTPRequest#header`/`#each` enumerate headers as a lowercase-keyed
    `Hash`. `HTTPRequest#cookies`/`HTTPResponse#cookies` are a plain
    `WEBrick::Cookie` name/value pair (no path/domain/expires attributes);
    response cookies are written out as repeated `Set-Cookie` headers.
28. Constant reflection: every class object has a generated constant
    table (own constants in definition order, then inherited ones, which
    `inherit = false` skips), behind `Module#constants`, `#const_get`
    (`A::B` paths, top-level fallback, `NameError` on a miss, MRI's
    name checks and errors) and `#const_defined?`. `M.const_get(name)` is
    typed: a literal name gets the constant's type (a literal path through
    a non-module stays untyped and raises at run time), any other name the
    join of the constants of `M` and its subclasses; unrelated classes
    join at their nearest common superclass. MRI orders `constants` by its
    symbol table, not by definition, so programs must not depend on it.
29. `extend M` includes `M` in the class object (a module can
    `extend Enumerable` over its own `self.each`). A named `&block`
    parameter needs a block in the signature; `block.call` is a yield,
    passing it to an iterator re-yields in a range loop, passing it to a
    closure-taking method hands the closure on; storing it is an error.
30. `Struct.new` and `Data.define` declare classes. Member types come from
    rbs-inline's per-member form (`:x, #: Integer`) or a trailing
    `#: [A, B]`. Accessors (readers only for `Data`), `initialize`
    (trailing nilable struct members optional, every `Data` member
    required), keyword `new`, `==`, `eql?`, `hash`, `to_h`, `members`,
    `inspect`, `to_a` and `with` are generated as Ruby and compiled like
    user code. A `do` block's def overrides a generated method and `super`
    reaches it, since MRI defines those on `Struct`/`Data`; the accessors
    are the class's own, so a block def replaces them. `keyword_init` is
    not supported.
31. `method_missing` on a typed receiver: an unknown method compiles to
    `method_missing(:name, *args)`, typed by its signature.
    `respond_to?(:name)` folds to a constant, or asks `respond_to_missing?`.
    `super` with no user-defined parent is Object's: `NoMethodError` for
    the name, or `false`.
32. Dynamic dispatch: a method called on an `untyped` value, an unknown
    method on a `Module`-typed class object, or a method only subclasses
    define compiles to `rbDynName(recv, args...)`. Each such name gets a
    `DynName(args ...any) any` wrapper on every class with a public,
    non-generic, block-less method of that name: MRI's `ArgumentError` for
    arity, `TypeError` for argument types, then the typed call. Without a
    wrapper the call goes to `method_missing`, then `NoMethodError` (or
    `NameError` for a bare name); `===` is every object's, so without a
    wrapper it is `==` (a class object held `untyped` gets `==`, not
    `Module#===`'s `is_a?`). A private method's wrapper is `_DynName`,
    which only `send` and receiver-less calls reach, as in MRI; a call with
    a receiver or `public_send` goes on to `method_missing`, then
    `NoMethodError` saying "private method". `respond_to?(name, true)`
    counts private methods. `send`/`public_send` with a literal name
    are ordinary calls; a computed name switches over every method name,
    private ones included, and makes the output larger, so it is generated
    only when used. *(Revised: the tables used to hold public methods only,
    so `send` could not reach a private method dynamically,
    `respond_to?(name, true)` ignored its flag, and the error said
    "undefined method".)* On
    generic classes, methods whose signatures nest the type parameters in
    another type get no wrapper: wrapping them makes Go instantiation
    cycles. Blocks cannot cross a dynamic call. A value typed `Object` or
    a module is Go `any` too, so it is dispatched, boxed and `is_a?`-tested
    like `untyped`; a method the module declares keeps its declared result
    type. *(Revised: they used to resolve like concrete classes, so
    `Kernel#to_s` printed `#<String>`, `is_a?` folded to false, literals
    reached `any` as Go `string`, and module methods failed `go build`.)*
    *Revised:* `<=>` differs twice. Its wrappers are always generated:
    `sort`/`min`/`max`/`sort_by` on untyped values reach them through
    `rbCmp` inside generic prelude code, where the compiler cannot see
    the instantiation. A wrong argument type answers `nil`, as MRI's `<=>`
    does, and `rbCmp` turns `nil` into MRI's `ArgumentError: comparison
    of X with Y failed` (which pair MRI names depends on its sort order).
    `hash` on an untyped value calls the class's `hash`, else hashes as a
    Hash key does (decision 1): by value where Go `==` is `eql?`, by
    identity for plain objects.
    *Revised:* the default `inspect` is MRI's (`#<Foo:0x… @a=1>`, with the
    address and the ivars, never calling `to_s`). Ivars list in
    `initialize`'s assignment order; a non-optional ivar still nil was never
    assigned and is left out, as MRI does, but an unassigned Integer shows
    as `0`. An ivar-less class gets a padding byte, since Go gives every
    zero-size object the same address and `equal?` needs identity.
33. Ruby semantics for looser code: `expr rescue fallback`; `return`,
    `break` or `next` in `ensure` discards the pending exception;
    `&&`/`||` return values of any types (unions become `untyped`) and
    evaluate the right side only when Ruby would; locals first assigned
    in a branch or `begin` body are visible after it (Ruby scopes are
    methods and blocks); `rescue` and `ensure` are generated after the
    body; `untyped` in a `bool` position is truthiness.
34. `Hash#inspect` prints symbol keys as labels (`{a: 1, "a b": 2}`), as
    Ruby 3.4 does.
35. Integer is a Go `int`: **decided, 64 bits, no Bignum or Rational.**
    Native types are Go primitives; a Bignum-capable Integer would box
    every value or branch every `%x{}` that treats it as an `int`. Where
    MRI would promote, rb2go raises `RangeError` instead of wrapping:
    `+`, `-`, `*`, `/`, `-@`, `abs` and `**` check for overflow (`*` only
    when an operand is past 32 bits, and all but `/` and `**` still
    inline), a Float past ±2**63 converts with MRI's `float … out of
    range of integer`, `String#to_i` raises rather than saturating, and a
    literal past 64 bits is a compile error. A negative exponent is a
    Rational in MRI, so `**` raises too, except for bases 0
    (`ZeroDivisionError`) and ±1 (MRI answers an Integer). Crystal makes
    the same trade. A Bignum would need Integer as a small-int/`*big.Int`
    union behind a decision of its own.
36. Return types can be inferred. A def with no `#:` takes rbs-inline's
    per-parameter form (`# @rbs x: T`, `# @rbs *xs: T`, optional
    `# @rbs return: T`); a def with no parameters needs no annotation at
    all. Without a return annotation the return type is the join of what
    the body returns (tail values and `return`s; `nil` + `T` is `T?`),
    found by a dry run of its generation when a caller first needs it,
    and again for every method after ivar discovery, whose types it
    depends on. Returns with no common type, recursion (direct or
    mutual) and blocks still need an annotation. An unannotated override
    of an inferred method takes the parent's inferred type. Parameter
    types are never inferred: that would make a method's type depend on
    its callers ([example 34](examples/34_inferred_returns/main.rb)).
37. `Range[E]` is a struct `{b, e E; excl, endless bool}` handled as a
    pointer; `a..b`, `a...b` and `a..` build it inline (generic classes
    have no class methods, so there is no `Range.new`). Beginless ranges
    are a compile error. Iteration (`each`, `step`, `to_a`, Enumerable)
    needs `Integer` or `String` (MRI's `String#succ`, without its
    all-digits mode); other `E` only compare (`cover?`, `include?`, `===`
    in `case/when`). `sum` and `size` are arithmetic, and `size` of an
    endless range raises (no Infinity), as does `last`; its `end` is
    `E`'s zero value, not nil
    ([example 36](examples/36_ranges/main.rb)).
38. `Kernel#p` prints each argument's `inspect` and returns nil, not its
    argument (that would need the argument's type as its return type).
39. `Time` wraps Go's `time.Time` plus MRI's UTC flag (printed `UTC`
    rather than `+0000`), handled as a pointer so `utc`/`localtime`
    convert in place as MRI's do. `Time.at` takes Integer or Float
    seconds, `Time.utc`/`gm`/`local`/`mktime` take integer parts only
    (no month names, no zone argument; `Time.new` with none is `now`).
    `t - t2` is a Float, `t ± n` a Time (via decision 12's overloads).
    `strftime` is MRI's, flags and widths included; `iso8601`/`xmlschema`
    are core, as in Ruby 3.4. No `Time#to_a`: 10-tuples do not exist
    ([example 37](examples/37_time/main.rb)). *Revised:* `Time.new`,
    `Time.at` and `Time.now` take a zone: `Time.new`'s 7th positional arg,
    or `in:` on any of the three, which (decision 23: no real keyword
    params) arrives as a trailing `{in: ...}` Hash; `localtime`/`getlocal`
    take the same zone as their one optional arg. A zone is an Integer
    offset in seconds, or a String — `"UTC"` (case-insensitive), `"Z"`,
    a single military letter (`A`.."I","K".."Z"`, `J` unused), or
    `"±HH[:MM[:SS]]"`/`"±HHMM[SS]"` (`"-00:00"`/`"-0000"` is UTC, MRI's
    quirk for an explicitly-unknown offset; `"+00:00"` is a real, merely
    zero, offset). An offset outside `(-86400, 86400)` or an unparseable
    String raises MRI's `ArgumentError`, with its exact message; any other
    argument class is silently treated as "no zone" (`Time.at`'s unused
    subsec argument, say), not MRI's `TypeError`. A zone is always a fixed
    UTC offset, via Go's `time.FixedZone`: there is no IANA tz database
    lookup (`Time.new(..., in: "Asia/Tokyo")` does not work), since that
    needs `time/tzdata`, an opt-in embed MRI-style named zones don't
    otherwise call for. `round`/`floor`/`ceil` round the fractional second
    to `digits` decimal places (default 0), ties toward +infinity as
    MRI's Time does (`Float#round`'s ties go away from zero instead);
    negative `digits` raises `ArgumentError`. `dst?`/`isdst` is
    `time.Time.IsDST()`, always `false` for a `FixedZone` or `UTC` time,
    as MRI's is. `ctime`/`asctime` format with `time.ANSIC`. `tv_sec`,
    `tv_usec`, `tv_nsec` alias `to_i`, `usec`, `nsec`
    ([testdata/run/time_mid.rb](testdata/run/time_mid.rb)).
40. `Rational` is a pointer to a `math/big.Rat`, so arithmetic is exact
    and never overflows; `numerator`/`denominator`/`to_i`/`round` raise
    RangeError past 64 bits (decision 35). Literals (`3r`, `3/4r`,
    `0.75r`), `Rational(n, d)` (Integers only, no strings), `Integer#to_r`,
    `Integer#quo` and exact `Float#to_r` build one. Integer and Rational
    mix into a Rational and Float and Rational into a Float, in either
    order, through decision 12's overloads; `r < 1` works the same way.
    Integer `**` with a negative exponent still raises (its type is
    Integer). `Float#round(n)` takes `n > 0` only: MRI answers an Integer
    for `n <= 0` ([example 38](examples/38_rational/main.rb)).
41. `Date` is a Julian Day Number on the proleptic Gregorian calendar
    (MRI switches to the Julian calendar before 1582-10-15; rb2go does
    not). It is always defined, `require "date"` or not. `Date.parse`
    reads ISO dates, `y/m/d`, `Mar 5, 2024` and `5 March 2024`, not all
    of MRI's heuristics; `strptime` reads date fields only. `d - d2` is a
    Rational, `d ± n` a Date, `>>`/`<<` clamp to the month's end, and a
    Range of Dates iterates, since Range iterates anything with `succ`.
    `strftime`'s `%Z` prints `UTC` where MRI's Date prints `+00:00`.
    `httpdate`/`rfc3339` format (and `Date.httpdate`/`rfc3339` parse
    through `Date.parse`, which already reads both shapes) at midnight
    UTC, since a Date has no time of day. `jisx0301` prints MRI's Japanese
    era code (`M`/`T`/`S`/`H`/`R` + 2-digit era year) for a date on or
    after Meiji 6 (1873-01-01, when Japan adopted the Gregorian calendar;
    MRI's own table has no era code before it), plain ISO otherwise;
    `Date.jisx0301` parses either shape back
    ([example 39](examples/39_date/main.rb)).
42. `Complex` is a pointer to `{re, im any}`: each part keeps its class
    (Integer, Rational or Float) and arithmetic follows MRI's rules for
    mixing them (`Complex(1, 2) / Complex(3, 4)` is exact, `Complex * real`
    scales part by part, Integer quotients that are whole come back as
    Integers). `real`, `imaginary`, `abs` and `abs2` are `untyped`, since
    their class depends on the parts. `3i`, `Complex(a, b)`,
    `Complex.polar`/`rectangular` build one; mixing with Integer, Float
    and Rational goes through decision 12's overloads. `**` takes an
    Integer (exact) or a Float (polar form)
    ([example 40](examples/40_complex_math/main.rb)).
43. `Math` evaluates every transcendental function in 128-bit
    `math/big` and rounds once, so results are correctly rounded. Go's
    `math` is only within 1 ulp (and its `Sin`/`Cos` lose digits near
    multiples of π/2), which diverged from MRI's printed output on a few
    percent of inputs. MRI inherits the platform libm, which on macOS
    misrounds `tan`, `sin`, `cos`, `cbrt`, `hypot` and `atan` on a few
    percent of inputs; there rb2go is right and MRI is off by one digit.
    `Math.sqrt` stays Go's (IEEE-exact). A call costs microseconds rather
    than nanoseconds. `Float::INFINITY`/`NAN`/`EPSILON`/`MAX`/`MIN`/`DIG`
    exist. `Float#*` converts its result explicitly, since Go may fuse
    `a * b + c` into one FMA rounding where MRI rounds twice.
44. `Set[E]` (core in Ruby 4) wraps a `Hash[E, Boolean]`, so it is
    insertion-ordered and matches elements by `eql?`/`hash`; it prints
    as Ruby 4's `Set[1, 2]`. `Set.new(array)` and `Set[a, b]` infer `E`;
    an empty `Set.new` needs an annotation (`#: Set[Integer]`), as
    `Hash.new` does. A generic `@go_type` class may now define class
    methods whose signatures use only their own type parameters (`[X]
    (Array[X]) -> Set[X]`); generic struct classes still may not.
    There is no `Array#to_set`: Go rejects the instantiation cycle
    `Array[E]` → `Set[E]` → `Hash[E, …]` → `Array[[E, …]]` (see decision
    9); `Set.new(xs)` is the spelling.
    `Set.new` also takes a `Range`, another `Set` or a `Hash` (as
    `[key, value]` pairs), not just an `Array`: decision 12's overload
    mechanism (`__new_<class>`, keyed on the first argument's static
    class) picks a concretely-typed sibling per source class, so `E` is
    still inferred with no annotation, same as the plain-`Array` case;
    this sidesteps the instantiation cycle above since each sibling is
    its own free function, not a forwarder on the source's own class. An
    untyped or otherwise-unmatched argument falls back to a duck-typed
    `self.new` that switches on the source's `_to_any` at run time (every
    Array/Range/Set/Hash has one) and needs an annotation, since `E` can't
    be inferred from `untyped`. `Set.new(xs) { |o| ... }` transforms each
    element (decision 12's block-overload, `__new_block`); its block
    param is `untyped` (see the `ponytail` note on `__new_block`), so `E`
    comes from the block's inferred return type or an annotation, not
    from `xs`. `Set#map!`/`select!`/`reject!` mutate in place
    (`select!`/`reject!` return `nil` on no change, as `Array`'s do);
    `classify` groups into a `Hash[U, Set[E]]`, `divide` into a
    `Set[Set[E]]` (`classify(&block).values`)
    ([example 41](examples/41_set/main.rb)).
45. `Mutex`, `ConditionVariable`, `Queue[E]` and `SizedQueue[E]` are Go
    `sync` primitives. `Queue#pop` is `E?`: nil once the queue is closed
    and drained, so `while (job = q.pop)` is the consumer loop (an
    assignment in an `if`/`while` condition now narrows its local, as a
    read does). `Queue.new` needs an annotation; `SizedQueue.new(n)` takes
    its element type from one (`#: SizedQueue[String]`): a call whose type
    parameters its arguments leave open unifies its return type with the
    expected type. Relocking a Mutex from its owner deadlocks rather than
    raising, `owned?` is missing (goroutines have no identity), and a
    deadlock is Go's "all goroutines are asleep" rather than MRI's fatal
    error. An iterator called with `&:name` is `{ |x| x.name }`
    ([example 42](examples/42_queues/main.rb)). *(Revised: `Thread.new(1, 2)
    { |a, b| ... }` forwards up to 3 constructor args into the block, by
    decision 12's arity overloads (`__new_1`/`__new_2`/`__new_3`).
    `join(timeout)` is `nil` on timeout, the Thread on success; `value`
    blocks and re-raises like `join`, but the block stays `-> void` (an
    arg-forwarding overload's block can end on a void call, and this
    compiler cannot coerce that to a value type), so `value` is always
    `nil` rather than the block's actual return. `name`/`name=` read and
    write an `atomic.Pointer[String]`. `status` is `"run"`, `"aborting"`
    while unwinding a panic, `false` once finished normally, or `nil` once
    finished with an unhandled exception; MRI's `"sleep"` is never
    reported, the same "goroutines have no identity" limit as `owned?`.
    `ConditionVariable#wait(mutex, timeout)` is `0` once signalled, `nil`
    on timeout, via a `time.AfterFunc` alongside the existing per-waiter
    channel. `Queue.new(array)` pre-fills from an Array. `SizedQueue#max=`
    resizes a running queue and wakes blocked pushers.
    `Queue#pop(timeout: n)`/`SizedQueue#pop(timeout: n)` (also `shift`,
    `deq`) take the timeout as a Hash (decision 23), routed by decision
    12's class-based overload (`__pop_hash`); a real `num_waiting` counts
    goroutines currently parked in a blocking `push` or `pop`, via an
    `atomic.Int64` incremented before `cond.Wait()` and decremented after.
    `Thread.current`, thread-locals and `Monitor`/`MonitorMixin` remain
    unsupported for the same reason as `owned?`.)*
46. `Random` is MRI's MT19937 seeded as MRI seeds it (one 32-bit word
    through `init_genrand`, more through `init_by_array`), and `rand(n)`,
    `rand`, `rand(a..b)`, `rand(Float)`, `bytes`, `Array#shuffle`/`shuffle!`
    and `Array#sample` draw exactly as MRI does, so a seeded program prints
    the same numbers. Seeds are 64-bit (MRI's `new_seed` is 128); an
    unseeded generator is seeded from `crypto/rand`. `Kernel#rand`/`srand`
    use one shared generator; `rand(0)` raises (MRI returns a Float) and
    Float ranges are not supported. `random:` is passed as a Hash
    (decision 23). Also `Array.new(n, v)`, `Array.new(n) { |i| … }` and
    `String#bytes` ([example 43](examples/43_random/main.rb)).
47. Procs are typed by RBS proc types (`^(Integer) -> Integer`) and are a
    Go `*func(Integer) Integer`: a pointer, so they satisfy `comparable`
    (decision 10) and can sit in Arrays and Hashes, and `==` is identity
    as for MRI's distinct procs. `->(x) { }`, `lambda { |x| }` and
    `proc { |x| }` build one; a lambda with parameters takes its types
    from the expected type (`#:` on the local, the parameter or the
    ivar), and one without infers its return type. `call`, `.()`, `[]`,
    `yield`, `===`, `arity`, `>>`/`<<` (composition), `&f` (as a block,
    iterators included), `is_a?(Proc)` and `.class` work. Every Proc
    behaves as a lambda: `return` leaves only the Proc, arity is strict,
    and `lambda?` is true. `next v` now works in any block, giving the
    block's value. A Proc held `untyped` cannot be called dynamically
    ([example 44](examples/44_procs/main.rb)).
48. `StringIO` is a byte buffer with a position: `write`/`<<`/`print`/
    `puts` (Kernel#puts's rules) overwrite at `pos` and extend, and
    `read`, `read(n)` (nil at EOF), `gets`, `getc`, `getbyte`, `each_byte`,
    `each_char`, `each_line`, `readlines`, `rewind`, `pos=`, `seek`
    (`IO::SEEK_SET`/`CUR`/`END`), `tell` (a `pos` alias), `eof?` and
    `lineno` read, as MRI's. It is always defined, `require "stringio"`
    or not; there is no shared IO base class, so a method taking
    `StringIO` does not take `$stdout`, though `close`/`closed?` and
    `rbReadable()`/`not opened for reading`/`not opened for writing`
    follow the same naming as `File`'s (decision 62) for consistency.
    `StringIO.new(str, mode)` takes File's `r`/`r+`/`w`/`w+`/`a`/`a+`
    (an unknown mode raises MRI's `ArgumentError`); the default (no mode
    given) is a duplex, non-truncating `r+`. `w`/`w+` truncate the string;
    `a`/`a+` always write at the buffer's end regardless of `pos`, like an
    `os.O_APPEND` file. `close` blocks both read and write (unlike
    `File`, there is no distinct "closed stream" message — reading or
    writing a fully closed `StringIO` raises the same `not opened for
    reading`/`writing` `IOError` as a stream never opened that way); only
    `seek` raises `IOError, "closed stream"` on a fully closed stream.
    `close_write` closes the write side only, raising `IOError, "closing
    non-duplex IO for writing"` if the stream was never writable;
    `truncate(len)` (MRI: zero-pads on growth, raises `Errno::EINVAL` on a
    negative length) needs the write side open. `ungetc(c)` (a `String`
    or `Integer` codepoint) overwrites the last-read character's bytes
    with `c` and rewinds `pos` before it (at `pos` 0 it prepends), matching
    MRI's splice rather than a real pushback buffer.
    `Enumerable#sum` is MRI's: Integers and Rationals add exactly, and
    once a Float appears the rest adds with Kahan-Babuska compensation
    (`[0.1, 0.2, 0.3].sum` is `0.6`); `sum { |x| … }` goes through
    decision 12's overloads, which now also route a call with a block
    to `__<name>_block` when the method takes none
    ([example 45](examples/45_stringio/main.rb),
    [testdata/run/stringio_mid.rb](testdata/run/stringio_mid.rb)).
49. Generated Go is pruned like a linker would: from `main`, `init` and
    `_` vars, a function, type or var is kept when an identifier names it,
    and a method when its receiver type is kept and its name is selected
    somewhere, declared by a kept interface, or one the standard library
    calls (`String`, `Error`, …). Names match without scoping, so it only
    ever keeps too much; `iota` const blocks stay whole. A program then
    compiles (and lints) only the prelude it reaches: `puts 1` is 7k
    lines instead of 18.5k. `RB2GO_NO_PRUNE=1` turns it off.
    *Amended:* a method whose name a kept interface declares but no kept
    code selects is kept as a stub, its body `panic("rb2go: pruned")` and
    not visited. Go calls a method only through a selector (the standard
    library's calls are the `stdMethodNames` list), so a stub can't run,
    but its type still satisfies the interface. Before, an interface
    declaration kept the whole body, and since every metaclass interface
    declares every `Module` method, constant reflection was always
    reachable and its tables named every class: `puts 1` had grown to 35k
    lines. It is 4k now, and a minitest program 15k. Code that must not
    reach every program keeps its costly paths behind unique names and
    out of generated interfaces: class-object tables (`_Consts`,
    `_Methods`, `_IsInstance`) are asked for through interfaces declared
    at their use (`rbConstTable`, `rbInstanceTest`), never by `ModuleI`.
    *Amended:* a type-switch case of one concrete type names that type
    weakly: if nothing else keeps it, no value of it can exist, so the case
    (body and all) is dropped instead of keeping the type. That lets the
    generated switches over every box and Proc type (`rbUnbox`,
    `rbKeyUnbox`, `rbCmpBox`, `rbIsProc`) list everything the compiler saw
    and still cost a program only the types it uses. Interface cases stay
    strong (values implement an interface without naming it), and so do
    cases listing several types (narrowing them would change the case
    variable's type). `puts 1` is 2.7k lines.

50. Stdlib libraries with a Go-stdlib twin are always defined, `require`
    or not, like decision 48: `Base64` (`encode64` wraps at 60 columns,
    `decode64` skips non-alphabet bytes, `strict_decode64` raises
    `ArgumentError`), `Digest::MD5/SHA1/SHA256/SHA384/SHA512` (class
    `digest`/`hexdigest`/`base64digest`; `new` returns a `Digest::Base`
    that buffers `update`/`<<` and sums on demand), `Zlib.crc32`/`adler32`
    (resumable from a prior value), `SecureRandom` on `crypto/rand`, and
    `cgi/escape`'s `CGI.escape`/`unescape`/`escapeHTML`/`unescapeHTML`/
    `escapeURIComponent`, decoding only the entities MRI does. Keyword
    options arrive as a Hash (decision 23): `urlsafe_encode64(s,
    padding: false)`. `String#ljust`/`rjust`/`center` take a pad string;
    `String#delete` takes one character set. `Digest::X.file(path)` reads
    the whole file (via `File.read`, decision 62) and digests it, raising
    `File.read`'s `Errno::*` on a bad path; `Digest::Base#==` recomputes
    and compares each side's current digest bytes (two untouched digests
    of the same algorithm are equal).
    `Digest::SHA2.new(bitlen)` picks SHA256/384/512 (default 256),
    `ArgumentError` on any other length. `Digest.hexencode(str)` is a
    standalone module function (raw bytes → hex), not on `Digest::Base`.
    `SecureRandom.uuid_v4` is `uuid` under another name (it was already
    v4); `uuid_v7` is RFC 9562: a 48-bit unix-ms timestamp, then a version
    nibble (`7`), 12 random bits, the variant bits (`10`), then 62 more
    random bits — time-ordered, unlike v4. `SecureRandom.alphanumeric`
    takes a real MRI keyword, `chars:` (an `Array[String]`, checked
    against `ruby -rsecurerandom -e 'p SecureRandom.method(:alphanumeric).parameters'`
    before assuming it existed): picking `n` elements from it, joined:
    the pool need not be single characters. Passing `chars:` alone,
    skipping the positional `n` default, is not supported (`n` must be
    given); MRI itself hangs on `chars: []`, so that case is untested
    either side ([example 46](examples/46_digests/main.rb)).
    *Amended:* `Zlib::Deflate.deflate`/`Zlib::Inflate.inflate`
    and the `Zlib.deflate`/`inflate` convenience wrap `compress/zlib`;
    `Zlib.gzip`/`gunzip` wrap `compress/gzip`. Go's `compress/flate` (both
    packages sit on it) does not emit byte-identical output to MRI's C
    zlib at the same level — different implementations, valid but
    different compressed bytes for the same input and level, and gzip
    headers differ too (mtime, OS byte). So compressed output can never
    be compared to MRI's byte-for-byte; what's checked instead is
    round-tripping (compress then decompress returns the input) and
    inflating a literal MRI-produced deflate/gzip blob embedded in the
    test, which does check this reads real zlib's output correctly, the
    direction that matters for files written by real Ruby elsewhere
    ([testdata/run/zlib_mid.rb](testdata/run/zlib_mid.rb)). `DEFAULT_COMPRESSION`
    (`-1`), `NO_COMPRESSION` (`0`), `BEST_SPEED` (`1`) and
    `BEST_COMPRESSION` (`9`) match MRI's values. `Zlib::GzipWriter`/
    `GzipReader` dispatch `write`/`read`/`close` dynamically on whatever
    they wrap (decision 32), so any object with those Ruby methods works
    — not just `File` — without a shared IO interface to require
    statically (decision 48 already declined one for `StringIO`).
    `GzipWriter` buffers the whole compressed stream in memory; `write`
    feeds the deflate stream, and `finish`/`close` make one `write` call
    with the finished bytes (`finish` leaves the wrapped object open, so
    a `StringIO`'s `#string` still reads afterwards; `close` then also
    calls the wrapped object's `close`, so it does not work over
    `StringIO`, which has none, decision 48). `GzipReader.new` makes one
    `read` call to slurp the compressed bytes and decompresses eagerly,
    then serves `read`/`gets`/`each_line` from the plain buffer like
    `StringIO`. A malformed zlib stream raises `Zlib::DataError`; a bad
    gzip header or corrupt body raises `Zlib::GzipFile::Error` — a
    module here, not MRI's class, since nothing subclasses it.
51. `StringScanner` keeps a byte pointer and matches each Regexp against
    the rest of the string, so `^`/`\A` match at the pointer (MRI's default
    `fixed_anchor: false`). `scan`/`skip`/`check`/`match?` use a cached
    `\A(?:…)` twin of the pattern, so a failed attempt costs one try at
    the pointer, not a search; the `_until` forms and `exist?` search.
    `[]` takes an Integer, and only Regexp patterns are accepted;
    `named_captures` (a `Hash[String, String?]` from the pattern's
    `SubexpNames`) and `values_at(*is)` (`is.map { |i| self[i] }`) round
    out group access. `<<`/`concat` append to the scan buffer without
    touching the pointer or the last match. `scan_full`/`search_full`
    take MRI's `(pattern, advance_pointer_p, return_string_p)`: anchored
    or searching, same as `scan`/`scan_until`, with the pointer move and
    the string-vs-length result each gated by their own flag, so all four
    combinations of the two booleans are real call shapes, not just
    `scan`/`scan_until` with extra steps. `Shellwords` is MRI's scan loop
    unrolled by hand (RE2 has no `\G` or atomic groups), with the same
    `Unmatched quote at N: …`
    errors. `Kernel#tap` joins `then`
    ([example 47](examples/47_strscan/main.rb)).
52. `require "time"`'s parsers are always defined. `Time.iso8601`/
    `xmlschema`, `httpdate`, `rfc2822` and `parse` each try a fixed list
    of Go layouts, not `Date._parse`'s heuristics, so `Time.parse` reads
    ISO, RFC 2822/1123, `asctime` and a few `Mon D YYYY` shapes. A string
    without a zone is local time; a `Z`, `UTC` or `GMT` zone gives MRI's
    UTC mode, any other offset a fixed zone. `Time.strptime` rewrites the
    format into a Go layout, so literal text that spells a Go layout
    token is misread, and a directive without a Go twin fails to parse.
    `Time#httpdate`/`rfc2822` format, and `Benchmark.realtime` times a
    block ([example 48](examples/48_time_parse/main.rb)). `Benchmark.measure`
    and `Tms` (`utime`/`stime`/`cutime`/`cstime`/`real`/`total`/`label`,
    `+`/`-`/`*`/`/`, `format`/`to_s`/`to_a`) read CPU time from
    `syscall.Getrusage` (`RUSAGE_SELF`/`RUSAGE_CHILDREN`), microsecond
    granularity, coarser than MRI's. `Benchmark.bm`/`bmbm` print MRI's exact
    `CAPTION`/`FORMAT`; `bmbm` runs a rehearsal pass, `GC.start`s between
    reports in the real pass as MRI's, but Go's GC is not MRI's generational
    one, so the pass evens out allocation load only approximately. Storing a
    named `&block` parameter is a compile error (decision 29), so `bmbm`'s
    `Job` (the object its block registers reports on) keeps its deferred
    blocks in a raw Go slice behind `%x{}`, not as Ruby-visible Procs; `bm`'s
    `Report` needs no such escape, since it measures each report immediately.
53. `CSV` covers strings and files: `parse`/`parse_line`/`String#parse_csv`
    return `Array[Array[String?]]` (an empty unquoted field is nil, a
    quoted one `""`, a blank line `[]`), and `generate_line`/
    `Array#to_csv`/`generate { |csv| csv << row }` quote a field holding
    the separator, the quote or a line break, and an empty String.
    *Revised:* `read`, `foreach(path) { |row| }` (an Array of rows without
    a block, decision 58's Enumerator stand-in) and
    `open(path, mode = "r") { |csv| csv << row }` (writing only; `<<` on a
    file opened `"r"` raises `IOError`, matching `File#write`) go through
    the real `File` class (decision 62); `foreach`/`open` read the whole
    file rather than streaming row-by-row, since the parser already works
    on a full string. Options are a `Hash[Symbol, untyped]` (not
    `Hash[Symbol, String]`, so booleans can mix with strings): `col_sep`,
    `quote_char`, `row_sep` (a literal separator; MRI's `:auto` LF/CRLF
    sniffing is kept only as the *default* when `row_sep` is omitted, not
    as an explicit value a caller can pass — a `Symbol` there doesn't
    typecheck against `String`, and detecting bare `\r` on top of the
    existing LF/CRLF sniff wasn't worth it for how rarely classic-Mac line
    endings show up), `skip_blanks` (drops empty lines, not rows of empty
    fields), `force_quotes` (quotes every field, and turns a nil field
    into `""` instead of empty). `headers:` and `converters:` remain
    unsupported: `headers: true`'s result (a `CSV::Table`/`CSV::Row`) and
    a `converters:` result's type both need compiler-level literal-keyword
    dispatch (extending decision 12's overloads) or an `untyped` result,
    which is undecided, so both keys are simply not recognized (ignored,
    like any other unknown key in the options Hash) rather than raising —
    tracked in [issue #1](https://github.com/jtarchie/ruby2go/issues/1).
    Errors are MRI's `CSV::MalformedCSVError` messages. `Array#index`/`find_index`
    (value or block) and `rindex` join Array
    ([example 49](examples/49_csv/main.rb), [testdata/run/csv_mid.rb](testdata/run/csv_mid.rb)).
54. `include Singleton` in a class makes the compiler add
    `def self.instance = @__singleton_instance ||= new` (typed as the
    class); `Singleton` itself is an empty marker module, so `is_a?`
    works. `new` stays public, and the memo is not thread-safe
    (MRI's is): two threads racing the first `instance` may build two.
55. `format`/`sprintf`/`printf`/`String#%` follow MRI's directives: flags
    `-+ 0#`, width and precision (also `*`), `%<name>…`/`%{name}` from a
    Hash, and `%d %i %u %f %e %E %g %G %x %X %o %b %B %s %p %c %%`, each
    argument converted as MRI does (a Float for `%d` truncates, a String
    is parsed, nil raises `TypeError`). `%g` gets C's default precision 6.
    Not supported: `%a`, and MRI's two's-complement `..f` for negative
    `%x`/`%o`/`%b` (rb2go prints `-ff`).
56. `sub`/`gsub`/`split` take a Regexp and `scan` exists, through decision
    12's overloads, now keyed on the *first* argument's class when the
    overload takes the call's argument count (`gsub(/re/, "x")` →
    `__gsub_regexp`). Replacements expand `\0 \& \1-\9 \k<name> \` \'
    \\`; the block forms (`gsub(/re/) { |m| … }`) take a Regexp or a
    String. `scan` returns `Array[String]`, or `Array[Array[String?]]`
    when its argument is a regexp literal with groups (the compiler picks
    `__scan_groups`); a non-literal Regexp with groups raises
    `NotImplementedError`. `split(/re/)` keeps captured groups and splits
    between characters on an empty match. Matches come from RE2's
    `FindAll`, so rxRegexp's Onigmo corrections (decision 24) do not
    apply to these.
57. Index assignments `h[k] ||= v` and `a[i] op= v` evaluate the receiver
    and index once, then call `[]` and `[]=` as Ruby does (`||=` writes
    only when `[]` gave nil or false). A block taking several params over
    an `Array[T]` element splats it, as Ruby does for any yielded Array:
    each param is `T?`, nil past the end (tuples keep their exact
    element types) ([example 50](examples/50_format_regexp/main.rb)).
58. Core gaps filled in the prelude, each checked against MRI in
    `testdata/run/*_mid.rb`: Enumerable (`each_slice`, `each_cons`,
    `each_with_object`, `filter_map`, `partition`, `minmax`, `min(n)`,
    `max(n)`, `sort { }`, `count(x)`/`count { }`, `inject`/`reduce`
    without an initial value, `take_while`, `drop_while`, `drop`, `zip`
    with one Array, `chunk_while`, `slice_when`, `combination`,
    `permutation`, `values_at`); Array (`-`, `&`, `|` by eql?/hash,
    `rotate`, `product`, `bsearch`, `insert`, `fill`, and the mutating
    `sort!`, `sort_by!`, `map!`, `select!`, `reject!`, `keep_if`,
    `delete_if`, `uniq!`, `reverse!`); Hash (`transform_values`/`keys`,
    `to_h { }`, `invert`, `key`, `value?`, `values_at`, `fetch_values`,
    `slice`, `except`, `store`, `update`/`merge!`, `merge { }`,
    `delete_if`/`keep_if` and bang forms, `any?`/`all?`/`none?`/`count`
    over `|k, v|`); Integer (`gcd`, `lcm`, `pow(e, m)`, `digits`, `fdiv`,
    `divmod`, `div`, `remainder`, `bit_length`, `to_s(base)`,
    `Integer.sqrt`, `step`); Float (`floor(n)`/`ceil(n)` as MRI's
    rb_float_floor, `truncate`, `%`, `divmod`, `finite?`, `infinite?`);
    String (`to_i(base)`, `hex`, `oct`, `succ`, `count`, `squeeze`,
    `swapcase`, `casecmp(?)`, `delete_prefix`/`suffix`, `partition`,
    `rpartition`, `chop`, `chr`, `ascii_only?`, `split(sep, limit)`,
    `start_with?`/`end_with?` with up to three candidates).
    A blockless call to a block-taking method goes to `__<name>_enum`
    when defined: an Array standing in for the Enumerator
    (`each_slice(2).to_a`, `each_with_index.map`, `3.times.map`), so
    chaining works but `puts`/`p` of it print elements, not
    `#<Enumerator…>`. Blockless `map`/`select`/`filter`/`reject` return
    `Enumerator::Map`/`Select`, whose `with_index` maps or filters as MRI's
    and whose inspect is MRI's. Block params may destructure one level
    (`|(k, v), i|`). Go forbids a method of `Array[E]` from building an
    `Array[Array[E]]`, `Array[E?]` or `Hash[E, …]` (an instantiation
    cycle), so such methods live in Enumerable or Go helpers.
    String stays immutable (see "Frozen strings"): no `<<`, `insert`,
    `prepend`.
59. The CLI is `rb2go build [-o prog] main.rb` and `rb2go run main.rb
    [args...]`, shaped like `go build`/`go run`; there is no emit-Go-only
    mode. Each call writes `main.go` and a `go.mod` (`go` directive
    `rb2go.GoVersion`, shared with the tests) into a fresh temp module and
    runs `go build -trimpath` there with `GOWORK=off` and an empty
    `GOFLAGS`, so the user's workspace can't leak in. Generated code is
    stdlib-only, so the build never touches the network, and Go's
    content-keyed build cache makes a warm `run` about 0.8s, mostly
    transpiling; there is no rb2go-level cache. `-work` keeps the module
    and prints its path (the way to read the generated Go); `-race` and
    `-gcflags` pass through. `run` executes the binary in the caller's
    directory, as `ruby main.rb` does, with stdio inherited, and exits
    with its status; transpile and build failures exit 1, bad usage 2.
60. Output and signals follow MRI where it shows at a terminal. stdout is
    buffered, but when it is a character device every write flushes, as
    MRI's does on a tty (`print "a"` shows before a later `$stderr` write;
    on a pipe both keep MRI's buffered order). SIGINT and SIGTERM flush
    stdout, then the program re-raises the signal with the default action,
    so it dies by it (130/143 in a shell) like MRI. Unlike MRI, no
    `Interrupt`/`SignalException` is raised: `rescue` and `ensure` don't
    run, because Go can't inject a panic into the main goroutine. A
    program started with SIGINT ignored (a non-interactive shell's `&`
    job) keeps ignoring it, where MRI would still take it. `rb2go run`
    catches both signals so it outlives the child, forwards them, removes
    its temp module, and exits 128+signal when the child died by one.
61. The process boundary: `ARGV : Array[String]` (built from `os.Args[1:]`),
    `ENV` (an `ENVClass`, Hash-like as MRI's ENV but not a Hash: `[]` is
    `String?`, `[]=` with nil unsets, `fetch`, `key?`, `delete`, `to_h`,
    `keys`, `each`), and `IO` with `STDIN`/`STDOUT`/`STDERR` (`write`,
    `print`, `puts`, `printf`, `<<`, `flush`, `fileno`, `tty?`; `gets`,
    `read`, `each_line`, `readlines`, `eof?` on stdin). STDOUT shares
    Kernel#puts's buffer; STDERR is unbuffered, and a write to it flushes
    stdout first only on a terminal (decision 60), so on a pipe the two
    streams keep MRI's order. `warn` and `abort(msg)` write to STDERR;
    `abort` exits 1. Globals are a fixed read-only set: `$stdin`,
    `$stdout`, `$stderr` read the constants, and `$0`/`$PROGRAM_NAME` and
    `__FILE__` are the Ruby file's name as given to the compiler, which is
    what `ruby main.rb` reports and keeps `__FILE__ == $0` true; any other
    `$name` is a compile error. `Kernel#gets` reads stdin only, where MRI
    reads the files named in ARGV first (ARGF). Tests feed programs with
    `# args:`, `# env: K=V` and `# stdin: "Go-quoted"` lines, and
    `# stderr: match` adds stderr to the MRI comparison
    ([example 51](examples/51_argv_env/main.rb),
    [example 52](examples/52_stdin/main.rb)).
62. Files: `File` is its own `@go_type` class over `*os.File` with
    buffered reader/writer (a `@go_type` class can't subclass `IO`), and
    shares `print`/`puts`/`printf` and `each_line`/`readlines` with `IO`
    through the `IOWritable`/`IOReadable` modules, where MRI uses
    `IO::generic_writable`/`readable`. `File.new(path, mode)` takes
    `r`/`w`/`a` and their `+` forms; `File.open` takes a block only and
    closes the file in `ensure`. Class methods: `read`, `write`,
    `readlines`, `foreach`, `exist?`, `file?`, `directory?`, `size`,
    `delete`/`unlink`, `rename`, `basename` (with a suffix or `".*"`),
    `dirname`, `extname`, `join`, `expand_path`, `absolute_path?`, all
    checked against MRI on edge cases (`"a."`, `".profile"`, `"//"`,
    `File.join("a//", "//b")`). `Dir`: `pwd`, `children`, `entries`,
    `glob`, `exist?`, `mkdir`, `rmdir`, and `mktmpdir` with a block (MRI
    needs `require "tmpdir"`, a no-op here). `children`/`entries` come back
    sorted where MRI uses readdir order, and `glob` is `filepath.Glob`
    (no `**` or `{a,b}`). A failed call raises MRI's `Errno::*` class
    (`ENOENT`, `EEXIST`, `EISDIR`, `ENOTDIR`, `EACCES`, `ENOTEMPTY`, all
    under `SystemCallError`) with MRI's message: `"<strerror> @ <MRI C
    function> - <path>"`. Anything else is an `IOError`
    ([example 53](examples/53_files/main.rb)). *Amended:* `File.symlink?`
    added (`os.Lstat`, `ModeSymlink`), needed to verify decision 64's
    `FileUtils.ln_s`.
63. `URI.parse`/`URI()` return `URI::Generic`, or `URI::HTTP`/`URI::HTTPS`
    (`HTTPS < HTTP < Generic`, matching MRI's own hierarchy so
    `is_a?(URI::HTTP)` holds for both) on Go's `net/url`. `port` defaults
    to 80/443 for `http`/`https` and is `nil` otherwise (only those two
    schemes' `default_port` is modeled). `to_s` concatenates components
    like MRI's own, not `net/url.URL.String` (which keeps an explicit
    default port MRI drops). `URI.join`/`URI#merge`/`#+` use
    `ResolveReference`, checked against MRI on trailing-slash, `..`,
    absolute-path and absolute-override cases. `URI.decode_www_form`
    splits on `&` only (not `;`, unlike old CGI query parsing) and is the
    inverse of the existing `encode_www_form`. `URI::InvalidURIError`
    (`< URI::Error < StandardError`) is raised with MRI's
    `bad URI (is not URI?): "<str>"` wording, both when `net/url.Parse`
    itself fails and when the string carries a raw byte MRI's stricter
    RFC 3986 parser never accepts unencoded (space, control bytes,
    `` "<>\^`{|} ``) that `net/url` would otherwise silently accept or
    percent-encode. *ponytail: not full RFC 3986 validation, so some
    strings MRI rejects still parse here*, checked against MRI in
    `testdata/run/uri_mid.rb`.
64. FileUtils and Tempfile, on `os`/`io`/`path/filepath` only (no new
    dependencies). `FileUtils.mkdir_p`, `rm_rf`, `rm_f`, `rm`, `cp`, `cp_r`,
    `mv`, `touch`, `ln_s`: each takes MRI's `pathlist` first argument (a
    single path or an `Array` of paths, checked against MRI's actual method
    signatures via `Method#parameters`, not guessed), so the `Array` case is
    routed through decision 12's overload-by-argument-class convention to a
    `__<name>_array` method that loops the plain-path body — no compiler
    changes needed. `cp`/`cp_r`/`mv`/`ln_s` copy/move/link into
    `dest/basename(src)` when `dest` is an existing directory, matching
    MRI's documented behavior. `rm` delegates straight to `File.delete`
    (decision 62), which already raises MRI's exact message, since real
    FileUtils.rm hits the same C function internally; `rm_f`/`rm_rf` swallow
    any error (MRI's implicit `force: true`). Keyword arguments (`noop:`,
    `verbose:`, `force:`, `preserve:`, `secure:`, ...) are not implemented —
    only each method's default, no-keyword behavior. `mv` falls back to
    copy+remove on `EXDEV` (cross-device), untested here since a temp dir is
    one filesystem. `cp` on a directory source raises `Errno::EISDIR`, not
    MRI's oddly-shaped `"Is a directory - read"` message from
    `IO.copy_stream` — a known, untested mismatch, since nothing here
    depends on its exact text.
    `Tempfile.new(basename)`/`.create(basename) { |f| }` wrap `File` over
    `os.CreateTemp` rather than subclassing it (`@go_type` classes can't be
    subclassed, decision 62); `Tempfile` is a plain struct holding a `File`
    ivar and delegating `write`/`gets`/`read`/`eof?`/`close` to it, the
    same delegation-not-reimplementation shape the Net::HTTP work used for
    `@go_type` limits. `.new` returns a `Tempfile` (`path`/`unlink`/`delete`
    plus `IOWritable`/`IOReadable`), matching MRI's `path` going `nil` after
    `unlink` (verified against real MRI, which was surprising — `unlink`
    doesn't just remove the file, it also clears `path`); `.create` returns
    a plain `File` and auto-deletes at block exit, also verified against
    MRI (`Tempfile.create` returns `File`, not `Tempfile`, and only the
    block form auto-unlinks — the blockless form leaves an open file the
    caller must close and unlink itself, both confirmed by running real
    MRI). No finalizer (Go has no GC hook comparable to MRI's): an
    unclosed, unreferenced `Tempfile` leaks until the process exits, same
    as the tmp file MRI's own docs warn `close`/`unlink` guards against.
    `Pathname`/`Find`/`Open3` etc. remain open, tracked in issue #1
    ([example 54](examples/54_fileutils/main.rb)).
65. `Pathname` is `@go_type string` (value semantics, like `String`/`Symbol`),
    a thin OO wrapper: `basename`/`dirname`/`extname`/`exist?`/`file?`/
    `directory?`/`read`/`write`/`children` (`Dir.children`, sorted per
    decision 62) all delegate straight to `File`/`Dir`, wrapping the result
    back in a `Pathname` where MRI does. `Kernel#Pathname(path)` is real MRI
    API and works unmodified (a capitalized method name, parsed as a call
    because it's followed by `(`). `+`/`/`/`join` reset to the right-hand
    side when it is absolute, otherwise concatenate with `File.join`'s
    seam-trimming (no `..` collapsing — same simplification as `File.join`,
    decision 62); MRI's `Pathname#+` additionally cancels a literal `..`
    component against the preceding one (`Pathname("/a/b") + ".."` is
    `/a`), which this skips. `relative_path_from` delegates to
    `path/filepath.Rel`, which matches MRI's `cleanpath`-based algorithm on
    every case checked by hand (common prefix, `.`/`..` components, the
    root-clamped case, and MRI's "different prefix"/"base_directory has
    .." `ArgumentError`s, which `Rel` also errors on) — `ArgumentError`'s
    message text is not matched, only its class and MRI's success/failure
    split. `==`/`eql?`/`hash`/`<=>` compare by path string; `<=>` and `+`
    require both sides to be `Pathname`, `==` accepts `untyped` and is
    `false` for anything else, like `String`/`Symbol`. Not implemented:
    `cleanpath`, `realpath`, `ascend`/`descend`, `find`, `glob`, `sub`/
    `sub_ext`, stat methods (`mtime`, `size`, …), and the write/rename
    family beyond `write` ([example 55](examples/55_pathname/main.rb)).
66. `Find`: `Find.find(*paths) { |path| ... }` is `path/filepath.WalkDir`,
    pre-order with sorted siblings (MRI's own `find.rb` sorts
    `Dir.children` before recursing, so the order already matches). All
    roots are checked to exist before anything is yielded, like MRI's
    upfront `File.exist?` pass, and a missing one raises `Errno::ENOENT`
    with MRI's un-tagged message (`raise Errno::ENOENT, d` names no C
    function, unlike decision 62's Errno raises). Errors found deeper in
    the tree (permission, a race) are swallowed, matching MRI's default
    `ignore_error: true`; there is no way to pass `ignore_error: false`.
    Both the block and method return void, so `Find.find` is a Go
    iterator (decision 4) and `break`/`next`/`return` in the block are
    plain Go, unlike MRI's `break value` there is no way to make
    `Find.find` itself return that value. `Find.prune` doesn't throw:
    it sets a package-level flag that the `WalkDir` callback checks right
    after the `yield(path)` call that ran it returns, translating a set
    flag on a directory entry into `filepath.SkipDir`; the flag is reset
    before every `yield`, so a `Find.find` nested inside another's block
    clears its own signal before control returns to the outer one. This
    only reproduces MRI's `throw :prune` when the call is the last thing
    the block does in that branch (the normal idiom, including MRI's own
    doc example); code after `Find.prune` in the same branch keeps
    running, where MRI would have unwound past it
    ([example 56](examples/56_find/main.rb)).
67. `Open3` execs argv directly, never a shell: `Open3.capture2("echo", "hello
    world")` is `os/exec.Command("echo", "hello world")`, not `sh -c`; MRI's
    other form, a single joined command string parsed by the shell, is not
    supported. `capture2`/`capture2e`/`capture3` return `[String,
    Process::Status]`/`[merged, Process::Status]`/`[String, String,
    Process::Status]`; `capture2e` reuses `Cmd.CombinedOutput`, whose single
    shared pipe keeps MRI's dup2 ordering between stdout and stderr. `popen3`
    is block-only (`{ (Open3::Writer, Open3::Reader, Open3::Reader,
    Process::Waiter) -> T } -> T`): rb2go has no finalizer to reap a process
    a non-block caller forgot to wait on, and the block form already fits
    `File.open`'s ensure-based close (decision 62). `Process::Status` (`pid`,
    `exitstatus`, `success?`) and `Process::Waiter` (`pid`, `value`, memoized
    so a block calling `wait_thr.value` and `popen3`'s own cleanup don't
    `Wait` twice) cover only what `Open3` needs of MRI's wait-status object.
    A start failure raises `Errno::ENOENT`/`EACCES` with MRI's `Open3`
    message shape (`"<strerror> - <cmd>"`, no ` @ <fn>`, unlike decision 62's
    File errors). This also taught the compiler to let a method take both
    `*rest` and a block: `sig`/`argNames` (decls.go) and `genArgs`
    (expr.go) now place the Go `blk` parameter, and the call-site block
    argument, before the trailing variadic, since Ruby allows `*rest` before
    a block but Go requires the variadic parameter last
    ([example 57](examples/57_open3/main.rb)).
68. `Logger` holds its device `untyped` and writes/flushes it through the
    dynamic dispatcher (no shared IO base, decision 48), since a device can
    be `$stdout`, a `File`, a `StringIO`, or a path String, which `Logger.new`
    opens itself in append mode, as MRI's `LogDevice` does; `nil` disables
    logging (MRI's `nil`/`File::NULL`). `debug`/`info`/`warn`/`error`/
    `fatal`/`unknown` take a message or a block (the block's value is the
    message, evaluated only when the level is enabled); since `block_given?`
    is unsupported, each has a paired `__<name>_block` (decision 48's
    block-routing) that repeats its own level check and write rather than
    forwarding a captured block through `add`, so a call can't override
    `progname` the way MRI's block form can. `level=`/`level` coerce a
    String/Symbol through the same table as MRI's `Severity.coerce` and
    raise its exact `invalid log level: X` message; `Logger::DEBUG` through
    `UNKNOWN` are `0..5`. `progname`/`progname=`, `formatter=` (a
    `^(String, Time, String?, untyped) -> String` Proc, MRI's four-arg
    signature) and `datetime_format=`/`datetime_format` are plain
    accessors. The default formatter reproduces MRI's exact
    `"%.1s, [%s #%d] %5s -- %s: %s\n"`, `Time#strftime`'s already-supported
    `%6N` giving the same microsecond default; a message that isn't a
    String or Exception is `inspect`ed, as MRI's `msg2str`. Not supported:
    `Logger.new`'s rotation/`level:`/`progname:`/`formatter:`/
    `datetime_format:` keyword arguments (decision 23; use the setters),
    `close`, `<<`, and the "Logfile created on …" header MRI writes to a
    new file (itself a timestamp, so never MRI-diffable). Tests can't
    byte-compare the default formatter's output, since MRI always embeds
    the real PID and `Time.now` with no way to fix either from outside a
    custom `formatter=`; example 58 sets a `formatter=` that drops time,
    pid and (when absent) progname, and checks `datetime_format=` and the
    default formatter's shape with `start_with?`/`end_with?`/`include?`
    instead of printing their output raw
    ([example 58](examples/58_logger/main.rb)).
69. `IPAddr` is `@go_type struct { addr netip.Addr; bits int }`, always
    defined like decision 50. `IPAddr.new` parses a v4 or v6 literal, with
    an optional `/prefixlen` or `/netmask` suffix (dotted-quad for v4, hex
    groups for v6), and masks the host bits immediately, like MRI:
    `IPAddr.new("10.0.0.5/24").to_s` is `"10.0.0.0"`. `ipv4?`/`ipv6?` are
    `netip.Addr.Is4`/`Is6`, so `"::ffff:1.2.3.4"` (written with colons)
    counts as v6, matching MRI. `mask` takes an Integer prefix length or a
    netmask string; `include?` takes an `IPAddr` or address string and
    checks the whole argument's range is inside `self`'s (its prefix must
    be at least as specific, and its network address must fall on `self`'s
    prefix), raising `InvalidAddressError` on a bad string, same as MRI.
    `succ` adds 1 to the raw address integer (not clamped to the prefix)
    and raises `InvalidAddressError` with MRI's overflowed decimal value
    past `255.255.255.255`/`ffff:...:ffff`. `to_range` is `network..broadcast`
    (both endpoints get a full-width prefix, like MRI's `to_range`).
    `<=>` is typed `(IPAddr) -> Integer?` (Comparable's adapter needs
    `Op_cmp(Self)`, decision 3), so unlike MRI it doesn't accept a String
    and returns `nil` only across address families; `==` is `untyped` and
    coerces a `String` argument, returning `false` on a parse failure or
    a family mismatch. Both compare address value only; `eql?`/`hash`
    additionally require the same prefix length, so `10/8 == 10/16` but
    `!10/8.eql?(10/16)`, matching MRI exactly. `inspect`
    prints the address and netmask fully expanded (no `::` compression,
    unlike `to_s`). Not implemented: `&`/`|`/`~`/`<<`/`>>`, `native`,
    `hton`, `ip6_arpa`/`ip6_int`, `ipv4_compat`/`mapped`, `link_local?`/
    `loopback?`/`private?`, `zone_id`, and the in-place `mask!`/`succ!`
    family ([example 59](examples/59_ipaddr/main.rb)).
70. `Etc` is a small, partial library over `os/user` and `runtime`, per the
    tracking issue (part of getpwuid): `getlogin` (`os/user.Current`, nil if
    it fails), `nprocessors` (`runtime.NumCPU`), `systmpdir` (`os.TempDir`,
    matches MRI on every platform Go's `os.TempDir` covers), and
    `getpwuid(uid = nil)`/`getpwnam(name)` (`os/user.Current`/`LookupId`/
    `Lookup`, raising MRI's `ArgumentError` message `"can't find user for
    <uid-or-name>"`). `Etc::Passwd` keeps MRI's full 10-member order
    (`name`, `passwd`, `uid`, `gid`, `gecos`, `dir`, `shell`, `change`,
    `uclass`, `expire`) so `members`/`to_h.keys` match MRI, but only
    `name`/`uid`/`gid`/`dir` are real (from `os/user.User`'s `Username`/
    `Uid`/`Gid`/`HomeDir`) and `gecos` is best-effort (Go's `User.Name`,
    which is only the GECOS field's first comma-separated component, not
    MRI's full string). `passwd`, `shell`, `change`, `uclass` and `expire`
    have no `os/user` equivalent on any platform and are always `nil`.
    `sysconfdir` is Ruby's own `--sysconfdir` configure-time constant, not
    an OS fact Go's stdlib knows, so it is not implemented; nor is
    enumerating the whole passwd/group database (`Etc.passwd { }`,
    `Etc.group { }`, `getpwent`, `getgrent`/`getgrnam`/`getgrgid`), since
    `os/user` only look up one entry at a time
    ([example 60](examples/60_etc/main.rb)).
71. `Timeout.timeout(sec, klass = nil, message = nil) { ... }` races the
    block against a second goroutine that only sleeps (`time.After`); no
    compiler support, so it is a prelude-only library (decision 26: threads
    are goroutines, and Go still has no thread preemption). `sec: nil`
    skips the race and just runs the block. On timeout it raises `klass`
    (default `Timeout::Error < RuntimeError`) constructed with `message`
    (default `"execution expired"`), through the same dynamic
    `singleton(StandardError)` dispatch as decision 19's `klass.new(...)`,
    not a compiler intrinsic. **This is not MRI's semantics and cannot be
    made to be:** MRI's `Timeout.timeout` can interrupt a genuinely stuck
    or CPU-bound block, because it raises an exception *into* that
    thread. A goroutine cannot be preempted from outside, so this
    implementation can only detect that the deadline passed and raise
    from the *caller's* side; if the block never returns on its own, its
    goroutine keeps running in the background forever (leaked), racing
    whatever it touches after the caller has already moved on. Programs
    that rely on `Timeout` to actually kill a stuck computation (rather
    than just bound how long they wait for it) will not observe MRI's
    behavior here ([example 61](examples/61_timeout/main.rb)).
72. `TSort` is a mixin: a class `include`s it and defines
    `tsort_each_node`/`tsort_each_child`, structurally the same as
    Comparable's `<=>` and Enumerable's `each` (decision 9); it gets
    `tsort`, `tsort_each`, `strongly_connected_components` and
    `each_strongly_connected_component` for free, via Tarjan's algorithm
    written to match MRI's `tsort` gem: ids are assigned in
    `tsort_each_node`'s visitation order and each finished component is
    popped off the DFS stack in push order, so output order is
    bit-identical to MRI whenever both walk the same insertion-ordered
    Hash/Array. A cycle raises `TSort::Cyclic` with MRI's exact
    `"topological sort failed: #{component.inspect}"` message. The
    module-function form (`TSort.tsort(each_node, each_child)`,
    `TSort.strongly_connected_components(each_node, each_child)`) is
    **not implemented**: there, `each_node`/`each_child` are objects that
    themselves take a block (`each_node.call { |n| ... }`), which needs
    an RBS proc type with a block clause (`^() { (Node) -> void } ->
    void`); rb2go's proc grammar is `^(A, B) -> R` only
    (`internal/rbs/rbs.go`), so typing it would be a compiler change,
    out of scope here. Separately, a block passed to
    `each_strongly_connected_component` (a Go-iterator-shaped method:
    void return, void block, no `rescue`) can only be `yield`ed to
    directly or forwarded whole via `&block` to another iterator-shaped
    method, not stored, passed as a plain value, or forwarded to the
    closure-shaped recursive helper (it returns `Integer`, so decision
    4's iterator rule excludes it) — so the Tarjan recursion collects
    finished components into an `Array[Array[Node]]` and
    `each_strongly_connected_component` yields them after the walk
    completes, rather than streaming them one at a time as MRI's
    Enumerator does; the visible behavior (order, `Cyclic`) is unchanged
    ([example 62](examples/62_tsort/main.rb)).
73. `Abbrev` (`require "abbrev"` is a no-op, like decision 50's libraries):
    `Abbrev.abbrev(words, pattern = nil)` and `Array#abbrev` port MRI's
    `lib/abbrev.rb` verbatim (a prefix seen once maps to its word, seen
    twice it's ambiguous and dropped, and `case`'s `else`/`break` stops
    shortening further once that happens; full words always map to
    themselves even when some other word's prefix collides, e.g. `"car"`
    stays `"car"` alongside `"cars"`). `pattern` is `untyped`, since
    rb2go's `Regexp` is compile-time literals only (no `Regexp.new` from a
    runtime string, decision 24): a `String` pattern is matched with
    `start_with?` instead of MRI's anchored `/\A.../` (same result, no
    Regexp construction needed), and a `Regexp` pattern calls `match?`
    directly, unanchored like MRI's. `Array#abbrev` maps `self` through
    `to_s` first (a no-op for `Array[String]`, since `String#to_s` is
    `self`) because Go generics can't pass a generic `Array[E]` where
    `Array[String]` is wanted ([example 63](examples/63_abbrev/main.rb)).
74. `Observable` (MRI's `require "observer"`; the gem name doesn't match
    its RBS stdlib signature directory, "observable", so the test harness's
    `require` → `-r` mapping (rb2go_test.go) special-cases it):
    `add_observer(observer, func = :update)`, `delete_observer`,
    `delete_observers`, `count_observers`, `changed(state = true)`,
    `changed?`, `notify_observers(*args)` calling each registered
    observer's `func` with `*args` only while `changed?`, then clearing
    it. Unlike Comparable/`<=>`, Enumerable/`each` (decision 9) or TSort
    (decision 72), it needs nothing from the
    including class, but it still can't hold its state (the peers list,
    the dirty flag) in `@ivar`s the way a plain class method would: a
    mixin module's methods compile to a Go function generic over `Self`
    (struct/module methods become free funcs), and an ivar write needs a
    concrete Go struct field on the owning class, which a module has
    none of (confirmed by trying it: `instance variables are only
    supported in struct classes`). So the state lives in a Go-stdlib side
    table keyed by object identity (`prelude/go/observable.go`, a
    `map[any]*state` behind a `sync.Mutex`; entries are never evicted,
    a deliberate leak for process-lifetime objects) instead. The
    observer's method is still invoked through plain Ruby `send` inside
    `notify_observers`, going through rb2go's ordinary computed-name
    `send` codegen (decision 32) — except that codegen doesn't forward a
    splat (`observer.send(func, *args)` fails to compile: "unsupported
    syntax: SplatNode"), so the actual call is a one-line `%x{}` to the
    generated `rbSendByName` directly, spreading the rest array with
    Go's own `(*args)...`. `notify_observers` keeps a (MRI doesn't have
    this) `observer.respond_to?(func)` recheck right before that
    specifically to guarantee `rbSendByName` is generated whenever
    `notify_observers` itself is compiled, rather than depending on
    `add_observer` being reachable too ([example 64](examples/64_observable/main.rb)).
75. `Kernel#at_exit { }` pushes onto a Go slice; the generated main's
    deferred `rbTopRecover` (and a thread's `exit`) pops handlers LIFO,
    so one registered inside a handler runs next. Probed on MRI 4.0: the
    handlers run after main returns, after `exit` and after an uncaught
    exception, and the uncaught error's message prints *after* them. A
    handler's `exit n` replaces the status (even after an uncaught error,
    which still prints); a handler's own exception prints at once and
    makes the status 1. `$!` isn't available inside a handler. The
    method returns nil rather than the Proc
    ([testdata/run/control_at_exit.rb](testdata/run/control_at_exit.rb)).
76. Class values at run time. Every class object gets a generated
    `_IsInstance(any) bool` (next to decision 28's constant table), the
    run-time half of `is_a?`: it looks the class up in the value's class
    ancestry by ID (decision 82); for a module, that ancestry is where its
    includers are recorded (decision 21's static check still errors where
    the static type can't decide). On it:
    `Module#===` (so `when k` with a class value works),
    `x.is_a?(k)`/`kind_of?(k)` where `k` is a `singleton(C)`/`Module`
    value or untyped (a TypeError "class or module required" otherwise),
    and `Kernel#instance_of?(k)` by class ID. `rescue k`/
    `rescue *list` with class values stay unsupported: rescue a common
    ancestor and test `k === e`, re-raising on no match
    ([testdata/run/dynamic_is_a.rb](testdata/run/dynamic_is_a.rb)).
77. Method-name tables: every class object gets a generated `_Methods`
    list of its public instance methods, then its ancestors' (included
    modules and superclasses), short of Object/Kernel/BasicObject, whose
    methods MRI also lists; `initialize`, private methods and `__`-named
    prelude internals are left out. `public_instance_methods(inherit =
    true)`, `instance_methods` (the same: there is no `protected`),
    `method_defined?` and `public_method_defined?` read it; only the
    first selects `_Methods`, so the pruner (decision 49) drops the
    tables from programs that don't reflect. Order follows definition,
    ancestors after; MRI's differs, so sort. minitest finds `test_*`
    methods this way. `Enumerable#grep`/`grep_v` (by `pattern === x`,
    no block form) came along ([testdata/run/object_mid2.rb](testdata/run/object_mid2.rb)).
78. Small enablers for minitest (issue #6), each general:
    - **Require hooks.** A user file's top-level `require "a/b"` calls
      the prelude's private `Kernel#__require_a_b` (non-alphanumerics
      become `_`) when one exists; otherwise it stays a no-op (decision
      50). `require "minitest/autorun"` is the first hook.
    - **`# @dynamic`** on a prelude method silences decision 32's
      dynamic-call warnings inside it: the method is duck-typed by
      design (Logger's device, Zlib's IO, `grep`'s pattern), and the
      warning pointed users at prelude lines they can't change. The
      test harness now fails on any warning from `prelude/`. In user
      code it is a compile error.
    - **`Process.clock_gettime(id)`** for `CLOCK_REALTIME`,
      `CLOCK_MONOTONIC` (counted from process start; MRI's from boot, so
      only differences agree) and `CLOCK_PROCESS_CPUTIME_ID`, as a Float
      (no unit argument); any other id raises `Errno::EINVAL`. Plus
      `Process.pid`.
79. minitest (issue #6): `prelude/minitest.rb` ports minitest 6.0.6, the
    version MRI 4.0 bundles, so a test file runs unchanged on both, and
    rb2go's output (run order, dots, failure reports, diffs, counts,
    `-v`, `--show-skips`, filters) equals MRI's apart from the
    `Finished in` timings. `require "minitest/autorun"` is a require hook
    (decision 78) calling `Minitest.autorun`, an `at_exit` (decision 75).
    The run order reproduces minitest's `srand(seed); shuffle` with the
    MT19937 of decision 46, so `Minitest::Spec` is registered to keep
    `Runnable.runnables` MRI's length. No `inherited` hook fills the
    list while the program runs: every class object has a generated
    `_Descendants` list in definition order, the order the hook would
    have seen (classes are declared statically), and `runnables` reads
    Runnable's, less `Result`, which MRI defines before its hook. Tests are found
    through `public_instance_methods` (decision 77). They are called, and
    `assert_operator`/`assert_predicate` send their operator, without
    `rbSendByName`, whose switch over every method name keeps the whole
    prelude (a 235k-line, 20-second build): each user-defined class gets a
    generated `_Call(name, args...)` over the methods user code defined on
    it (through decision 32's Dyn wrappers), and core values answer a
    fixed list of common operators and predicates (`==`, `<`, `include?`,
    `even?`, `empty?`, ...), a NoMethodError for any other. Shape changes, each marked `port:` in the source:
    `@@vars` become constants; OptionParser is hand-parsed (same options
    and help text); Reportable and Assertions' methods live on
    Runnable/Test, because a module can't see its includers' attribute
    types; messages are typed procs, so a user's `msg` is a String, not a
    Proc; `rescue *exp` is a catch-all testing `k === e` (decision 76), and
    `assert_raises` returns `Exception`; `mu_pp` has no encoding lines;
    `diff -u` runs over temp files as in minitest. A failure's
    `[file:line]` is found when the Assertion is made, from the Go stack
    (`rbMtLocation`): of the frames in user code (prelude frames stand for
    MRI's filtered `lib/minitest` ones; generated forwarders, Go methods,
    are skipped), the one after the last whose method is named like an
    assertion, as minitest's rule, else the first. An Assertion made
    inside a `rescue` (`assert_raises`'s `flunk`) is created by a deferred
    recover, whose stack still holds the frames that panicked, frames
    MRI has already unwound; those are skipped, from `runtime.gopanic` to
    the function that deferred the recover. The Go code after a block's
    closing brace gets a `//line` for the call that took the block, so a
    multi-line statement doesn't drift to later Ruby lines. rb2go has no
    backtraces yet, so an error's report and `exception_details` print
    `No backtrace` where MRI lists frames: tests that error differ from
    MRI there. Spec's DSL is compiled (decision 83). Not ported:
    `assert_output`/`capture_io` (no `$stdout` reassignment),
    `assert_throws`, `assert_pattern`, `parallelize_me!`, class-body
    calls like `i_suck_and_my_tests_are_order_dependent!`, plugins,
    `stub`/`Mock` ([example 65](examples/65_minitest/main.rb)).
    Along the way: a method whose block yields no values is a closure,
    not an `iter.Seq` (iterators yield one or two values); `Regexp.escape`,
    `Kernel#object_id` (addresses, not MRI's numbers), `exit!`-like
    `__exit_bang`, and the `NoMemoryError`, `SignalException` and
    `Interrupt` classes, which rb2go never raises.
80. `rb2go test [-v] [-run regexp] [-p n] [-race] [-gcflags f] [-work]
    [paths...] [-args ...]`, shaped like `go test`: directories expand
    recursively to `*_test.rb`/`test_*.rb` (minitest's glob, without
    spec files; dot directories skipped); each file is its own closed
    world and program, built in parallel (`-p`, default GOMAXPROCS), then
    run in order in its own directory, as `ruby x_test.rb` would be, so
    failure locations match MRI's. A passing file prints `ok  <path>
    <secs>`; a failing one its captured output, then `FAIL <path>`; a
    build error `FAIL <path> [build failed]`. `-v` streams output and
    passes minitest `-v`; `-run re` becomes minitest's `-i /re/`; words
    after `-args` go to every test binary (`--seed 7`, `-e /x/`). The exit
    status is 1 when any file fails. No `--mri` comparison yet: the repo's
    TestMinitest does that. Multi-file tests wait on `require_relative`
    in user code.
81. Behaviour checks move to minitest. `TestMinitest` runs
    `testdata/test/*_test.rb` with `--seed 1` (the MRI cache key now
    includes extra arguments): MRI must pass, or the harness reports that
    the test itself is wrong, and rb2go's stdout and exit code must equal
    MRI's with the timing lines blanked. Because a passing assertion is
    checked by rb2go's own `==`, the expected side is a literal (MRI
    passing proves it right), `assert_raises` also checks `.message`, and
    `refute_*`/`assert_in_delta` are never a check's only evidence; a
    trace oracle logging every operand would lift that rule (issue #6).
    The String checks moved first: `testdata/run/string_*.rb` became
    `testdata/test/string_test.rb`, `string_frozen_test.rb` (the
    `frozen_string_literal` pragma is file-wide) and
    `string_reopen_test.rb` (reopening String changes the whole closed
    world), three builds instead of sixteen. What tests output itself
    (`puts`/`print`/`p` formatting, exit status, an uncaught crash) stays
    print-and-compare in `testdata/run`, as do `# skip:` known failures.
82. Generated programs never use `reflect` (the Go package for inspecting
    types while a program runs): rb2go compiles Ruby, so what MRI asks its
    object model while running, the compiler writes out as class metadata.
    - **Class IDs.** Each class's ID is its position in the class list.
      Every Go type that holds Ruby values gets a generated `_ClassID()`:
      struct classes, metaclasses, `@go_type` classes, generics and
      tuples (`Array`'s). `Boolean`'s answers `TrueClass` or `FalseClass`
      by value.
    - **Tables.** `rbClassNames` holds the names. `rbAncestry` lists each
      class's ancestors: itself, its superclasses, and every module they
      include.
    - **Readers.** `rbClassName`, `is_a?`/`kind_of?`/`instance_of?` with
      a class value, and `Module#===` read these tables (`rbClassID`,
      `rbKindOf`). A class object's `_DescID` names the class it describes.
    - **Values that can't carry methods.** nil and Go func types can't
      have methods, so nil is `NilClass` by value, and Procs are
      recognized by a generated type switch over every Proc type the
      program uses (`rbIsProc`).
    - **Boxes.** A `T?` box is opened by the generated `rbUnbox`/
      `rbKeyUnbox` switches. They cover every box type rendered, plus a
      box of each concrete type argument of a generic, since generic code
      may hold `E?`.
    - **Identity.** Classes whose values are pointers get a `_Ref()`
      marker: `#inspect` prints the pointer's address (`%p`), and
      `object_id` hashes it. No `unsafe` either. `_Ivars` records whether each field is
      nil, decided from the field's type when it is generated.
    - **Enforcement.** `reflect` is out of the import table
      (`format.go`), so a prelude helper that uses it doesn't compile, and
      `.golangci.generated.yml`'s depguard denies it.
    - **Remaining gap.** A Go value held in a prelude ivar has no class,
      and prints as `Object`.
83. `Minitest::Spec`'s DSL is compiled, not run. MRI builds specs while
    the program runs: `describe` makes an anonymous `Class.new(Spec)` and
    `class_eval`s its block, and `it`/`let`/`before`/`after` call
    `define_method`. rb2go's closed world has neither, so the compiler
    does that work and a spec is ordinary classes and methods:
    - **`describe X do ... end`** (top level, or inside a spec) is a
      class declaration: a subclass of `Minitest::Spec`, or of the
      enclosing spec, named as MRI names it (`Stack`,
      `Stack::when empty`) through a display name, since the class itself
      is anonymous in MRI. Its block is the class body, so `def` works
      in it; a local variable doesn't (it would be a class-body local).
      The description must be a literal string, symbol or constant.
    - **In a `Minitest::Spec` descendant's body** (`describe` blocks and
      `class FooSpec < Minitest::Spec` alike):
      - `it "does x" do ... end` (and `specify`) declares
        `test_0001_does x`, numbered per class. With no block, it
        skips "(no tests defined)". The Go method name escapes the
        characters Go can't hold (`goIdent`).
      - `before`/`after` declare `setup`/`teardown` calling `super`
        first/last; the last of each wins, as with `define_method`.
      - `let(:x) { ... }` and `subject { ... }` declare `x`, memoized
        per test behind a flag, so `false` and `nil` memoize too. Its
        type is the block's; a `#:` on the `let` line types it, which a
        `nil`-only block needs.
    - **Nesting.** A nested describe inherits the outer's lets, hooks
      and helpers, but not its tests (MRI's `nuke_test_methods!`).
    - **Checks at compile time.** The DSL outside a spec, a `let` named
      `test*` or shadowing a Minitest::Spec method, and a block with
      parameters are compile errors, with MRI's wording where MRI has
      one.
    - **Known difference.** An `it` without a block reports its skip at
      the user's line, where MRI reports its own `lib/minitest` path.

    - **Expectations.** `_(x)`, `value(x)` and `expect(x)`, or
      `_ { ... }` for `must_raise`, return a `Minitest::Expectation`.
      Its 30 `must_*`/`wont_*` methods are written out, each calling its
      assertion the way minitest's `infect_an_assertion` does, so failure
      messages and assertion counts match MRI's. Not ported:
      `must_output`, `must_be_silent`, `must_throw`, and
      `must_pattern_match`/`wont_pattern_match`.
    - **Optional blocks.** Methods take an optional block (`?{ ... }` in
      RBS); a call without one passes a nil func.

    Not supported: `register_spec_type` and `describe` with a computed
    name ([example 66](examples/66_minitest_spec/main.rb),
    [testdata/run/minitest_spec.rb](testdata/run/minitest_spec.rb),
    [testdata/test/spec_test.rb](testdata/test/spec_test.rb)).
