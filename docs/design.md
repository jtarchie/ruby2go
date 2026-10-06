# rb2go design record

How rb2go got its shape, and the numbered decisions (below) that bind it. For
installing and using rb2go, see the [README](../README.md).

I want to compile a subset of Ruby to Golang. The runtime of Golang supports
enough that I believe it to be possible.

Requirements:

- use this type syntax (https://github.com/soutaro/rbs-inline,
  https://github.com/soutaro/rbs-inline/wiki/Syntax-guide)
- we should be able to support `class`, `module`, `includes`, and inheritance

Anti-goals:

- we don't need to support eval or `define_method`. Class-based virtual
  dispatch on `self` **is** required — inheritance doesn't work without it
  (see [01_inheritance](../examples/01_inheritance/)).
- *Amended:* reflective dispatch (`send`, `method_missing`, `const_get`,
  `constants`) is supported, generated from the closed world rather than
  looked up at run time: typed code never pays for it, calls on `untyped`
  values do, and the compiler warns at each one (decisions 28–33,
  [docs/dynamic-dispatch.md](dynamic-dispatch.md)).

I want to support all the native types of Ruby as Golang primitives, but with
methods. These are ideas, and not limited to or the strict implementation.

## Layout

```
rb2go.go              public API: embeds the prelude, calls the compiler
cmd/rb2go/            CLI: rb2go build|run|gen main.rb and rb2go test [paths], like go build|run|test
internal/compiler/    Ruby → Go: declarations, types, codegen
internal/rbs/         the RBS type-syntax subset the compiler understands
prelude.rb            core library entry point; require_relatives prelude/*.rb
prelude/              core library, written in Ruby, compiled by the same transpiler;
                      includes net_http.rb and webrick.rb over Go's net/http
examples/NN_*/main.rb  one feature per program; runs on MRI unchanged
testdata/run/*.rb     what output itself shows (puts/print, exit, crashes, stdin): print-and-compare
testdata/test/*_test.rb  behaviour checks, one minitest file per area: MRI must pass, rb2go must match
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
`go run ./cmd/rb2go gen main.rb` when needed.

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

Example: [00_string_hierarchy](../examples/00_string_hierarchy/) —
[main.rb](../examples/00_string_hierarchy/main.rb).

### The prelude idea

The real question: can the whole object model be *written in Ruby*, with the
transpiler only providing syntax sugar? Yes — [prelude.rb](../prelude.rb) is the
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
  `untyped` (see [06_puts](../examples/06_puts/)), and when the literal is a
  receiver or bound with `:=` (`(-1).abs`, `case 3`, `false && x`), where Go
  would infer `int`/`string`/`bool`.

## Examples

Each `main.rb` produces identical output under `ruby` and as transpiled Go.
User code is plain Ruby that runs on MRI, so the transpiler is tested by
diffing against MRI output. The design notes below describe the Go shape
each feature compiles to.

### 01 — Inheritance, `super`, overriding

[main.rb](../examples/01_inheritance/main.rb)

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

[main.rb](../examples/02_enumerable/main.rb)

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

[main.rb](../examples/03_nil/main.rb)

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

[main.rb](../examples/04_exceptions/main.rb)

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

[main.rb](../examples/05_word_count/main.rb)

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

[main.rb](../examples/06_puts/main.rb) ·
[prelude `Kernel#puts`](../prelude.rb)

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

[main.rb](../examples/25_resty/main.rb) is a typed port of
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
anti-goal list above:

- `"#{ns}::#{path.camelize}Controller".constantize` and
  `const_get(action_name)` become a typed registry,
  `Hash[String, singleton(Resty::Action)]`;
- `Actions.constants.map { const_get }` becomes an explicit `ALL` list;
- `NullController`'s `method_missing` becomes a `NullAction` class;
- `Rack::Request` becomes `Resty::Request`, built from WEBrick's request;
  ActiveRecord becomes an in-memory `Resty::Model`;
- idioms whose types are unions (`path =~ re || …` as a `bool`,
  `match(...)[1]` on a possible nil) are spelled with `match?` and a check.

[Example 32](../examples/32_resty_reflective/main.rb) then compiles resty's
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
   A block that takes no values qualifies too: the method returns a
   `func(func() bool)`, which Go ranges over with no loop variables, so
   `Kernel#loop { ... break }` is a plain `for range`. *(Revised: blocks
   with no parameters were always closures, and `loop` did not exist.)*
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
   (`Rect` is `RectI`, `Rect?` is `*RectI`). Example 03's "one
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
9. Module constraints are derived from the module body, as the prelude section
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
    `untyped`. A local assigned both is `Integer | Float`
    (`total = 0; total += 1.5`), so its later arithmetic switches on the
    member (decision 150; it was `untyped`, and dynamic), and
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
    methods ([example 33](../examples/33_empty_literals/main.rb)).
14. Locals are inferred from their assignments (joined across branches:
    `nil` + `String` → `String?`, `Integer` + `String` → `Integer | String`
    (decision 150), and `x = nil` then `x ||= v` counts as
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
    time ([example 35](../examples/35_ivar_containers/main.rb)).
16. `%x{}` bodies are Ruby xstrings, so Ruby escape processing applies to
    the Go inside them: write `\\n` for a Go `\n`, `\#{` for a literal `#{`,
    and keep braces balanced (no `"{"` in Go strings). A one-line body of a
    non-void method gets `return` prepended. In user code a backtick or
    `%x()` is a shell command, as in Ruby (decision 97).
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
    arguments do (decision 32), except to a `T | untyped` parameter (a
    gradual parameter, not a union, decision 150): typed arguments are checked against
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
    method without keyword parameters passes a Hash, as Ruby 3 does.
    Keyword parameters (`a:`, `b: 1`, `**opts`) are ordinary Go
    parameters after the positional ones (`*rest` stays last, Go's
    variadic): every call site knows its target, so the compiler matches
    `name: value` arguments by name, fills a literal default at the call,
    and reports a missing or unknown keyword as a compile error (MRI's
    ArgumentError). A default that reads other parameters or `self`
    (decision 8's `rbArgc` scheme) runs in the callee: the Go function
    then also takes `rbKw int`, one bit per optional keyword the caller
    passed. `**opts` is a `Hash[Symbol, T]` of the call's other keywords
    and `**h` splats, in source order. The signature spells them as RBS
    does (`(a: Integer, ?b: String, **untyped opts)`) or per parameter
    with `# @rbs a: T`. A `**h` into named keywords (one `Hash[Symbol, T]`,
    no `**rest` parameter) is matched at run time: h is evaluated once, a
    key no keyword names raises MRI's "unknown keyword" ArgumentError,
    each keyword takes h's value when present (its `rbKw` bit then set at
    run time), else its default, else raises "missing keyword". A named
    `k: v` beside it wins. A present key whose value type cannot be the
    keyword's (`{ name: "a" }` holds Strings; `age:` takes Integer) is
    decision 20's TypeError (#51). A call on an untyped value passes its
    keywords as a trailing Hash; a keyword method's dynamic wrapper takes
    a last argument whose keys are all Symbols as the keywords (Ruby 2's
    reading: the call site cannot tell them from a positional Hash),
    unless the required positional parameters need it, and passes them
    on as `**h`, the run-time match above. Post parameters through an
    untyped call still raise ArgumentError. Keyword block parameters
    (`|a:|`) are not supported.
    *(Revised: keyword parameters were a compile error.)*
24. Regexps are Ruby syntax on Go's RE2. Every pattern gets `(?m)` (Ruby's
    `^`/`$` are line anchors), Ruby `/m` and inline `(?m)` become `(?s)`,
    `\h` is expanded. Onigmo syntax RE2 reads differently is rewritten: `\s`
    includes `\v`; POSIX brackets are Unicode (Go's tables); nested classes
    and `&&` are computed as rune ranges; `{,n}` is `{0,n}`; `X{n}?` is
    `(?:X{n})?`; `\u`/`\e` become `\x{...}`; `\Q` is a literal `Q`; plain
    groups don't capture once one is named. *(Revised: these passed through
    and silently matched RE2's meaning.)* Lookaround, backreferences and `\Z`
    are rejected with `file:line` at transpile time. A literal `/x` pattern
    drops its whitespace and `#` comments before translation (`source`
    and `inspect` keep them); an interpolated `/x` is stripped at run time,
    the values' spacing included, as MRI's. An inline group that turns x
    off (`(?-x:...)`, `(?-x)`, an interpolated Regexp's `(?-mix:...)`)
    keeps its spacing (`stripExtended`, shared with the run-time
    translator; #52). Static patterns compile once into package
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
    meaning.)* `$~`/`$1` are not supported; use `match`. A literal
    pattern with named groups on the left of `=~` assigns them to locals
    (nil without a match), since the names are known statically.
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
    cycles. Blocks cannot cross a dynamic call. A receiver-less call in a
    class that neither the class, a subclass (or what it includes),
    `method_missing` nor Object answers is a compile error, `undefined
    method x for C`: no run time can reach a method then. *(Revised: it
    was a dynamic call, a warning, and a NoMethodError when run.)* A
    module's body still dispatches, since an includer may define the
    name. nil held untyped answers Kernel's and Object's public methods
    (`nil.should`, `frozen?`) through its dispatcher's `recv == nil` arm,
    since a nil interface has no Go methods to assert. *(Revised: those
    were NoMethodError.)* A value typed `Object` or
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
    `&&`/`||` return values of any types (their union, decision 150) and
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
    depends on. Returns with no common type join to their union (decision
    150); recursion (direct or mutual) and blocks still need an annotation. An unannotated override
    of an inferred method takes the parent's inferred type. Parameter
    types are never inferred: that would make a method's type depend on
    its callers ([example 34](../examples/34_inferred_returns/main.rb)).
    *Revised (#56):* they are now, from the calls (decision 146). The
    cost named here is real, and accepted: editing one caller can change
    a method's signature. A caller that disagrees with the others is a
    compile error at that call, so the change cannot go unnoticed.
37. `Range[E]` is a struct `{b, e E; excl, endless bool}` handled as a
    pointer; `a..b`, `a...b` and `a..` build it inline (generic classes
    have no class methods, so there is no `Range.new`). A beginless range
    (`..5`) sets `beginless`: it covers, slices (`a[..1]`), matches in
    `case/when` and prints, and iterating it raises MRI's TypeError. Iteration (`each`, `step`, `to_a`, Enumerable)
    needs `Integer` or `String` (MRI's `String#succ`, without its
    all-digits mode); other `E` only compare (`cover?`, `include?`, `===`
    in `case/when`). `sum` and `size` are arithmetic, and `size` of an
    endless range raises (no Infinity), as does `last`; its `end` is
    `E`'s zero value, not nil
    ([example 36](../examples/36_ranges/main.rb)). *Revised (#56):*
    `min(n)` of an endless range is `first(n)` and `max(n)` raises MRI's
    RangeError, where Enumerable's sort-based ones never returned; a
    beginless range's `min(n)` raises too.
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
    ([example 37](../examples/37_time/main.rb)). *Revised:* `Time.new`,
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
    ([testdata/run/time_mid.rb](../testdata/run/time_mid.rb)).
40. `Rational` is a pointer to a `math/big.Rat`, so arithmetic is exact
    and never overflows; `numerator`/`denominator`/`to_i`/`round` raise
    RangeError past 64 bits (decision 35). Literals (`3r`, `3/4r`,
    `0.75r`), `Rational(n, d)` (Integers only, no strings), `Integer#to_r`,
    `Integer#quo` and exact `Float#to_r` build one. Integer and Rational
    mix into a Rational and Float and Rational into a Float, in either
    order, through decision 12's overloads; `r < 1` works the same way.
    Integer `**` with a negative exponent still raises (its type is
    Integer). `Float#round(n)` takes `n > 0` only: MRI answers an Integer
    for `n <= 0` ([example 38](../examples/38_rational/main.rb)).
41. `Date` is a Julian Day Number on the proleptic Gregorian calendar
    (MRI switches to the Julian calendar before 1582-10-15; rb2go does
    not). It is always defined, `require "date"` or not. `Date.parse`
    reads ISO dates, `y/m/d`, `Mar 5, 2024` and `5 March 2024`, not all
    of MRI's heuristics; `strptime` reads date fields only. `d - d2` is a
    Rational, `d ± n` a Date, `>>`/`<<` clamp to the month's end, and a
    Range of Dates iterates, since Range iterates anything with `succ`.
    `strftime`'s `%Z` prints `UTC` where MRI's Date prints `+00:00`
    *(fixed by decision 133, which also made Date a struct class)*.
    `httpdate`/`rfc3339` format (and `Date.httpdate`/`rfc3339` parse
    through `Date.parse`, which already reads both shapes) at midnight
    UTC, since a Date has no time of day. `jisx0301` prints MRI's Japanese
    era code (`M`/`T`/`S`/`H`/`R` + 2-digit era year) for a date on or
    after Meiji 6 (1873-01-01, when Japan adopted the Gregorian calendar;
    MRI's own table has no era code before it), plain ISO otherwise;
    `Date.jisx0301` parses either shape back
    ([example 39](../examples/39_date/main.rb)).
42. `Complex` is a pointer to `{re, im any}`: each part keeps its class
    (Integer, Rational or Float) and arithmetic follows MRI's rules for
    mixing them (`Complex(1, 2) / Complex(3, 4)` is exact, `Complex * real`
    scales part by part, Integer quotients that are whole come back as
    Integers). `real`, `imaginary`, `abs` and `abs2` are `untyped`, since
    their class depends on the parts. `3i`, `Complex(a, b)`,
    `Complex.polar`/`rectangular` build one; mixing with Integer, Float
    and Rational goes through decision 12's overloads. `**` takes an
    Integer (exact) or a Float (polar form)
    ([example 40](../examples/40_complex_math/main.rb)).
43. `Math` evaluates every transcendental function in 128-bit
    `math/big` and rounds once, so results are correctly rounded. Go's
    `math` is only within 1 ulp (and its `Sin`/`Cos` lose digits near
    multiples of π/2), which diverged from MRI's printed output on a few
    percent of inputs. MRI inherits the platform libm, which on macOS
    misrounds `tan`, `sin`, `cos`, `cbrt`, `hypot` and `atan` on a few
    percent of inputs; there rb2go is right and MRI is off by one digit.
    `Math.sqrt` stays Go's (IEEE-exact). `erf` is its Taylor series below
    |x| = 6 (±1 beyond), `erfc` that series' complement below 3 and its
    continued fraction above; `gamma` and `lgamma` are Stirling's series
    with Bernoulli numbers up to B₄₀, shifted to x ≥ 30 by the recurrence.
    macOS's libm rounds these four wrong far more often (about 40% of a
    grid of 416 inputs for lgamma and erfc, all checked against mpmath),
    so printed results differ from MRI there in the last digit. A call costs microseconds rather
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
    `Array#to_set` and `Enumerable#to_set` exist since decision 139:
    the instantiation cycle `Array[E]` → `Set[E]` → `Hash[E, …]` →
    `Array[[E, …]]` (see decision 9) that kept them out went away with
    decision 86's free funcs. *(Revised: `Set.new(xs)` was the only
    spelling.)*
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
    ([example 41](../examples/41_set/main.rb)).
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
    ([example 42](../examples/42_queues/main.rb)). *(Revised: `Thread.new(1, 2)
    { |a, b| ... }` forwards up to 3 constructor args into the block, by
    decision 12's arity overloads (`__new_1`/`__new_2`/`__new_3`).
    `join(timeout)` is `nil` on timeout, the Thread on success; `value`
    blocks and re-raises like `join`, then answers the block's value,
    `untyped` since Thread is not generic (decision 91: a block ending on
    a void call gives `nil`). `name`/`name=` read and
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
    *(all three since: decisions 104 and 108)*
    unsupported for the same reason as `owned?`.)* *(Amended by decision
    104: goroutines now have an identity, so `Thread.current`,
    `Thread.main`, `Mutex#owned?`, MRI's `ThreadError: deadlock; recursive
    locking` on relocking and `Attempt to unlock a mutex which is locked
    by another thread/fiber` from a non-owner all work. Thread-locals, `status` `"sleep"`
    and `Monitor` are still not done.)*
46. `Random` is MRI's MT19937 seeded as MRI seeds it (one 32-bit word
    through `init_genrand`, more through `init_by_array`), and `rand(n)`,
    `rand`, `rand(a..b)`, `rand(Float)`, `bytes`, `Array#shuffle`/`shuffle!`
    and `Array#sample` draw exactly as MRI does, so a seeded program prints
    the same numbers. Seeds are 64-bit (MRI's `new_seed` is 128); an
    unseeded generator is seeded from `crypto/rand`. `Kernel#rand`/`srand`
    use one shared generator; `rand(0)` raises (MRI returns a Float) and
    Float ranges are not supported. `random:` is passed as a Hash
    (decision 23). Also `Array.new(n, v)`, `Array.new(n) { |i| … }` and
    `String#bytes` ([example 43](../examples/43_random/main.rb)).
47. Procs are typed by RBS proc types (`^(Integer) -> Integer`) and are a
    Go `*func(Integer) Integer`: a pointer, so they satisfy `comparable`
    (decision 10) and can sit in Arrays and Hashes, and `==` is identity
    as for MRI's distinct procs. `->(x) { }`, `lambda { |x| }` and
    `proc { |x| }` build one; a lambda with parameters takes its types
    from the expected type (`#:` on the local, the parameter or the
    ivar), and one without infers its return type. *Revised (#56):* with
    no expected type, from its calls instead (decision 146); its return
    type is then inferred too. `call`, `.()`, `[]`,
    `yield`, `===`, `arity`, `>>`/`<<` (composition), `&f` (as a block,
    iterators included), `is_a?(Proc)` and `.class` work. Every Proc
    behaves as a lambda: `return` leaves only the Proc, arity is strict,
    and `lambda?` is true. `next v` now works in any block, giving the
    block's value. A Proc held `untyped` cannot be called dynamically
    ([example 44](../examples/44_procs/main.rb)). Kernel and Object
    methods (a user's included, `frozen?`, `tap`) are called statically
    with the func type as Self, and `.class` on an untyped Proc is
    `Proc`; a method a user defines on Proc itself is a compile error,
    since a func type carries no methods.
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
    ([example 45](../examples/45_stringio/main.rb),
    [testdata/run/stringio_mid.rb](../testdata/run/stringio_mid.rb)).
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
    *Amended:* a stub survives only for a name an interface literal
    asserts or a `_` marker; named interfaces drop the rest (decision 89).
    *Amended:* a prelude constant's assignment in `main` names it weakly,
    like a type-switch case: it runs only if something else names the
    constant, so an unused library's constants (`Logger::LEVELS`,
    `Minitest::HELP`, `Zlib::BEST_SPEED`, …) no longer keep their
    initializers and what those reach (`Minitest::Undefined`, `Hash`
    setters). Prelude initializers are allocations, so skipping one is
    unobservable; main.rb's constants stay strong, since a user's
    initializer may print or read input. The compiler hands the pruner the
    prelude constants' Go names. `05_word_count` went from 3.5k lines to
    2.4k, `puts 1` to 1.4k.
    *Amended:* pruning now runs before most of the output exists. With
    the computed-send switches in play (`dynAll`, which any prelude
    method's `send(name)` sets) nearly every method name was noted, so
    `emitDynamic` wrote a dispatcher per name over every class: 1.7 MB of
    a 4.7 MB emit, all of it pruned for a typical program, and 66% of a
    compile's CPU; forwarders were another 1.3 MB. Now `emitProgram`
    stops before forwarders and dispatchers, the pruner (`newPruner`,
    `add`, `run`) takes that as its first batch, and `emitNext` adds
    batches for what the kept code so far reaches: forwarders of reached
    classes, dispatchers of names it calls (`rbDyn<X>`), asks
    `respond_to?` of, or asserts (`Dyn<X>` in an interface literal), and a
    name a wrapper body notes while being emitted (`dynLazy` marks the
    names from before). When a round adds nothing, the tables every body
    contributes to (tuples, boxes, regexps, string literals, class
    tables) go out once; rounds continue after them, since they open
    paths too, and a late body that changes a table's inputs
    (`tableInputs`) is `errPruneIncomplete`: the compile reruns with
    everything eager (`dynEvery`), which is the old output. No program
    in examples/ or testdata/ needs that. Each batch is parsed into the
    same FileSet and merged for the printer. With the prelude's parse
    cached per process (`preludeFiles`; Prism runs interpreted under
    wasm, where it was 2.5 s of 5.5 s), a method index on `Class.lookup`
    and a binary search in the comment sweep, a compile is ~0.2 s native
    (was 0.69 s) and ~0.9 s as wasm in Chrome (was 5.5 s). `RB2GO_TIMING=1`
    prints the phases.
    *Amended:* a blank name is neither a root nor an index entry: `var
    x, _ = f()` was kept as if it were a `var _ = ...` marker, and again
    by the first `_` any kept code used, so `rbHalfPi` (π/2 to 110 digits
    for `Math.sin`'s argument reduction) sat in `puts "hello"`.

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
    either side ([example 46](../examples/46_digests/main.rb)).
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
    ([testdata/run/zlib_mid.rb](../testdata/run/zlib_mid.rb)). `DEFAULT_COMPRESSION`
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
    ([example 47](../examples/47_strscan/main.rb)).
52. `require "time"`'s parsers are always defined. `Time.iso8601`/
    `xmlschema`, `httpdate`, `rfc2822` and `parse` each try a fixed list
    of Go layouts, not `Date._parse`'s heuristics, so `Time.parse` reads
    ISO, RFC 2822/1123, `asctime` and a few `Mon D YYYY` shapes. A string
    without a zone is local time; a `Z`, `UTC` or `GMT` zone gives MRI's
    UTC mode, any other offset a fixed zone. `Time.strptime` rewrites the
    format into a Go layout, so literal text that spells a Go layout
    token is misread, and a directive without a Go twin fails to parse.
    `Time#httpdate`/`rfc2822` format, and `Benchmark.realtime` times a
    block ([example 48](../examples/48_time_parse/main.rb)). `Benchmark.measure`
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
    ([example 49](../examples/49_csv/main.rb), [testdata/run/csv_mid.rb](../testdata/run/csv_mid.rb)).
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
    element types) ([example 50](../examples/50_format_regexp/main.rb)).
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
    `#<Enumerator…>`. *(Superseded by decision 140: `__<name>_enum`
    returns a real `Enumerator`.)* Blockless `map`/`select`/`filter`/`reject` return
    `Enumerator::Map`/`Select`, whose `with_index` maps or filters as MRI's
    and whose inspect is MRI's. Block params may destructure one level
    (`|(k, v), i|`). Go forbids a method of `Array[E]` from building an
    `Array[Array[E]]`, `Array[E?]` or `Hash[E, …]` (an instantiation
    cycle), so such methods live in Enumerable or Go helpers.
    String stays immutable (see "Frozen strings"): no `<<`, `insert`,
    `prepend`.
59. The CLI is `rb2go build [-o prog] main.rb` and `rb2go run main.rb
    [args...]`, shaped like `go build`/`go run`. `rb2go gen main.rb`
    is the emit-Go-only mode: it prints the generated source to stdout
    (warnings to stderr) and needs no go command, so the output can be
    read, diffed or vendored; it takes no flags, since redirecting stdout
    covers `-o`. Each build/run call writes `main.go` and a `go.mod` (`go` directive
    `rb2go.GoVersion`, shared with the tests) into a fresh temp module and
    runs `go build -trimpath` there with `GOWORK=off` and an empty
    `GOFLAGS`, so the user's workspace can't leak in. Generated code is
    stdlib-only, so the build never touches the network, and Go's
    content-keyed build cache makes a warm `run` about 0.8s, mostly
    transpiling; there is no rb2go-level cache. `-work` keeps the module
    and prints its path (for rebuilding the module by hand); `-race` and
    `-gcflags` pass through. `run` executes the binary in the caller's
    directory, as `ruby main.rb` does, with stdio inherited, and exits
    with its status; transpile and build failures exit 1, bad usage 2.
60. Output and signals follow MRI where it shows at a terminal. stdout is
    buffered, but when it is a character device every write flushes, as
    MRI's does on a tty (`print "a"` shows before a later `$stderr` write;
    on a pipe both keep MRI's buffered order). SIGINT and SIGTERM flush
    stdout, then the program re-raises the signal with the default action,
    so it dies by it (130/143 in a shell) like MRI. *Amended (#2):*
    Ctrl-C is catchable where the main thread blocks. An untrapped
    SIGINT/SIGTERM is posted (`rbInterruptPost`), and `sleep`,
    `Thread#join`/`value`, `Queue#pop` and `ConditionVariable#wait` on
    the main goroutine take it (`rbTakeInterrupt`) and raise `Interrupt`
    (`message` "Interrupt", `signo` 2) or `SignalException` ("SIGTERM",
    15), so `rescue` and `ensure` run as in MRI; uncaught, it prints
    `Interrupt (Interrupt)` and ends the program by the signal. Go can't
    inject a panic into a computing goroutine, so a program that takes
    nothing within 200ms (`rbInterruptGrace`) dies by the signal as
    before; so does one blocked in `gets` (stdin is not pollable, so the
    read can't be woken), `Mutex#lock` or `SizedQueue#push`. A thread
    blocked in those calls sleeps on: the signal is the main thread's.
    `trap` (decision 100) still comes first. A program started with
    SIGINT ignored (a non-interactive shell's `&` job) now exits 130
    after the grace instead of ignoring it; MRI raises Interrupt there,
    which stays undone (`TestInterrupt`). `rb2go run`
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
    what `ruby main.rb` reports and keeps `__FILE__ == $0` true; `$$` is
    `Process.pid`; any other `$name` is a compile error (*`$?` added by
    decision 97; `$stdout` and `$stderr` assignable, decision 109*). `Kernel#gets` reads stdin only, where MRI
    reads the files named in ARGV first (ARGF; *revised by decision 95*). Tests feed programs with
    `# args:`, `# env: K=V` and `# stdin: "Go-quoted"` lines, and
    `# stderr: match` adds stderr to the MRI comparison
    ([example 51](../examples/51_argv_env/main.rb),
    [example 52](../examples/52_stdin/main.rb)).
62. Files: `File` is its own `@go_type` class over `*os.File` with
    buffered reader/writer (a `@go_type` class can't subclass `IO`), and
    shares `print`/`puts`/`printf` and `each_line`/`readlines` with `IO`
    through the `IOWritable`/`IOReadable` modules, where MRI uses
    `IO::generic_writable`/`readable`. `File.new(path, mode)` takes
    `r`/`w`/`a` and their `+` forms; `File.open` takes a block only and
    closes the file in `ensure`. *(Settled, #2: `File` stays no subclass
    of `IO`, so `is_a?(IO)` is false for a `File`; the shared modules
    give it IO's methods, and nothing has needed the ancestry.
    `Dir.children`/`entries` stay sorted: MRI's readdir order depends on
    the filesystem, so no test could compare it.)* Class methods: `read`, `write`,
    `readlines`, `foreach`, `exist?`, `file?`, `directory?`, `size`,
    `delete`/`unlink`, `rename`, `basename` (with a suffix or `".*"`),
    `dirname`, `extname`, `join`, `expand_path`, `absolute_path?`, all
    checked against MRI on edge cases (`"a."`, `".profile"`, `"//"`,
    `File.join("a//", "//b")`). `Dir`: `pwd`, `children`, `entries`,
    `glob`, `exist?`, `mkdir`, `rmdir`, and `mktmpdir` with a block (MRI
    needs `require "tmpdir"`, a no-op here). `children`/`entries` come back
    sorted where MRI uses readdir order, and `glob` is `filepath.Glob`
    (no `**` or `{a,b}`; *revised by decision 94*). A failed call raises MRI's `Errno::*` class
    (`ENOENT`, `EEXIST`, `EISDIR`, `ENOTDIR`, `EACCES`, `ENOTEMPTY`, all
    under `SystemCallError`) with MRI's message: `"<strerror> @ <MRI C
    function> - <path>"`. Anything else is an `IOError`
    ([example 53](../examples/53_files/main.rb)). *Amended:* `File.symlink?`
    added (`os.Lstat`, `ModeSymlink`), needed to verify decision 64's
    `FileUtils.ln_s`. *Amended by 138:* `IO` also wraps pipe and popen
    ends, so `IO.pipe`/`IO.popen` objects are `IO`s, not `File`s.
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
    ([example 54](../examples/54_fileutils/main.rb)).
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
    family beyond `write` ([example 55](../examples/55_pathname/main.rb)).
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
    ([example 56](../examples/56_find/main.rb)).
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
    ([example 57](../examples/57_open3/main.rb)).
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
    ([example 58](../examples/58_logger/main.rb)).
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
    family ([example 59](../examples/59_ipaddr/main.rb)).
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
    ([example 60](../examples/60_etc/main.rb)).
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
    behavior here ([example 61](../examples/61_timeout/main.rb)).
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
    ([example 62](../examples/62_tsort/main.rb)).
73. `Abbrev` (`require "abbrev"` is a no-op, like decision 50's libraries):
    `Abbrev.abbrev(words, pattern = nil)` and `Array#abbrev` port MRI's
    `lib/abbrev.rb` verbatim (a prefix seen once maps to its word, seen
    twice it's ambiguous and dropped, and `case`'s `else`/`break` stops
    shortening further once that happens; full words always map to
    themselves even when some other word's prefix collides, e.g. `"car"`
    stays `"car"` alongside `"cars"`). `pattern` is `untyped`, since
    rb2go's `Regexp` was compile-time literals only then (no `Regexp.new` from a
    runtime string, decision 24; *since added, decision 115*): a `String` pattern is matched with
    `start_with?` instead of MRI's anchored `/\A.../` (same result, no
    Regexp construction needed), and a `Regexp` pattern calls `match?`
    directly, unanchored like MRI's. `Array#abbrev` maps `self` through
    `to_s` first (a no-op for `Array[String]`, since `String#to_s` is
    `self`) because Go generics can't pass a generic `Array[E]` where
    `Array[String]` is wanted ([example 63](../examples/63_abbrev/main.rb)).
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
    `add_observer` being reachable too ([example 64](../examples/64_observable/main.rb)).
    *Revised (decision 147):* the side table and its global mutex are
    gone. Observable is plain Ruby over `@observer_peers` (a Hash of
    observer to method name, as MRI's) and `@observer_state`, instance
    variables a module may now hold, so the state is per object and
    freed with it.
75. `Kernel#at_exit { }` pushes onto a Go slice; the generated main's
    deferred `rbTopRecover` (and a thread's `exit`) pops handlers LIFO,
    so one registered inside a handler runs next. Probed on MRI 4.0: the
    handlers run after main returns, after `exit` and after an uncaught
    exception, and the uncaught error's message prints *after* them. A
    handler's `exit n` replaces the status (even after an uncaught error,
    which still prints); a handler's own exception prints at once and
    makes the status 1. `$!` isn't available inside a handler. The
    method returns nil rather than the Proc
    ([testdata/run/control_at_exit.rb](../testdata/run/control_at_exit.rb)).
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
    ([testdata/run/dynamic_is_a.rb](../testdata/run/dynamic_is_a.rb)).
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
    no block form) came along ([testdata/run/object_mid2.rb](../testdata/run/object_mid2.rb)).
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
    `even?`, `empty?`, ...), a NoMethodError for any other. The tables
    need only the dispatchers of the names they switch over, so kept
    code selecting `_Call` emits those (`callableNames`), not every
    method name as a computed send does: a one-test file's compile went
    from 0.9 s to 0.4 s, most of it 3 MB of Go generated to be pruned.
    Shape changes, each marked `port:` in the source:
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
    multi-line statement doesn't drift to later Ruby lines. An error's
    report and `exception_details` list the frames as MRI does, filtered
    to the test's own (decision 106). Spec's DSL is compiled (decision 83). Not ported:
    `assert_throws`, `assert_pattern`, `parallelize_me!`, class-body
    calls like `i_suck_and_my_tests_are_order_dependent!`, plugins,
    `stub`/`Mock` ([example 65](../examples/65_minitest/main.rb)).
    Along the way: `Regexp.escape`,
    `Kernel#object_id` (addresses, not MRI's numbers), `exit!`-like
    `__exit_bang`, and the `NoMemoryError`, `SignalException` and
    `Interrupt` classes, which rb2go never raises.
80. `rb2go test [-race] [-gcflags f] [-work] [paths and minitest flags...]`
    is Ruby's `minitest` command (minitest 6's `bin/minitest`; `rake test`
    loads files the same way): every test file is compiled into one
    program (decision 84), which runs once, with one seed, and prints one
    report, byte for byte MRI's apart from timings. Arguments are sorted
    as minitest's PathExpander does: one that exists on disk is a path (a
    directory expands to `**/{test_*,*_test,spec_*,*_spec}.rb`, sorted;
    `-path` excludes one), and any other is a flag for the program
    (`--seed 1`, `-v`, `-n /re/`); with no path, `test`. Files load in
    argument order, which sets the class order and so the seeded shuffle.
    rb2go's own build flags are recognized by name wherever they appear
    (minitest's never share them). The program runs in the caller's
    directory, and failure locations name files as given
    (`testdata/test/array_test.rb:12`), as MRI's do. Not yet: `path:LINE`
    selection. (Superseded: an earlier `go test`-shaped command built each
    file as its own program and printed `ok`/`FAIL` per file, which no Ruby
    runner does.)
81. Behaviour checks move to minitest. The suite loads the way Ruby's
    does: `minitest testdata/test --seed 1` (one MRI process) and
    `rb2go test testdata/test --seed 1` (one rb2go program, decision 84)
    both pass, with the same report. Sharing one process, the files keep
    to a namespace each (`module ArrayTests`, ...; top-level helper defs
    get a file prefix, since a top-level def is a private method on every
    object), and core-class reopens only add methods. `TestMinitest`
    still builds each file on its own (`--seed 1`; MRI must pass, or the
    harness reports that the test itself is wrong, and rb2go's stdout
    and exit code must equal MRI's with the timing lines blanked): as one
    program the suite is 946k lines of Go that take ~30 minutes to build.
    Dyn wrappers (decision 32) are emitted per class per name, so they
    grow as classes × names: dynamic_test's computed `send` asks for every
    name, and 27 Kernel/Object names alone land on all ~1650 classes
    (44k of 66k wrappers). Without dynamic_test it is still 480k lines,
    since each file's untyped call names get wrappers on the other files'
    classes. Making wrappers scale with definitions, not classes, is the
    next step, and then the harness becomes one build. Because a passing assertion is
    checked by rb2go's own `==`, the expected side is a literal (MRI
    passing proves it right), `assert_raises` also checks `.message`, and
    `refute_*`/`assert_in_delta` are never a check's only evidence. *(Lifted
    by decision 105's trace oracle, which compares every operand with
    MRI's; what remains is that a message rb2go words differently by
    design is not put through `assert_raises`.)*
    The String checks moved first: `testdata/run/string_*.rb` became
    `testdata/test/string_test.rb`, `string_frozen_test.rb` (the
    `frozen_string_literal` pragma is file-wide) and
    `string_reopen_test.rb` (reopening String changes the whole closed
    world), three builds instead of sixteen. Then every area: array,
    hash, control, dynamic, number, object, rxjson and the stdlib files
    are one `testdata/test/<area>_test.rb` each, so `testdata/run` went
    from 142 files to 27. Each file is still its own program and build
    (the closed world is one user file); fewer files is what makes the
    suite faster. What tests output itself (`puts`/`print`/`p`
    formatting, exit status, an uncaught crash, ARGV/stdin) stays
    print-and-compare in `testdata/run`, mostly in one `<area>_output.rb`
    per area, as do the `# skip:` known failures. A value compared
    through its printed form is asserted as that string (`assert_equal
    "1.0e+20", x.to_s`), so Float, Hash and inspect formatting stay
    checked exactly. The examples stay print-and-compare programs, as a
    Ruby project keeps `examples/` as scripts and its tests in `test/`:
    converting them to minitest tested nothing new (MRI's output is
    already an exact oracle) and made every example carry minitest's
    ~12k lines of Go, TestExamples 7x slower.
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
    - **Identity.** Classes whose values are pointers are flagged in
      the `rbClassRefs` table (`rbIsRef`; a `_Ref()` marker method until
      decision 89): `#inspect` prints the pointer's address (`%p`), and
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
      RBS); a call without one passes a nil func, which a def sees as a
      `Proc?` (decision 126).

    Not supported: `register_spec_type` and `describe` with a computed
    name ([example 66](../examples/66_minitest_spec/main.rb),
    [testdata/run/minitest_spec.rb](../testdata/run/minitest_spec.rb),
    [testdata/test/spec_test.rb](../testdata/test/spec_test.rb)).
84. A program may be several Ruby files, compiled as one closed world
    (`rb2go.CompileFiles`), as Ruby loads files into one process:
    - **Load order.** Files load in the given order, each one's top level
      (its statements, and what its class bodies run: constant
      assignments, hook calls) running whole before the next's, as
      `require` does.
    - **Scope.** Top-level locals are file-scoped, as in Ruby: each file's
      top level is its own Go block in `main`.
    - **Reopening.** A class reopened in a later file adds to it, and a
      method or top-level def defined again replaces the earlier one.
      Ruby does that when the later file loads; a closed world has every
      method from the start, so code that runs while an earlier file
      loads already sees the later definition. Suites never notice: tests
      run after every file has loaded.
    - **Constants.** One assigned in two files is a compile error (Ruby
      only warns, but the two values may not share a type).
    - **Names.** Files keep the names they were given in messages and
      `//line`, so failure locations read `testdata/test/x_test.rb:12`, as
      MRI's do (the one-file `Compile` passes a basename, as before).
      `$0` is the first file.
    - **Proof:** [testdata/multi](../testdata/multi) (`TestMulti` compares
      with MRI loading the same files) and the whole `testdata/test` suite.
    - **Groundwork** for `require_relative` in user code.
85. Dynamic dispatch (decision 32) grows with method definitions, not with
    classes × names. A wrapper whose body is the same on many classes (an
    inherited prelude method, `rbArity(...); return self.ToS()`, or a
    Kernel free func over `Self any`) is one `case <class IDs>:` arm in
    `rbDynName`, which asserts the method's Go signature
    (`interface{ ToS() String }`) and calls it. The class ID switch keeps
    dispatch nominal: only classes that reach the method take the arm,
    never a Go type that merely has a method of that shape. The same
    applies to the `respond_to?` and private markers: `rbHas_RespondsName`
    and `rbHas_PrivateName` switch on the class ID for struct classes
    instead of a marker method on each.
    - **Kept per class:** primitive (`@go_type`) classes, generic
      classes, a body that names its class (a metaclass's `new`), a
      user's own methods (`_Call` calls them by Go name, decision 79),
      and the names Go helpers assert for (`<=>` for rbCmp,
      `method_missing`, `respond_to_missing?`). A body shared by only one
      class stays a wrapper too.
    - **Why:** with computed `send` every name is dispatchable, and 27
      Kernel names on ~1650 classes made 44k identical wrappers: the
      whole `testdata/test` suite as one program was 946k lines of Go
      and a ~36 minute build (`-race -gcflags=-l`). It is now 393k lines
      and ~20 minutes, and dynamic_test alone went from 329k to 151k. Most
      of what is left is forwarders (one per inherited Kernel method per
      class, ~150k lines): the next step before TestMinitest is one build.
    - **Proof:** the dynamic dispatch checks in `testdata/test` (send,
      respond_to?, private methods, method_missing) and `testdata/run`.
86. A generic primitive's methods (`Array`, `Hash`, `Set`, `Range`,
    `Enumerator`, `Queue`: `@go_type` with `@rbs generic`) are free funcs,
    `Array_Push[E](self *Array[E], v E)`, as struct and module methods
    already are, and a call site names the free func with the receiver's
    type args (`Array_Push[FooI](a, NewFoo())`: Go infers a free func's
    type from every argument and won't widen `*Foo` to `FooI` on its own).
    Each also gets a one-line forwarder method, which the pruner keeps
    only when a kept interface declares the name (a module constraint's
    `Each`, a dynamic-dispatch `interface{ Size() Integer }`, `Op_eq`
    for rbEq), not on a bare selector match: Ruby's names (`size`,
    `first`, `join`) are on every class, so an unscoped match would keep
    Array's forwarder whenever a String is measured. Raw Go in the
    prelude (`%x{}`, `prelude/go`) therefore calls these by free func too
    (`Hash_Op_idxSet(h, k, v)`), and a hash literal is
    `Hash___Set[K, V](NewHash[K, V](), k, v)`. Non-generic primitives
    (`String`, `Integer`) keep plain Go methods.
    - **Why:** Go compiles every method of a generic type for every
      instantiation, used or not, both the shaped body and a wrapper
      (golang/go#70511, closed as not planned); a free generic func is
      compiled only where it is called. array_test's program had 42
      `Array[...]` instantiations × 93 methods: the Go compiler emitted
      54.8k functions and the linker kept 10.1k, with 77% of compile time
      in the backend. It now emits 39.7k, and the uncached build went from
      12.4s to 7.6s (25.4s to 15.4s with `-race`), same test output.
      Crystal's compiler has the same top cost (crystal#4864: `Array(T)`
      methods re-instantiated per T) and the same answer: only a called
      (type, method) pair is emitted. What remains per shape is the
      `Dyn*` wrappers and the interface-declared forwarders; emitting
      only the pairs the type checker resolved is the next step.
    - **Proof:** `testdata/test/array_test.rb`, `hash_test.rb`,
      `stdlib_test.rb` (Set, Queue) and `dynamic_test.rb` unchanged; the
      prelude's raw Go compiles in every example.
87. `respond_to?` markers (decision 85) are a class-ID switch for generic
    classes too, not a `_RespondsName()` method on each. A marker is never
    called, only asserted for, so the pruner kept it as a stub on every
    instantiation: in the whole `testdata/test` suite as one program,
    each of Array's 390 instantiations carried 146 marker stubs, and
    Hash, Set and Range as many again. Every generic class has a
    `_ClassID` (tuples report Array's, which is what Ruby answers for
    them), so one `case` covers every instantiation. Non-generic
    primitives (`String`, `Boolean`, whose `_ClassID` answers TrueClass
    or FalseClass) keep their one marker each.
    - **Why:** the suite as one program went from 513k to 372k compiled
      functions, and its `rb2go test testdata/test -race -gcflags=-l`
      from 202s to 144s.
    - **Proof:** `testdata/test/dynamic_test.rb` (`respond_to?` on
      untyped Arrays, Hashes and tuples) and the whole suite.
88. Test builds keep the Go build cache to what a prelude edit really
    changes. Go caches a compile by the package's source bytes, and each
    generated program's entry is 100–270 MB, so a run after a prelude edit
    was writing gigabytes: every `//line prelude/x.rb:N` shifts when a
    line is added above it, so a comment in `string.rb` changed all 105
    generated programs (1.5M lines) though none compiled differently.
    - **Stable prelude lines.** `TestRun`, `TestMinitest` and `TestMulti`
      set every prelude `//line` to `:1` before building
      (`stablePreludeLines`). A line shift then changes nothing; adding a
      method changes only the programs that reach it, plus the ones whose
      dispatch tables list every name (computed `send`; minitest's
      `assert_respond_to`, a leak to fix). Only the file name is
      observable at run time (`rbMtLocation` filters `prelude/` frames),
      so a prelude line in a Go panic trace reads `:1`; `rb2go build
      -work` keeps the real lines, as do the examples, whose lint
      correlates `//nolint` comments through them.
    - **No debug info.** Every test build passes `-gcflags=-dwarf=false`:
      DWARF was a quarter of each cache entry (array_test: 106 MB to 82
      MB), and panics and `runtime.Callers` read the pclntab, not DWARF.
      A repeated `-gcflags` replaces the earlier one, so `goBuild` composes
      one.
    - **Measured** (105 programs, 1.5M lines of Go): a comment or blank
      line in `string.rb` rebuilt 105 programs, now 0; a new method at the
      top of `string.rb` rebuilt 105, now 10 (the every-name programs, 52%
      of the lines); a body change inside `squeeze` rebuilt 4 before and
      after.
    - **What it doesn't do:** a program's entry is as big as its function
      count (array_test: 38k functions from 41k lines, 18k of them generic
      instantiations), which is decision 86's next step, not this one.
    - **Deterministic output, and the lint pass cached too** (2026-10-01).
      A full `go test ./...` took 937s, over Go's default 10m `-timeout`,
      with the examples built by 121s. Three causes. (1) `frameLabel`
      picked a `__helper`'s public name by map order (`__each_with_index_enum`
      was `each` or `each_with_index` per run), so the `rbFrameLabels`
      table, and every program that reads it (the ten every-name programs,
      75% of the lines), differed byte-wise run to run: no cache ever hit
      them. Now the longest public prefix wins. A sweep generates each of
      the 145 programs twice and compares. (2) `go vet` and golangci-lint
      load the module through `go list`, whose compile is cached by
      directory unless `-trimpath` is set, so the fresh `t.TempDir` paid a
      full recompile of 1.16M lines for export data every run (283s with
      golangci-lint's own cache warm; 670s and 10.8 GB cold), serially after
      the examples. `lintGenerated` now runs both under `GOFLAGS=-trimpath`:
      4s when nothing changed. (3) The top-level suites ran one after
      another; they are `t.Parallel()` now, so TestRun, TestMinitest and
      TestErrors overlap the lint pass (the global `-parallel` budget still
      bounds concurrent builds). Measured: 937s before; 180s on the first
      run after the fix (caches fill); 135s steady, of which TestErrors'
      492 compiles are 91s of overlapped wall and FuzzCompile's seeds 39s
      serial at the end. After a prelude edit, only the programs whose
      bytes change rebuild and re-lint; the every-name ten are ~30s of
      `go build -race` each and most of the lint CPU (staticcheck alone is
      27s on `64_observable`), which is decision 81's classes × names
      problem, not the harness's.
89. Named interfaces declare only what kept code selects. A class's
    interface (`FooI`, `Foo_MetaI`) declared every method the class has,
    inherited Kernel ones included, so the pruner (decision 49) kept a
    `panic("rb2go: pruned")` stub of each unselected method on every
    class and Go compiled them all: 114k of the 332k functions across the
    105 test programs were stubs (`_Ref`, `object_id`, `!=`, `nil?`,
    `to_json`, ...), and a cache entry's size follows its function count,
    not its lines. `sweep` now drops from a named interface each method
    whose name nothing kept selects; a stub survives only for a name an
    interface literal in kept code asserts at run time
    (`x.(interface{ _Ref() })`) or a `_` marker a kept interface declares
    (`_Foo()` is what makes `rescue Foo` and `is_a?` match through
    `FooI`; without it the interface would match every class).
    - **Why safe:** dropping an unselected method from an interface only
      widens its method set, nothing selects the name on any value, and
      the markers still decide membership. Generic forwarders (decision
      86) still key on `declared`, which named interfaces fill before
      they are slimmed.
    - **Measured** (105 programs): 332k → 242k functions, 114k → 24k
      stubs, 1.52M → 1.38M lines; cache entries (no DWARF, decision 88)
      array_test 82 → 66 MB, dynamic_test 274 → 161 MB, `05_word_count`
      6 → 3 MB. Test output unchanged.
    - *Amended:* the marker rule is `_X()` with X a declared type
      (`identity`), not every `_`-prefixed name: `_ClassOf()`, `_Kind()`
      and the `__Sleep*`/`__FreshObject` helpers were kept on every kept
      interface too, and `_ClassOf` alone pulled each exception's
      metaclass and the Module layer into `puts "hello"` (1199 → 1080
      lines). `RB2GO_PRUNE_WHY=1` prints, per kept declaration, the chain
      of visits that kept it, back to a statement of main: how these
      were found.
    - *Amended:* a named interface's unselected methods are parked, not
      visited, until something selects them (like a pending type-switch
      case): sweep would slim them away anyway, but their signatures had
      already kept their types (`_ClassOf() Exception_MetaI` kept the
      metaclass layer). And a selector on a std package (`os.Interrupt`,
      `sync.Mutex`) no longer names a class of ours: the unscoped match
      kept `Interrupt` and `Mutex` in every program. `puts "hello"` is
      772 lines (from 1080). A type switch left with only `default`
      becomes that body (`inlineDefaultOnly`, its guard kept as `_ = e`),
      and the blank lines dropped cases leave at a brace are tidied after
      printing (`tidyBraces`): the generated code still lints.
    - **`_Ref` too:** the marker was 11k of the 24k stubs left, an empty
      method on every pointer class so `rbObjToS`/`rbObjectID` could ask
      whether a value's address is its identity. It is now the
      `rbClassRefs` table by class ID (`rbIsRef`), as decision 85's
      markers are. A tuple reports Array's ID, so its `object_id` hashes
      it with `maphash.Comparable` instead of `rbHash`: a value hash
      either way. Measured: 242k → 231k functions, 24k → 13k stubs, but
      cache entries only 66 → 65 MB (array_test) and 161 → 159 MB
      (dynamic_test): an empty method costs far less than the average
      function, so the remaining size is in real bodies.
90. `Kernel#Integer(x, base = 0)` and `Kernel#Float(x)` (issue #3) are
    strict: a String must be a number from end to end bar surrounding
    whitespace, else `ArgumentError: invalid value for Integer(): "abc"`;
    `nil` is a `TypeError`. The argument's static class picks the
    overload (decision 12): `__Integer_string`, `__Integer_float`,
    `__Integer_integer` and Float's twins take their argument unboxed,
    and only an untyped or nilable one reaches the `(untyped)` base,
    which switches on its run-time class. A two-argument call on a
    non-String goes to `__Integer_2`, which raises MRI's `base specified
    for non string value` unless the value turns out to be a String.
    Past 64 bits is a `RangeError`, as `to_i` (decision 35). `Float()`
    of an out-of-range string is ±Infinity or 0 without MRI's warning.
    `exception: false` is not supported yet.
    ([example 67](../examples/67_strict_numbers/main.rb)).
91. `Kernel#catch(tag = Object.new) { |tag| }` / `Kernel#throw(tag, value
    = nil)` (issue #4). `throw` panics with an `rbThrow{tag, val}`, which,
    like `rbStop`, is not an exception: `rbWrapPanic` passes it through, so
    `rescue` (even `rescue Exception`) re-panics it and `ensure` runs;
    `catch` recovers it when the tag is identical (`rbIdentical`) and
    re-panics otherwise, so the innermost matching catch wins. A throw
    with no running catch for its tag raises `UncaughtThrowError`
    (`< ArgumentError`, with `tag`/`value`) where it is, as MRI does, so it
    is rescuable and exits 1 uncaught. *Revised:* the running tags belong
    to the goroutine's Fiber, else Thread, else Ractor, else the main
    thread (decision 104's maps), touched only by that goroutine: no lock,
    and a thread or fiber cannot throw to another's catch, as in MRI
    (they were one mutex-guarded list for all threads). `catch`
    returns `untyped`: the thrown value's type is known only at the throw.
    Two equal string literals share Go's backing bytes, so they match as
    tags where MRI's two objects would not (`rbNewStr`'s ponytail).
    - **RBS `bot`** is now read as a return type: a call to a `bot` method
      is `noreturn` like `raise` (it ends a statement list, so `x > 0 ? x
      : throw(:neg)` and a method whose last statement is a throw type-check
      as the other branch), and the Go call is followed by a `panic` Go
      can see. `terminates` and the unset-local walk recognise a
      receiverless `throw` by name, as they do `raise`.
    - **A void call where `untyped` is wanted** runs as a statement and
      yields `nil` (issue #28): `1.tap { work }` and a Thread block ending
      on a void call used to emit `return <void call>`. MRI returns what
      the method returns, usually nil; a void prelude method that returns
      something else in MRI (an iterator's receiver) reads as nil here.
      A void call where a typed value is wanted is still an error.
    ([example 68](../examples/68_catch_throw/main.rb)).
92. A prelude method on a `@go_type` class may carry `# @self T` (issue
    #5): the receiver must unify with `T`, which binds the method's own
    type parameters, and `self` inside it (and the Go free function's
    `self` parameter) has type `T`. So `Array[E]#transpose` declares
    `# @self Array[Array[U]]` / `[U] () -> Array[Array[U]]` and works on
    `*Array[*Array[U]]` directly. It needs type parameters, so it is always
    a free function and never a forwarder on `Array[E]`, which is what
    keeps Go's instantiation cycle away (decision 9). A receiver that does
    not fit is a compile error naming both types. When the method found
    by name does not fit (or has no `@self`), decision 12's overloads also
    try each `__<name>_*` sibling with an `@self` the receiver fits and
    that takes the call's arguments and block, first defined first.
    - `flatten(depth = -1)` on non-Array elements copies, or for untyped
      elements splices nested Arrays (tuples too) at run time.
      `__flatten_nested3` (`@self Array[Array[Array[U]]]`, no arguments)
      and `__flatten_nested` (`@self Array[Array[U]]`, `(?Integer)`)
      take typed nesting to `Array[U]`; after them an untyped `U` goes on
      splicing dynamically. `flatten(0)` of nested Arrays, and
      `flatten(n >= 2)` or a no-argument flatten past three typed levels,
      raise `NotImplementedError` rather than mistype the result. A tuple
      (`[[1, [2]], 3]`) has no flatten.
    - `to_h` is `@self Array[[K, V]]`; `__to_h_arrays` takes
      `Array[Array[U]]` to `Hash[U, U]`, checking each length with MRI's
      `wrong array length at i (expected 2, was n)`; `to_h { |x| [k, v] }`
      is `__to_h_block` on any element type.
    - `transpose` raises MRI's `IndexError: element size differs (n
      should be m)`.
    - A block wanted as `[K, V]` whose probe typed a same-typed `[a, b]`
      literal as `Array[X]` now unifies as `[X, X]` (`h.to_h { |k, v| [v,
      k] }` on a `Hash[Integer, Integer]` used to be a compile error).
    `sum` of Arrays is not supported.
    ([example 69](../examples/69_nested_arrays/main.rb)).
93. Typed minitest assertions (issue #29). The assertions stay `untyped`
    (decision 79), and each gains typed twins a call takes when its
    arguments allow, so `==`, `include?`, `empty?` and the operator are
    compiled calls and no by-name table (`rbRespondsByName`, the `rbDyn`
    dispatchers) is kept: a test file of `assert_includes`/`assert_empty`
    /`assert_operator` generates about 15k lines of Go, not 36–40k.
    - **`T` with `T?`.** Arguments that share a method type variable and
      are `X` and `X?` (or `nil`) bind it to `X?` before any is coerced
      (`optJoin` in `genArgs`), so `[T] (T, T)` takes `(3, h[:a])`. A
      variable bound from the receiver (a class's `E`) is never widened.
    - **`__<name>_same`** (decision 12's overloads): chosen when the first
      two arguments have one static type, `T` and `T?` or `nil` counting
      as `T?`, not `untyped`, and the twin's first parameter takes it:
      `assert_equal`/`refute_equal`/`assert_same`/`refute_same` as `[T]`,
      `assert_in_delta`/`refute_in_delta` for two Floats. Two different
      types (`assert_equal 1, 1.0`, a subclass) keep the untyped method,
      so a mismatch is still a failed assertion and never a conversion
      error.
    - **`__<name>_<class>`**: `assert_includes`/`refute_includes` on an
      Array, Hash, Set or String and `assert_empty`/`refute_empty` on the
      same call `include?`/`empty?` directly.
    - **Literal operators.** `assert_operator a, :<, b`,
      `assert_predicate a, :even?` and `assert_respond_to a, :m` (and the
      `refute_` forms) on a typed `a` are rewritten by the compiler
      (`mtLiteral`): `a` and `b` are evaluated once into temporaries,
      `a < b` is an ordinary call, and a `__assert_*_lit` twin builds
      MRI's message from it. A computed symbol, an untyped `a` or a block
      keeps the run-time send, with its dynamic-call warning.
    - Each twin counts the assertions its untyped original does (the
      `assert_respond_to` inside `assert_includes`, `assert_empty` and
      `assert_operator` counts one), and failure messages are MRI's,
      checked in `testdata/test/assertion_test.rb`.
    - `x.nil?` on a type variable now unboxes first: `T` may be `X?`, whose
      nil `*X` is a non-nil `any`.
94. File and Dir gaps from the roadmap (#1 Phase 0, #2). `Dir.glob` is
    `rbGlob`, not `filepath.Glob`: `{a,b}` alternatives expand first, in
    order and nested; then each `/`-segment matches with
    `filepath.Match` (`*`, `?`, `[set]`), a leading dot only by a
    pattern's leading dot; `**/` is any depth of non-hidden directories,
    a bare `**` is `*`, and a trailing `/` keeps directories only, shown
    with the slash. Each directory's entries are visited sorted, an
    entry before its subtree, which is the order MRI's `sort: true`
    default gives (`**/*` lists `a`, `a/1.rb`, `a/b`, ...; `**/*.rb`
    lists `a/...` before `top.rb`). A literal segment is a stat, not a
    listing. Not done: `File::FNM_*` flags, `base:`, an Array of
    patterns, `.`/`..` from `.*`. `Dir[*patterns]` concatenates globs;
    `Dir.each_child` (sorted, like `children`); `Dir.chdir(path) { }`
    returns the block's value and restores the directory in `ensure`
    (process-wide, as MRI's), and without a block returns 0.
    `File::SEPARATOR`/`PATH_SEPARATOR`/`ALT_SEPARATOR` (`nil`);
    `File.stat` is a `File::Stat` over one `os.Stat` (`size`, `mtime`,
    `file?`, `directory?`, `zero?`, `mode` with the file-type bits, from
    `syscall.Stat_t`); `File.mtime` is its `mtime`. A blockless
    `File.open` is `File.new`, through decision 12's no-block overload
    (`__open_enum`, the same hook `Dir.chdir` uses). `Kernel#sleep(secs)`
    rounds the seconds actually slept, as MRI, raises MRI's
    `ArgumentError` for a negative interval, and with no argument blocks
    forever (Go's deadlock detector ends a program with nothing else
    running, where MRI would hang).
    ([example 70](../examples/70_glob/main.rb)).
95. `ARGF` and `$stdout.sync` (#2). `Kernel#gets` is `ARGF.gets`: at the
    first read, an empty ARGV means stdin (named `-`); otherwise each file
    named in ARGV opens in turn, shifted out of ARGV as it opens (so a
    program may push paths onto ARGV before reading), and once they are
    spent `gets` is nil and `eof?` raises MRI's `IOError: closed stream`.
    A missing file raises `Errno::ENOENT` with MRI's `rb_sysopen`
    message. `ARGF` has `gets`, `read`, `readlines`, `each_line`,
    `filename` and `eof?`; `$<` stays a compile error (decision 61's fixed
    set). `STDIN.gets` still reads stdin, sharing its buffer with ARGF.
    `IO#sync` is true for STDERR and false for STDOUT until `sync = true`,
    which flushes and then flushes after every write, pipe or terminal, so
    stdout and stderr interleave as written.
    ([example 71](../examples/71_argf/main.rb)).
96. `Array#freeze` and `Hash#freeze`. Every prelude method that
    mutates one (`<<`, `[]=`, `push`, `pop`, `shift`, `unshift`,
    `insert`, `concat`, `delete*`, `clear`, `fill`, the `!` forms, Hash's
    `[]=`/`store`/`delete`/`delete_if`/`keep_if`/`clear`/`update`) first
    calls `rbCheckFrozen`, raising MRI's `FrozenError: can't modify
    frozen Array: [1, 2]` (`FrozenError < RuntimeError`). `dup` is a new,
    unfrozen object; `clone` keeping the flag is not done.
    *Revised:* the flag is per object, on the object. It was a global
    `sync.Map` of frozen pointers behind an atomic "anything frozen" flag,
    so one `X = [...].freeze` anywhere made every Array mutation in the
    program a map lookup (+33% on a map/select loop) and kept frozen
    objects alive forever. Now `Array` is `struct { s []E; frozen atomic.Bool }`
    (not a bare `[]E`: a slice has no room for a flag, and a side table
    keyed by pointer is the global again) and Hash's struct gained
    `frozen`, an `atomic.Bool` since a Thread may freeze what another
    reads. The check is one uncontended atomic load and inlines. Prelude Go reads
    the elements as `a.s`; the compiler builds literals through
    `arrayLit`. `rbFreeze`/`rbIsFrozen` remain for untyped callers
    (Ractor), over an `rbFreezable` interface. User objects, Struct and
    Set still have no `freeze` (a compile error), as before.
    Strings keep their own frozen set (literals and `String#freeze`). *Revised:* without a lock (decision 147's
    rule): the literals are a read-only map built at start, and strings
    `freeze` marks go in 256 shards of immutable maps, read with an
    atomic load and replaced by compare-and-swap.
    ([example 72](../examples/72_freeze/main.rb)).
97. `Kernel#system`, backticks and `$?` (#2), over `os/exec`. A command
    string with shell syntax (MRI's metacharacters, or a first word that
    is a shell reserved word or special built-in such as `exit`) runs as
    `/bin/sh -c`; otherwise, like several arguments, it is exec'd directly,
    so `system("echo", "$HOME")` passes `$HOME` through. stdout is flushed
    before every spawn, as MRI does, so a child's output lands in order.
    `system` returns true for exit 0, false for another status, and nil
    when the program could not run (`$?` then reports 127); a backtick
    (`` `cmd` ``, `%x(cmd)`, interpolation included) in user code compiles
    to `Kernel#__backtick`, returns the child's stdout, and raises MRI's
    `Errno::ENOENT: No such file or directory - cmd` for a missing
    program. In the prelude `%x{}` stays the Go escape hatch (decision
    16). `$?` is `Process::Status?` (Open3's class: `exitstatus`,
    `success?`, `pid`, `to_s`), one for the program where MRI's is
    per-thread. Not done: an env Hash or options, a signalled child's nil
    `exitstatus`. *(`exec`, `spawn` and `Process.wait`: decision 107.)*
    ([example 73](../examples/73_shell/main.rb)).
98. Operands run left to right. Some expressions hoist statements ahead
    of the Go expression that uses them (`x&.y`, `a || b` into a temp);
    when a later call argument or array element did, an earlier one
    already generated as a plain Go expression ran after it
    (`[log.push(2).size, log.last&.succ]` saw the push late). Now, when
    an operand's generation emits statements (not just `//line`s), the
    earlier operands' codes are evaluated into temporaries inserted before
    those statements (`pinBefore`/`pinExprs`, with the buffer spliced at
    the operand's start). Literals and nil are left alone. Covered: call
    arguments (positional and rest) and array literal elements; a
    receiver, hash literal pairs and interpolation parts are not yet.
99. Forwardable (#1) is compile-time codegen. `extend Forwardable` (an
    empty prelude module) and, in the class body, `def_delegators :@x,
    :a, :b`, `def_delegator :@x, :a, :alias` and `delegate [:a, :b] =>
    :@x` (also `instance_delegate`) are recorded while collecting; in
    `link`, once supers and includes are resolved and before signatures
    are, each becomes an ordinary def written out as Ruby and parsed:
    `#: (T) -> R` / `def a(a0) @x.a(a0) end`, with the target method's
    signature after substituting the accessor's type (`E` → `Integer`,
    `self` → the accessor's type, as Forwardable returns the target's
    value). A block is forwarded as `{ |b0| yield b0 }`, so a delegated
    `each` is an iterator and `include Enumerable` works over it. Each
    optional parameter adds a `__<name>_<n>` def taking that many
    arguments (decision 12's arity overloads), annotated per parameter
    with its return inferred, so an omitted argument stays omitted and
    the target's own default runs (`fetch(k)` raises, `fetch(k, 0)`
    doesn't). The accessor's type must be known at link time: a
    `# @rbs @x: T` declaration, a `#:` on an assignment to it in
    `initialize` (found by syntax), or a method with a declared return.
    The generated source is padded so its errors and `//line`s point at
    the `def_delegators` line. Method-set caches are cleared after the
    defs are added. Not done: an expression string accessor
    (`"@a.b"`). *(Constant and global accessors and SingleForwardable:
    decision 132.)*
    ([example 74](../examples/74_forwardable/main.rb)).
100. `Kernel#trap` / `Signal.trap` and `Process.kill` (#2). The signal
    goroutine of decision 60 now consults a table: a trapped signal runs
    its block there (MRI runs it in the main thread; here it runs beside
    it, so share state through a Queue or Mutex), `"IGNORE"` drops it, and
    `"DEFAULT"`/`"SYSTEM_DEFAULT"` restore decision 60's flush-and-die for
    INT/TERM or Go's default for others. Signals are named as MRI takes
    them (`"INT"`, `"SIGINT"`, `:INT`, a number); an unknown one is MRI's
    `ArgumentError: unsupported signal 'SIGFOO'`, KILL/STOP are
    `Errno::EINVAL`, and SEGV/BUS/ILL/FPE/VTALRM are reserved as in MRI.
    `trap("EXIT") { }` is an at_exit handler. An exception from a trap
    block ends the program like an uncaught one. `trap` returns the
    previous command (`"DEFAULT"` the first time); where MRI returns the
    previous Proc, this returns nil. A signal already queued when its
    trap changes is handled by the new setting. `Process.kill(sig, pid)`
    is `syscall.Kill` and returns 1. Interrupt/SignalException are still
    never raised (decision 60).
    *Amended:* the signal loop consults the trap table through
    `rbTrapHook`, which only `Kernel#trap` sets, so a program without
    `trap` carries neither the table nor `rbRunTrap` and what it reaches
    (decision 49): `puts "hello"` lost 160 lines.
    ([example 75](../examples/75_trap/main.rb)).
101. OptionParser (#1, Phase 3), over a Go engine (`prelude/go/optparse.go`,
    no `flag`). The one typing problem is `on`'s block, whose parameter
    MRI decides at run time; here the compiler decides it from `on`'s
    literal arguments (`optKind`, a decision 12 overload like `scan`'s):
    a switch string with no argument is a flag (`bool`, false for
    `--no-x`), `--name NAME`/`-n NAME`/`--name=NAME` a required argument,
    `[NAME]` an optional one, and a coercion class sets the type: none or
    `String` → `String`, `Integer` → `Integer` (MRI's `Integer()`, so
    `0x10` works), `Float`, `Array` → `Array[String]` split on commas; an
    optional argument makes it `T?` (nil when absent; no optional Array).
    The call goes to `__on_<kind>` (and `__on_tail_`/`__on_head_`); a
    non-literal switch string keeps the untyped `on`. Other coercions,
    completion lists and patterns are compile errors.
    - **Parsing** is MRI's permute mode: switches anywhere, operands
      returned in order, all after `--` untouched; `-abc` clusters,
      `-nVALUE`/`--name=VALUE` attached values, a required argument
      taking the next word even if it starts with `-`, an optional one
      only if it doesn't; long names and `--no-` forms complete by unique
      prefix, and a short letter with no switch completes against long
      names (`-h` is `--help`). `parse(argv)` returns the operands,
      `parse!(argv = ARGV)` also leaves them in `argv`, and a trailing
      `into: hash` stores each value under the long name's Symbol (a
      `Hash[Symbol, untyped]` or an unannotated `{}`).
    - **Errors** are MRI's classes under `OptionParser::ParseError <
      RuntimeError` with its messages: `invalid option: -z`, `missing
      argument: -n`, `invalid argument: --count=abc`, `needless argument:
      --extra=1`, `ambiguous option: --l`.
    - **Help** (`help`, `to_s`, `puts parser`) is Switch#summarize ported
      line for line: `banner` (default `Usage: <$0's basename> [options]`),
      a 4-space indent, a 32-column switch field, descriptions after one
      space, a switch field too wide on its own line, extra description
      lines aligned, `separator` lines as given, `on_head` first and
      `on_tail` last. `--help` (or any prefix, `-h`) prints it and exits
      0; `--version` prints `version=`'s value or MRI's `version unknown`.
    - Not done: `order!`/`permute!`, POSIXLY_CORRECT, `getopts`,
      completion lists, `Regexp` patterns, `accept`, `summarize` with a
      block, `on`'s long-description Hash, `OptionParser::Arguable`.
    ([example 76](../examples/76_optparse/main.rb)).
102. `rb2go web` (#2) is a local playground: Ruby on the left, the Go it
    compiles to on the right (recompiled 300 ms after typing stops), a Run
    button, the examples in a picker, and the source in the URL hash for
    sharing. It is a stdlib `net/http` server around `Compile`, not a
    static page: `Compile` builds for `js/wasm` unchanged, but measured
    ~5.6 s per compile there against ~0.7 s native (most of it
    `emitDynamic`'s walk over the prelude, not Prism), and Run needs the
    local `go` command anyway. Compiles are serialized (one user). `/run`
    builds without `-race`, runs in a temp dir with empty stdin, a 10 s
    wall clock and 1 MB per stream. Because `/run` executes what it is
    sent, the server binds to `127.0.0.1` by default and refuses a Host
    other than loopback or `-addr`'s (DNS rebinding), a foreign `Origin`,
    and a POST that isn't `application/json` (a "simple" cross-site POST
    gets no CORS preflight); `-addr` off loopback warns. The Go pane
    shows only the user's code unless "Show prelude" is ticked: the
    top-level decls whose last preceding `//line` is the user file, and
    of the top-level function the part from its first such directive
    (prelude constant setup runs before it), plus the plumbing the
    compiler generates for the user's types, found by name from those
    types (`X`, `XI`, `NewX`, `X_…` incl. the metaclass, `super_X_`, and
    methods on them). For that, a user class's struct and a user module's
    `M_Self` constraint carry a `//line` to their `class`/`module` line
    (they had none, so a Go error in one pointed at the prelude). Both panes are highlighted by a
    vendored highlight.js (`cmd/rb2go/highlight.min.js`, served from the
    binary so the page works offline); the Ruby editor is a transparent
    textarea over a highlighted mirror. Not done: a hosted wasm build (needs the
    `emitDynamic` cost fixed first), multi-file programs, stdin/args for
    Run. (`cmd/rb2go/web_test.go`.)
    *Amended:* a static build for hosting without a server:
    `cmd/rb2go-wasm` (`GOOS=js GOARCH=wasm`, `-ldflags=-s -w`) defines
    `rb2goCompile(src)`, returning `/compile`'s shape; the "your code"
    filter is `rb2go.UserCode`, shared by both. The wasm (12.5 MB, 3.2 MB
    gzipped) and its matching `wasm_exec.js` live in the public R2 bucket
    `ruby2go` under a content-hash prefix (stored gzipped with
    `Content-Encoding: gzip`, immutable cache, CORS GET from anywhere), so
    a static page on another host streams it. `rb2go web -static DIR
    -assets URL` writes that page: `index.html` sets `rb2goAssets`, runs
    the compiler in a Worker made from a Blob (a Worker's script must be
    same-origin; it `importScripts` wasm_exec.js from the assets),
    compiles only the latest edit while one is in flight, and turns Run
    into Copy Go. highlight.js is inlined into the page (140 KB, under
    Top Banana's 256 KB file cap): its lint fails a cross-origin
    `<script src>` and any `.js` outside `functions/`; `examples.json`
    and the wasm are fetched, which it doesn't lint. `scripts/deploy-web.sh` builds,
    uploads to R2 (`wrangler`), and deploys the page to the Top Banana
    site `$TOPBANANA_SLUG` (default `ruby2go`) through
    `scripts/topbanana.mjs`, which talks to Top Banana's MCP endpoint over
    JSON-RPC (it has no other upload API), signing in once by OAuth PKCE
    in the browser; a deploy is the whole site, so files an earlier one
    left are deleted. Live at https://ruby2go.apps.topbanana.dev. A
    compile there is under a second (decision 49's last amendment); the
    worker warms the compiler up at load so the first keystroke doesn't
    pay for it, and runs with the GC tuned for throughput (a compile
    allocates ~130 MB and is done in well under a second, so collecting
    mid-way only costs time).
103. `Ractor` (Ruby 4.0's API: `Ractor::Port`, `value`/`join`, no
    `yield`/`take`) is a goroutine with a port, in `prelude/ractor.rb` and
    `prelude/go/ractor.go`; `Ractor::Port` is an unbounded queue with a
    lock per port. A blocked `receive` or `Ractor.select(*ports)` parks on
    its own one-slot channel (`rbWaiter`), registered on every port it
    watches, so a `send` wakes exactly one receiver, as a channel would;
    `select` answers `[port, message]`. Not a channel outright: a port never
    blocks the sender, `select` over N ports is dynamic (`reflect.Select`
    is out, decision 82), and `closed?`/`ClosedError` need a flag a channel
    doesn't expose, the same reasons as decision 45's `Queue`. Checking
    for a message and parking happen under one lock (`park`), which is
    what rules out a lost wake-up; a select that finishes through one port
    leaves the others (`leave`) and passes on a wake that may have been
    spent on it. Only the creating ractor may `receive` from a port
    (`Ractor::Error: only allowed from the creator Ractor of this port`),
    checked through decision 104. MRI checks most of Ractor's
    rules at run time; rb2go checks them at compile time
    (`internal/compiler/ractor.go`):
    - **Isolation.** A `Ractor.new` block that reads or writes an outer
      local (Prism's `Depth` past the blocks nested in it), an instance
      variable, a global or a class variable is a compile error with MRI's
      message (`can not isolate a Proc because it accesses outer variables
      (x)`). Reading an unfrozen constant, MRI's one run-time-only
      `IsolationError`, is not checked: an MRI-valid program never does it,
      so rb2go is merely more permissive, as with the GVL (decision 26).
    - **Copying.** Constructor arguments and messages are deep-copied, as
      MRI copies non-shareable objects, through `rbCopier` (`_Copy(seen)`):
      generated on every struct class (`emitCopy`, next to `_Ivars`),
      written for `Array` and `Hash`, and raising MRI's errors for `Set`
      with unshareable members (`Ractor::Error: can not copy Set object.`),
      `Queue`/`SizedQueue` (`NoMethodError ... initialize_copy`) and
      `Thread` (`TypeError: allocator undefined for Thread`); a Proc
      argument is a compile error. Value types, class objects, ractors and
      ports pass as is, and so does a frozen Array/Hash (`rbIsFrozen`,
      decision 96), as MRI shares shareable objects. `seen` keeps identity
      inside one message (`[s, s]` arrives as two references to one copy, a
      cycle stays a cycle). The whole mechanism prunes away with
      `rbCopyDeep` when a program has no Ractor (decision 49). `move: true`
      is a plain send: the sender keeps a usable object where MRI leaves a
      `MovedObject`, since every method of a typed Go value cannot be
      proxied.
    - **Which ractor.** `Ractor.current`, `Ractor.receive`/`recv` and
      `Ractor.main?` are run-time lookups by goroutine id (decision 104):
      `start` registers its goroutine in `rbRactorOf`, `rbThreadRun`
      registers a thread under the ractor that started it, and anything
      unregistered is main. So `def helper = Ractor.receive` works as in
      MRI, from a helper method or from a thread inside the ractor.
      `Ractor.new` is an ordinary prelude call (`new`/`__new_1..3` by
      arity; a `name:` keyword literal is routed to `__new_named` by the
      compiler, since a Hash argument is a message); only the isolation
      check is otherwise the compiler's. *(The first cut resolved these lexically, with a hidden
      local per `Ractor.new` and a compile error in method bodies; it was
      replaced the same day.)*
    - **Errors.** The block's exception is kept and reported on stderr like
      a thread's (`rbThreadAbort`, shared with `rbThreadRun`); `value` and
      `join` raise `Ractor::RemoteError` ("thrown by remote Ractor.") with
      `ractor` and the original as `cause`. A ractor's port closes when its
      block ends, so a later `send` raises `Ractor::ClosedError` ("The port
      was already closed"), as does `receive` on a closed, drained port.
      `Ractor.count` is main plus the live ractors; `inspect` is
      `#<Ractor:#N running|terminated>` (MRI's own is timing-dependent, so
      no test prints it). `name:` is `__start_named` (a keyword Hash,
      decision 23); other keyword forms are unsupported.
    - **`Exception#cause`**, general and needed by `RemoteError`: `raise`
      lexically inside a rescue clause body compiles to
      `panic(rbWithCause(e, r_))`, which sets the cause to the exception
      being handled unless one is set or it is that exception itself (so
      `raise e` re-raises cleanly). A raise in a method the clause calls
      does not set it, where MRI's dynamic `$!` would.
    - **Not done:** `make_shareable`/`shareable?` (need `Object#freeze` on
      struct classes, decision 96 covers only Array/Hash/String),
      `Ractor[]`/`store_if_absent`, `shareable_proc`, `monitor`,
      `receive_if`. ([example 77](../examples/77_ractor/main.rb),
      `testdata/test/ractor_test.rb`, `testdata/errors/ractor.txtar`.)
104. Goroutines have an identity after all: `rbGoID()` parses the running
    goroutine's id from the first line of `runtime.Stack(buf, false)`
    (`goroutine N [running]:`), about a microsecond, into a 64-byte buffer.
    The format is not a documented API, but it has not changed in a
    decade and a wrong parse gives id 0, which only degrades to the old
    "unknown" behaviour (main). Registries are `sync.Map`s from id to
    `*Thread` (`rbThreadOf`, written by `rbThreadRun`) and to `*Ractor`
    (`rbRactorOf`, written by `Ractor#start` and inherited by the threads
    a ractor starts), each entry deleted when its goroutine ends. Only
    blocking or locking paths pay for the lookup: `Thread.current`,
    `Ractor.current`/`receive`/`main?`, `Port#receive`'s owner check,
    `Mutex#lock`/`unlock`/`try_lock`/`owned?` (the Mutex now stores the
    owner's id instead of a held flag, so relocking raises
    `ThreadError: deadlock; recursive locking` and a non-owner's unlock
    `Attempt to unlock a mutex which is locked by another thread/fiber`,
    both MRI's messages, where it used to deadlock; MRI also releases a
    dead thread's mutexes, which is not done). `Thread.current` on a goroutine no Thread
    started (main's, or a ractor's own) is `Thread.main`, which is what
    MRI answers inside a ractor too (each ractor has its own main thread).
    This supersedes the "goroutines have no identity" limits in decisions
    26, 45 and 103. Not done: `status` `"sleep"`, the per-thread inspect
    set of decision 26 *(thread-locals and `Monitor`: decision 108)*.
    (`testdata/test/stdlib_test.rb` `test_thread_identity`,
    `testdata/test/ractor_test.rb` `test_receive_in_method_and_thread`,
    `test_port_owner`.) Identity also gives `Thread#join`/`value` on the
    current thread MRI's `ThreadError: Target thread must not be current
    thread` instead of a goroutine waiting on itself, and `Ractor.select`
    over a port another ractor created raises `ClosedError`, as MRI's
    select does (`Port#receive` there is `Ractor::Error`, above).
105. The trace oracle (issue #31). A passing assertion was checked by
    rb2go's own `==`, so two identical reports never proved an operand
    right; decision 81 answered with authoring rules. Now `TestMinitest`
    runs both sides with `RB2GO_MT_TRACE=<file>` and diffs the files:
    each assertion a test calls appends `Class#test assertion operand…`,
    operands inspected (minitest's `mu_pp` minus its encoding note, which
    rb2go strings cannot carry), `assert_raises` after its block with the
    classes expected, then the class and message of what came. The MRI
    half is `testdata/mt_trace.rb`, preloaded with `-r` (its source keys
    the MRI cache): a module prepended to `Minitest::Assertions` with a
    depth counter, so an assertion another assertion calls
    (`assert_empty`'s `assert_respond_to`, `assert_raises`'s `pass`, and
    any assertion inside an `assert_raises` block) is not logged. rb2go's
    half is in `prelude/minitest.rb`: each public assertion and each of
    decision 93's typed twins logs once through `__mt_trace`, and what an
    assertion calls itself goes through `__mt_respond_to`/`__mt_predicate`
    /`__mt_in_delta`/`__mt_flunk`, which don't; `assert_raises` nests its
    block (`rbMtTraceDepth`). `assert_operator a, :pred` logs as
    `assert_predicate`, which it delegates to. The harness masks what
    never agrees across two processes: object addresses, `oid=`, Ractor
    and Port numbers. `TestTraceOracle` proves the hole is closed:
    `refute_equal 0, Process.pid` passes on both and is reported. Unset,
    nothing is written and an assertion costs one flag read. The oracle's
    first run found, and this decision fixed: a UTF-8 String's controls
    inspect as `\u0001` (a Symbol's stay `\x01`, MRI's US-ASCII form;
    `Integer#chr`'s US-ASCII result still inspects the UTF-8 way: an
    encoding on `String` is not planned, which settles #28);
    `Array#[]=` past the front raises `IndexError: index -3 too small for
    array; minimum: -2` instead of Go's bounds panic; a failed `<=>` in a
    sort names (earlier, later) and in `min`/`max`/`min_by`/`max_by` (best
    so far, candidate), as MRI's messages do (the sorts compare
    `-rbCmp(b, a)`: Go's insertion sort calls `cmp(later, earlier)`); JSON,
    Regexp (`end pattern with unmatched parenthesis: /a(b/`), Zlib
    (`incorrect header check`, `Zlib::BufError`) and `clock_gettime(99)`
    errors are worded as the json gem, Onigmo and zlib word them, each by
    a table from Go's error to MRI's text (an error with no twin keeps
    Go's); `Thread#inspect` and `Ractor#inspect` show the creating call
    site (`#<Thread:0x… main.rb:12 dead>`, `rbCallerLoc`: the nearest
    user-code frame) and `Ractor::Port#inspect` its owner and number. Not
    fixed, by design, and so not asserted through `assert_raises` in the
    suite: a `TypeError` from decision 20's argument check where MRI fails
    later and differently (`"abc".match?(5)`), and `min`/`max`'s order
    once `Float` is reopened (MRI's generic path names them the other way
    round). Also found: MRI numbers ractors and ports differently from
    rb2go, so they are masked, not matched.
106. `Exception#backtrace` (issue #30). A `rescue` that binds (`=> e`)
    records the Go stack's program counters on the exception
    (`rbCaptureBacktrace`, inside the deferred recover, where the
    panicking frames still are), once: a re-raise keeps the original
    frames. `backtrace` builds MRI 4.0's lines from them lazily
    (`prelude/go/backtrace.go`): `file:line:in 'K#m'`, `'K.s'`,
    `'Object#top_def'`, `'<main>'`, `'block in K#m'`, `'block (2 levels)
    in K#m'`, and a Ruby-written prelude method (`Array#each`,
    `Kernel#Integer`) as MRI shows a C method: its label at the caller's
    `file:line`. `set_backtrace` overrides; an exception never rescued
    with a binding answers nil, as MRI's never-raised one does. Labels
    come from `rbFrameLabels`, a table the compiler emits after pruning
    for the functions kept (`addFrameLabels`): a struct class's free
    funcs, direct methods, top-level defs, `main`, constructors as
    `Class#new`; an overload twin (`__Integer_string`, decision 12) is
    labelled as the public method it stands in for. Closures take their
    enclosing function's label with the Go name's suffixes
    (`.func1`, `.1`, `-range1`) counted as block levels, which is why the
    compiler's own func literals must be told apart: a
    begin/rescue/ensure body runs through `rbBegin(func() {...})`
    (`//go:noinline`, so its frame shows), so the literal is skipped and
    its line goes to the method's frame; a deferred rescue/ensure
    handler (called by the runtime, or by its literal at a normal
    return) counts as the method's own frame, and when it raises, the
    frames the old panic unwound (up to its literal) and the method's
    frame below are not shown twice. A fresh exception's frames start
    after the first `runtime.gopanic`; one that had already been through
    a rescue (`rbWrapPanic` marks it) starts after the deepest, so a
    re-raise through a non-binding rescue keeps its origin. Runtime
    frames, prelude Go helpers, forwarders and `Kernel#raise` are left
    out; one Ruby-level call of a prelude method is several Go frames
    (closures, what it calls), of which the outermost entry stays, so
    `each` is one `Array#each`, and `sort_by` is `Array#each` then
    `Enumerable#sort_by` as MRI's. Best effort past the first frame:
    `map` shows `Array#each` then `Enumerable#map` where MRI has one
    `Array#map`, since that is rb2go's implementation; class bodies show
    as `<main>`, not `<class:K>`. A rescue that does not bind pays
    nothing (a loop of `Integer(s) rescue nil` is unchanged at ~0.6µs a
    rescue); a binding one pays `runtime.Callers`, about a microsecond. A
    thread's exception is captured when the thread dies, so `join` and
    `value` re-raise it with the thread's frames. minitest prints them:
    `UnexpectedError#message` and `exception_details` list
    `Minitest.filter_backtrace(e.backtrace)`, the frames up to the first
    with a `Minitest::` label, as MRI drops its `lib/minitest` ones;
    `rbMtLocation` (decision 79) keeps its own walk. Uncaught exceptions
    still print `msg (Class)` alone (decision 11): MRI's full report with
    `from` lines would change every program's stderr, its own decision
    when wanted. `backtrace_locations` is not done. ([example
    78](../examples/78_backtrace/main.rb), `testdata/test/object_test.rb`
    `BacktraceTest`, `testdata/run/minitest_error.rb`.)
107. `Process.spawn`, `wait`, `wait2`, `waitpid` and `Kernel#exec` (#2),
    over `os/exec` with decision 97's command rule (several arguments
    exec directly; one string goes to `sh -c` when it has shell syntax).
    `spawn` starts the child on the program's standard streams, after
    flushing stdout as MRI does, and answers its pid; a goroutine waits
    on it, so it is reaped when `wait` asks. `wait(pid)` answers the pid
    and sets `$?` (`Process::Status`, decision 97); `wait` with no pid
    reaps a finished child first, else any, and raises `Errno::ECHILD:
    No child processes` with none left; `wait2` answers `[pid, status]`.
    `exec` flushes stdout and `syscall.Exec`s the command, so nothing
    after it runs; a missing program raises `Errno::ENOENT: No such file
    or directory - cmd` from both. Not done: `spawn` options (env Hash,
    `:out`/`:err` redirection, `chdir:`), `Process.detach`,
    `Process.kill` of a child by name, `wait` flags (`WNOHANG`).
    (`testdata/run/process_spawn.rb`.)
108. `Monitor`, `MonitorMixin` and thread-locals (#1's won't-do table,
    lifted by decision 104's goroutine identity). `Monitor` is a `Mutex`
    plus an entry count: the owning thread may `enter` again, `exit`
    releases at zero, `try_enter`, `synchronize`, the `mon_*` names,
    `mon_locked?`/`mon_owned?`, and `mon_check_owner` raising MRI's
    `ThreadError: current fiber not owner`. `new_cond` is a
    `MonitorMixin::ConditionVariable` over the monitor: `wait(timeout =
    nil)` releases it however deep it is entered (`wait_for_cond`), waits
    on decision 45's `rbCondVar`, and takes it back as deep;
    `wait_while`/`wait_until` loop; `signal`/`broadcast`. `MonitorMixin`
    gives an including (or extending) object those methods through a
    monitor kept in a Go-side table by identity (`rbMonitorFor`, decision
    74's pattern, since a module has no ivars); `mon_initialize` is a
    no-op. `Thread#[]`/`[]=`/`key?`/`keys` and `thread_variable_get`/
    `set`/`thread_variable?`/`thread_variables` are two `sync.Map`s on the
    Thread, keyed by Symbol or String (MRI takes either; `keys` answers
    Symbols, sorted, since a map has no order), a nil value deleting the
    key. MRI's `Thread#[]` is fiber-local; rb2go has no fibers, so the two
    maps differ only in name *(fibers came with decision 140; `Thread#[]`
    is still per thread)*. `require "monitor"` is a no-op (decision
    50). ([example 79](../examples/79_monitor/main.rb).)
    *Revised (decision 147):* MonitorMixin's monitor is the object's
    `@mon_data`, as in MRI, made on first use with a compare-and-swap on
    that field (two threads racing agree on one); the identity-keyed
    table and its global mutex are gone.
109. `$stdout = io` and `$stderr = io` (#1's minitest follow-ups), the
    first assignable globals (decision 61 amended). The compiler turns the
    assignment into `rbSetStdout(v)`/`rbSetStderr(v)`: `v` must answer
    `write` (`TypeError: $stdout must have write method, Integer given`,
    MRI's wording) and is kept as the stream's target, an `IO` on the same
    fd clearing it. `Kernel#puts`/`print`/`p`/`printf` write through
    `rbWriteOut`, which hands the text to the target when one is set (a
    program that never assigns pays one atomic load); `warn` goes through
    `$stderr`. Reading `$stdout` while one is set answers an `IO` bound
    to the target (`via`), so `$stdout.puts` reaches it, and `orig =
    $stdout; …; $stdout = orig` restores exactly orig's object; `STDOUT`
    itself keeps writing to the real stream, as in MRI. `$stdout.sync`,
    `flush`, `fileno` and `tty?` stay the real stream's. Not done: a
    `$stdout` that is not an IO at the type level (reading it is typed
    `IO`, so `$stdout.string` on an assigned StringIO does not compile;
    keep the StringIO in a local, as minitest does), `$stdin`
    assignment, and MRI's `$stdout` as a Ractor-local. minitest gains
    `capture_io` (a StringIO per stream, restored to STDOUT/STDERR in
    `ensure`), `assert_output(stdout = nil, stderr = nil) { }` (a String
    is `assert_equal`, a Regexp `assert_match`, each counted, messages
    `In stdout.`/`In stderr.` as MRI's) and `assert_silent`, logged by
    decision 105's oracle like the others.
    (`testdata/test/assertion_test.rb` `test_output`.)
110. Vendored signatures, and `WeakRef` (#1, Phase 3). A library the rbs
    gem ships no signatures for (`weakref`, `rexml`, `ostruct`, `prime`)
    gets a minimal `sig/<name>.rbs` in the repository, covering what the
    prelude ports; the harness passes `-I sig` to `rbs validate` and skips
    `-r name` for a library found there (`rbsLibraries`), so `require`
    lines stay valid RBS without weakening validation. `WeakRef[T]` is
    generic over the referent, where MRI's delegates every method through
    `method_missing`: callers go through `__getobj__`, typed `T`
    (`RefError: Invalid Reference - probably recycled` once collected),
    `weakref_alive?` is true or nil as MRI's, `__setobj__`. An object with
    identity (a struct class, `rbClassRefs`) is held through Go's weak
    pointer to its first byte: `T` is usually the class's interface, so
    the pointer is the data word of the value as `any` and the type word
    is kept to rebuild it on access (`rbWeakSet`/`rbWeakGet`,
    `prelude/go/weakref.go`, no `reflect`). A value (String, Integer) is
    held outright and always alive, as MRI's immediates are. `GC.start`
    (`runtime.GC`) clears a weak pointer whose object nothing reaches,
    deterministically enough that the example prints it; MRI's own
    `GC.start` collected it on every run tried. Not done: `WeakRef` as a
    `Delegator` (no `method_missing`), `ObjectSpace::WeakMap`, finalizers
    (`ObjectSpace.define_finalizer`). ([example
    80](../examples/80_weakref/main.rb).)
111. ERB (#1, Phase 3) is compiled with the program. `ERB.new(literal,
    trim_mode: "-")` takes a String or heredoc without interpolation (a
    computed template is a compile error: there is no `eval`), and the
    compiler ports `ERB::Compiler` (`internal/compiler/erb.go`: the
    TrimScanner's four modes `-`, `<>`, `>` and `%`, `<%#` comments,
    `<%%`/`%%>` literals, one output line per template line) to Ruby
    statements that append to an Array local and end with its `join`.
    Where `result(binding)`, `result` or `result_with_hash(k: v)` is
    called (`run` prints it), that Ruby is parsed as its own File, named
    like the caller's and padded so its lines are the template's lines
    in that file and its byte offsets lie past the file's (locals are
    keyed by offset), and generated by the caller's fctx into a Go block,
    so the template runs in the caller's scope: its locals are the
    caller's, `binding` is never a value. The caller's visible locals are
    declared first in the parse (`x = x`, which the compiler makes a
    no-op), so the template's parse reads them as locals rather than
    calls and its blocks assign the outer ones; a bare name that is a
    local anyway resolves as one inside a template. `result_with_hash`'s
    literal Hash becomes local assignments of the values' source text.
    The receiver may be the `ERB.new` call, a local assigned one, or a
    constant; the `ERB` object itself only marks the template. Errors in
    template code point at the template's line in the file. `ERB::Util`
    has `html_escape`/`h` (CGI's). Not done: a template held in an ivar
    or passed as an argument, `src`, `def_method`/`def_class`,
    `eoutvar:`, templates on the file's first line. ([example
    81](../examples/81_erb/main.rb), `testdata/test/erb_test.rb`,
    `testdata/errors/erb.txtar`.)
112. BigDecimal (#1, Phase 3) over `math/big`, with bigdecimal 4.1's
    rules taken from its C source (`prelude/go/bigdecimal.go`). A value
    is a sign, a mantissa `*big.Int` with trailing zeros stripped and a
    decimal exponent, or Infinity/NaN. `+`, `-` and `*` are exact (MRI
    sizes them to fit); `add`/`sub`/`mult(v, digits)` round to digits
    significant digits half-up. `/` and `quo` give
    `max(precision(a), precision(b)) + 16` significant digits, at least
    32 (`BigDecimal_div2`), rounded half-up with the remainder as the
    sticky digit; `div(v, digits)` gives digits; `div(v)` is the floored
    quotient as an Integer, `divmod` `[Integer, BigDecimal]` floored,
    `remainder` truncated, as `BigDecimal_DoDivmod`. Rounding (`round`,
    `floor`, `ceil`, `truncate`, `fix`, `frac`) is `VpMidRound` at a
    digit position, with all seven modes by Symbol (`:half_even`,
    `:banker`, `:ceiling`, …) or `ROUND_*` constant. `round` with no
    argument, or with n < 1, is an Integer, otherwise a BigDecimal, as
    MRI's: the compiler types `round(2)` with a literal n ≥ 1 as
    BigDecimal (`__round_digits`, a literal overload like decision
    101's), a computed n as `untyped`; with a mode it is always a
    BigDecimal. `to_s` is MRI's E form (`0.123e1`); `"F"`, a leading
    `+`/space and a group size (`"3F"`, `to_s(4)`) follow
    `VpToString`/`VpToFString`. `BigDecimal(str)` parses as `VpAlloc`
    (spaces, `_` between digits, `e`/`d` exponents; MRI's `invalid value
    for BigDecimal(): "x"`); `String#to_d` takes the longest valid
    prefix. A Float converts by its shortest round-trip digits capped at
    16 (`rb_float_convert_to_BigDecimal`), or digits significant digits;
    a Rational needs digits (`can't omit precision for a Rational.`),
    and as an operand is divided to the receiver's coerce precision.
    `**`/`power` with an Integer exponent is exact (a negative one is 1
    divided by the power, by the division rule); any other exponent and
    `sqrt` are computed with `big.Float` (ln and exp by series) past the
    target precision and rounded to `max(digits(x), digits(y), 16) + 16`
    (`sqrt`: the requested digits, or digits + 16): the same digits as
    MRI's Newton loops in every case tried, not a port of them.
    Comparisons take any number, `==` with a non-number is false and an
    order raises `comparison of BigDecimal with String failed`; NaN
    compares false and `<=>` is nil. `hash`/`eql?` agree for equal values
    (`1.0` and `1`), so BigDecimals work as Hash keys. An Integer or Float
    on the left (`2 * price`) goes through decision 12's class twins on
    Integer/Float (`__mul_big_decimal`). Integers past 64 bits from
    `to_i` raise `RangeError` (decision 35). `to_digits` keeps
    bigdecimal/util's own algorithm, sign quirk included (`-0.5` is
    `"0.5"`); `split` is an Array (tuples stop at 3). Not done:
    `BigDecimal.mode`/`limit`/`save_*` (global precision and exception
    modes: the defaults always apply), `BigMath`, `_dump`/`_load`,
    `nil.to_d`, Complex operands. ([example
    82](../examples/82_bigdecimal/main.rb), `testdata/test/bigdecimal_test.rb`,
    whose expected values are MRI's own output.)
113. `pp`, `pretty_inspect` and `PP` (#1, Phase 3). MRI's PrettyPrint
    (prettyprint.rb 0.2.0, Oppen's algorithm: Text and Breakable buffers,
    groups queued by depth, the outermost broken first) is ported line
    for line to Go (`prelude/go/pp.go`); pp.rb 0.6.3's per-class
    `pretty_print` methods become one type switch over the closed world:
    Array, Hash (Ruby 3.4's pairs: `key: v`, `"odd key": v` when the
    Symbol's inspect needs quotes, `k => v` otherwise), Set (`Set[...]`,
    Ruby 4.0's), Range, a multi-line String (its lines joined by ` +`),
    Struct and Data (`#<struct Point` with `x=` members, from two
    generated methods `__pp_kind`/`__pp_values` next to the generated
    `inspect`), an object whose `inspect` is the default (`pp_object`:
    its ivars sorted by name, each `@a=` group breakable), and anything
    else as its `inspect`. Cycles print `[...]`/`{...}`/`Set[...]`/
    `#<struct X:...>` by identity, as pp.rb's inspect keys. A class may
    define `pretty_print(q)` typed `(PP) -> void`: `PP` offers `text`,
    `breakable`, `comma_breakable`, `group(indent, open, close) { }`,
    `nest`, `pp`, `object_group` and `seplist(list) { |x| }`, and the
    type switch calls it before any default (`rbPPUser`). Width is
    `PP.width_for`'s: the terminal's (`TIOCGWINSZ`), else `$COLUMNS`,
    else 80, minus one, so 79 on a pipe; `pretty_inspect` has no
    terminal (MRI's String target). `Kernel#pp` writes through `$stdout`
    (decision 109) and returns its argument (the arguments as an Array),
    as MRI's. `PP.pp(obj, out = $stdout, width = nil)` writes to an IO or
    StringIO and returns a String target with the text appended (rb2go
    strings are values, so the caller uses the result). Not done:
    `PrettyPrint.format`/`singleline_format`, `PP.singleline_pp`,
    `pretty_print_inspect`, `pretty_print_cycle`/`pretty_print_instance_variables`
    overrides, `fill_breakable`, MatchData and File::Stat layouts.
    ([example 83](../examples/83_pp/main.rb), `testdata/test/pp_test.rb`,
    whose layouts are MRI's at each width.)
114. REXML (#1, Phase 3), the subset scripts reach for. A Go tokenizer
    (`prelude/go/rexml.go`) turns the source into events and keeps text
    and attribute values as written, entities intact, as REXML's tree
    parser keeps them raw; the DOM is Ruby (`prelude/rexml.rb`): `Child`,
    `Parent`, `Element`, `Document`, `Text` (`to_s` escaped, `value` with
    the five predefined entities and numeric references replaced; text
    made through the API escapes `& < > " '` on output, as
    `Text::normalize`), `CData`, `Comment`, `Instruction`, `XMLDecl`
    (written only when the source had one), `Attribute`/`Attributes`
    (insertion-ordered; `[]=` normalizes, `[]` unnormalizes) and
    `Elements` (1-based, or by XPath). Whitespace text before the root is
    kept as REXML keeps it, after the root dropped. `Document#write(out,
    indent)` and `Element#to_s` use the two formatters ported line for
    line: Default (attributes sorted by name) and Pretty (insertion
    order, whitespace-only text dropped, text squeezed and wrapped at 80
    columns less the indent, instructions unindented, `compact`), so
    output is MRI's byte for byte on everything tried. XPath is a
    subset: absolute and relative paths, `//`, `.`, `..`, `*`, names,
    `text()`, `node()`, `comment()`, `@name`, `@*`, and predicates `[n]`,
    `[last()]`, `[@a]`, `[@a='v']`, `[@a!='v']`, `[name]`, `[name='v']`,
    `[text()='v']`; anything else raises at run time naming the
    predicate. `XPath.first`/`match`/`each` return `untyped` (an Element,
    Text or Attribute); `Elements#[]`/`each`/`to_a` keep the Elements.
    Errors are `REXML::ParseException` with the first line of REXML's
    message (`Missing end tag for 'b' (got 'a')`, `Duplicate attribute
    "x"`, `Malformed XML: Content at the start of the document (got
    'x')`); REXML goes on with the line, position and unconsumed input,
    which rb2go does not. Output goes to `$stdout`, an IO or a StringIO
    (a String cannot be appended in place: use `to_s`). Signatures are
    vendored in `sig/rexml.rbs` (decision 110). Not done: DOCTYPE and
    DTDs (a ParseException), namespaces beyond prefixed names passing
    through, entity declarations, XPath functions and axes beyond those
    above, `REXML::Security` limits, SAX2/stream/pull parsers,
    `Transitive`, `attribute_quote` and `:raw`/whitespace contexts.
    ([example 84](../examples/84_rexml/main.rb), `testdata/test/rexml_test.rb`.)
115. `Regexp.new(pattern, options = nil)` and `Regexp.compile` (#34)
    build a Regexp from a String at run time the way an interpolated
    literal already is (decision 24): `rbRegexpFromValue` translates the
    Ruby source with the compiler's own translator, which every program
    carries (`rbRegexpDyn`), and a bad pattern raises `RegexpError` with
    Onigmo's wording where rb2go has a twin (decision 105). Options are
    MRI's: an Integer of `IGNORECASE | MULTILINE`, a String of `"m"`/`"i"`
    (`unknown regexp option: z` otherwise), `true` or any truthy value for
    `/i`, nil for none; a Regexp argument is returned as is. `EXTENDED`
    and `"x"` raise `NotImplementedError`, as `/x` literals are a compile
    error. `options` and `casefold?` read the flags back.
    (`testdata/test/rxjson_test.rb` `RxjsonRegexpNewTest`.)
116. `assert_raises(K) { }` and `_ { }.must_raise(K)` with exactly one
    class literal are typed `K` (#37), so `e.key` on a `KeyError`
    compiles. The assertions stay untyped and return `Exception` (decision
    93); the compiler wraps the call in a Go type assertion to `K`'s type
    (`narrowRaises`), which cannot fail, since the assertion has checked
    the exception is a `K`. Several classes, a computed class or
    `Exception` itself keep `Exception`. With it, `KeyError` gained MRI's
    `key` and `receiver` (`no key is available` when none was set),
    filled in by `Hash#fetch` and `ENV.fetch`. Narrowing exposed an
    older gap, now closed: a local assigned two sibling classes (`e =
    IOError…; e = ArgumentError…`, or `x = B.new; x = C.new`) was checked
    against its first type during analysis; it is now checked against the
    join so far, so it takes the nearest common superclass, as decision 14
    always intended. (`testdata/test/assertion_test.rb`
    `test_raises_is_typed`, `testdata/test/spec_test.rb`,
    `testdata/test/object_test.rb` `SiblingAssignTest`.)
117. CSV's `headers: true` and `converters:` (#36), chosen by the compiler
    from the literal options (`csvOverload`, a literal overload like
    decisions 101 and 112): `CSV.parse`/`read`/`foreach` with `headers:
    true` go to `__<name>_headers` and give a `CSV::Table` of `CSV::Row`s;
    with `converters:` alone, `__<name>_converted` gives rows of untyped
    fields. Iterator calls (`CSV.foreach(path, headers: true) { }`) take
    literal overloads too (`genIterCall`). `Row#[]` takes a header (the
    first, when headers repeat) or an index; `fetch` raises MRI's
    `KeyError`; `to_h`, `fields`, `headers`, `each`, `to_s` (the CSV
    line) and `inspect` (`#<CSV::Row "name":"Ada" ...>`) are MRI's. A
    short row is padded with nil and a long one gets nil headers, as MRI
    does. `Table#[]` is a Row for an Integer and a column's values for a
    header; `to_a` is the rows as Arrays with the header row first,
    which is why Table is not Enumerable in rb2go (Enumerable's `to_a`
    would be the Rows): it has `each`, `map`, `select`, `reject`, `find`,
    `each_with_index`, `first`, `count` and `rows` itself. `inspect` is
    MRI's `#<CSV::Table mode:col_or_row row_count:N>` followed by the
    text. Converters are MRI's: `:integer` is `Integer(field)`, `:float`
    `Float(field)`, each kept when it raises (so `" 08 "` becomes `8.0`
    and `"0x1A"` `26`), `:numeric` both, or an Array of them; nil stays
    nil and headers are not converted. Any other literal option key, any
    other converter, or a `headers:` that is not literally true or false
    is a compile error instead of being ignored, as decision 53 used to.
    Not done: `headers:` as an Array or String of names,
    `header_converters:`, `return_headers:`, `by_col`/`by_row` modes,
    custom converter lambdas, `CSV.parse_line` with headers. ([example
    85](../examples/85_csv_headers/main.rb), `testdata/test/stdlib_test.rb`
    `test_headers_and_converters`, `testdata/errors/regexp_json.txtar`.)
118. OpenStruct, SimpleDelegator and DelegateClass (#35).
    - **OpenStruct** is Ruby in the prelude over decision 31's
      `method_missing`: a field read is `method_missing(:name)` and a
      write `method_missing(:name=, v)`, both on an ordered
      `Hash[Symbol, untyped]`, so fields are untyped. `[]`/`[]=` (Symbol
      or String keys), `to_h`, `each_pair`, `dig`, `delete_field` (MRI's
      `NameError: no field 'x' in #<OpenStruct ...>`), `==`,
      `respond_to?` through `respond_to_missing?`, and MRI's inspect
      (`#<OpenStruct name="Ada", age=36>`). Signatures are vendored in
      `sig/ostruct.rbs` (decision 110).
    - **Delegation** is resolved at compile time, not by a run-time send:
      a method a `Delegator` subclass doesn't define (called on it, or
      receiverless in its body) compiles to the same call on its
      `__getobj__` (`delegateCall`). For `SimpleDelegator` the object is
      untyped, so the forwarded call is a dynamic call (decision 32) and
      warns as one; `to_s`, `inspect`, `==`, `!=`, `hash` and
      `respond_to?` forward too, as MRI's Delegator does, while `class`
      stays the wrapper's. `class W < DelegateClass(Foo)` makes the
      compiler generate, once per Foo, a `DelegateClass_Foo` Delegator
      whose `__getobj__` is typed Foo (`delegateClassSuper`), so W's
      forwarded calls are typed. Not done: forwarded calls with a block
      on SimpleDelegator (dynamic calls take no block), `method_missing`
      on the delegator itself overriding forwarding, `DelegateClass` of
      a generic or `@go_type` class beyond what its typed calls allow,
      and a dynamic call that needs an arity twin (`delegated.first` with
      no count reaches `Array#first(n)`, decision 12). WeakRef (decision
      110) keeps `__getobj__`. ([example 86](../examples/86_delegation/main.rb),
      `testdata/test/stdlib_test.rb` `DelegationTest`.)
119. The compiler's two `Type` interfaces (`rbs.Type`, the annotation as
    written; `compiler.Type`, the resolved type) are sealed sum types
    (#33): each has an unexported `isType()` only its members implement,
    and `//sumtype:decl` lets `gochecksumtype` (enabled in `.golangci.yml`
    only; generated programs have no sealed interfaces) require every
    type switch on them to list every member. `default:` does not count,
    so a member cannot be forgotten behind one; a deliberate fallthrough
    is an explicit arm listing the members that take it, with a comment
    saying why. Adding a member means visiting each switch the linter
    names. Turning it on listed 22 switches. 19 were deliberate and now
    say so; three were real gaps, each a valid Ruby program rb2go
    rejected:
    - a call on a void method's value (`log(x).nil?`, `.class`,
      `is_a?`) was "undefined method for void". The void call now runs
      as a statement and the call goes to nil (`voidAsNil`, in
      `genMethodCall`), as decision 91 already did for an untyped slot;
    - `.class` on a tuple was "undefined method"; it is `Array`
      (decision 22);
    - `x.is_a?(K)` on a generic `T` was "not supported"; it is checked at
      run time as for untyped. That exposed a fourth bug: a literal
      passed for a type variable (`string?("a")`) was an untyped Go
      constant, so Go inferred `string`, not `String`, and the check
      failed. `coerceArg` now names the literal's type.
    The issue also asked whether a sum-type linter could narrow `untyped`
    in generated programs. It cannot: `untyped` holds Go builtins
    (`int`, `string`, decision 35), which no sealed interface can
    contain, and calls on it go through interface assertions in the
    `rbDyn*` dispatchers, not switches. A Ruby `case` with no `else`
    is also legal and yields nil, so generated switches must not be
    exhaustive anyway. (`testdata/test/object_test.rb` `SumTypeGapsTest`.)
120. ruby/spec gaps (#49): syntax that rewrites into what rb2go already
    compiles, and core methods with a direct Go form. Each is checked
    against MRI in `testdata/test` (`*RubySpec*Test` classes) and shown in
    [example 87](../examples/87_ruby_spec_syntax/main.rb).
    - `case` with no subject is `if`/`elsif` (`caseAsIf`, one synthetic
      `IfNode` chain per case, cached so every pass sees the same nodes);
      `when a, b` is `a || b`.
    - `begin ... end while c` runs the body, then tests `c`, which may
      read the body's locals. Go's `continue` would skip the test, so when
      the body has a `next` for this loop it goes in its own `{ }` block
      followed by a `nextN:` label before the test, and `next` (or one
      re-issued after a begin wrapper) is `goto nextN`. The inner block
      means the forward goto jumps over no declarations; locals the test
      reads are hoisted as for any other block.
    - `begin/rescue/else/ensure`: the rescues guard the body only, in an
      inner `rbBegin` literal; `else` runs after it when the body finished
      (a flag), still under `ensure`. Its value (or a rescue's) is the
      result; the body's is dropped, as in MRI.
    - `alias new old` and `alias_method :new, :old` copy the method as it
      stands at that point (a second `Method` with the same body), so a
      later `def old` leaves `new` alone, as in MRI. An inherited method
      (superclass or included module) is copied in `link`, once supers
      resolve, from the ancestors only, so the class's own later `def old`
      does not change it; its body is recompiled with the class as
      owner. That copy needs a plain Ruby def: an inherited `%x{}`
      primitive (`alias old_inspect inspect` on Kernel's), a body with
      `super`, or a generic owner is a compile error (#52).
    - `{ a: }` reads `a`; `{ **h, k: v }` merges `h` in order
      (`rbHashSplat`), its key and value types joined with the literal's;
      `:"x#{y}"` is the interpolated String as a Symbol; `fail` is
      `raise`; `__LINE__` is the literal line; `__method__` is the
      enclosing def's name (nil at top level); `__dir__` is resolved at
      run time against the working directory, as MRI does at load.
    - Integer gained `& | ^ ~ << >> []`, `round`/`floor`/`ceil`/`truncate`
      with negative digits (half away from zero), `allbits?` and friends;
      `<<` past 64 bits raises `RangeError` (decision 35). `Math.asinh`,
      `acosh`, `atanh`, `log1p` and `expm1` are big-float like the rest
      (decision 43), and so are `erf`, `erfc`, `gamma` and `lgamma`.
    - `MatchData` keeps its subject and byte offsets, so `begin`, `end`,
      `offset`, `byteoffset`, `named_captures` and `m[:name]` work; with
      a duplicated group name, `m[:name]` is the last group that matched,
      as Onigmo picks.
    - `retry` sets a flag and leaves the rescue; the begin runs again in a
      `for` loop around its wrapper. With an `ensure` the begin is split
      in two, so `ensure` runs once, as MRI's does. A `retry` inside a
      closure block is a compile error.
    - `for x in coll` is `coll.each` as a Go range loop whose variables
      and body locals stay in the enclosing scope (Ruby's `for` opens no
      block), so `x` is readable after the loop.
    - `defined?` is answered at compile time: locals, constants, methods
      (on a receiver's static type; private ones only without a
      receiver), `super`, `self`, `nil`/`true`/`false`, assignments and
      other expressions. `defined?($g)` is "global-variable" for MRI's
      startup globals (MRI answers so whatever their value, even ones
      rb2go cannot read, like `$;`) and nil for any other: rb2go rejects
      assigning a user global (decision 109), so one is never set and no
      run-time flag is needed. `defined?(yield)` is "yield" in a def with
      a required block (or an iterator), nil without one or outside a
      def; with an optional block (decision 126) it is the one run-time
      answer, a `String?` read off the block local as `block_given?` is
      (nested in another expression there, `defined?(yield.x)`, it is a
      compile error). `defined?(X)` for a constant main may read before
      its assignment runs (guardConsts: BEGIN, a method called first) is a
      second run-time answer, the constant's set flag, so it is nil until
      the assignment as in MRI; a private constant's path is nil (#53).
      `defined?(@ivar)` depends on run-time state rb2go
      does not track and is a compile error. (`testdata/test/control_test.rb`
      `test_defined`, `test_defined_yield`.)
    - Multiple assignment takes `*rest`, `a, = xs` and nested targets; an
      Array gives each target its element or nil and `*rest` the middle
      (`rbMidSplat`, `rbTrailIdx`), a tuple splits statically. Block
      params take a trailing `*rest` too (`|a, *r|`). An `untyped` value
      (a target list or a block's several params) goes through `rbToAry`,
      MRI's implicit `to_ary`: an Array of any instantiation splits,
      anything else is the first target (#79; a user class's `to_ary` is
      not called).
    - `class << self` holds class methods (a bare `private` there makes
      them private); `module_function` (bare or with names),
      `protected` (checked at compile time: an explicit receiver is
      allowed only inside the owner's family), `private :x`,
      `private_class_method` (including `:new`) and `undef`,
      `undef_method`, `remove_method`. `private_constant` is enforced at
      compile time: a bare lexical `X` reads it, and any `M::X` path
      (a subclass's too) is a compile error where MRI raises NameError
      (#53). `self::X` in a class body or class method is the lexical
      class's X; a subclass with its own X makes it a compile error, since
      MRI picks by the receiver at run time.
    - `Class#superclass` and `#subclasses` come from per-class tables
      (newest subclass first, as MRI); `Integer.superclass` is Object,
      since Numeric is a module (decision 142). `Module#ancestors` and
      `included_modules` are decision 125.
    - `when *LIST` tests each element's `===`; `rescue *ERRORS` checks
      each class object at run time (`rbIsInstanceOf`), binding
      `=> e` as Exception.
    - `DATA` in the main file is one StringIO over the text after
      `__END__` (MRI's is a File at that offset; reads see the same).
    - `redo` jumps (`goto`) to a label that opens the loop body of a
      while, until, for or iterator loop, or a closure block's body after
      its parameters are bound, so the block reruns with the same
      arguments (a closure is its own Go function, #52). Inside a
      begin/rescue wrapper (a Go func literal the goto cannot leave) it
      is a compile error.
    - `BEGIN { }` bodies move to the front of their file's statements, in
      the top-level scope, so their locals are the file's. `END { }` is
      `at_exit` with its block, behind a package flag so it registers once
      however often the statement runs. `defined?(X)` inside BEGIN
      answers nil for an X the file assigns later, as MRI does (#53).
    - Post parameters (`def f(a, b = 1, c)`, `def f(a, *r, z)`) take the
      last arguments; with callee-side defaults `rbArgc` counts only the
      arguments before them. Anonymous `*`, `**` and `&` bind reserved
      locals that `g(*)`, `g(**)`, `g(&)` and `[*]` read. `def f(...)`
      takes the parameters and block of the one method it forwards to (a
      method of its own class, a top-level def, or `super(...)`), and its
      `(...)` call passes them on; an override keeps its parent's result,
      any other's is inferred. Leading parameters (`def f(tag, ...)`)
      come first, typed by `# @rbs` or from use (decision 146), and
      leading arguments (`g(x, ...)`) fill the target's first required
      parameters. The target may be on another receiver when its type is
      known before bodies are typed: a constant's class method, or an
      ivar declared `# @rbs @x: T` (an ivar typed only by discovery is
      not, since signatures resolve first). A target's defaults in another
      file (the prelude) forward when each is a plain literal; any other
      needs an annotation (#51). Literal splats `[*a, 1, *b]` concatenate.
    `Array#to_set` is decision 139's. Instance-variable reflection is decision 123, class variables
    decision 124.
121. Dynamic wrappers call Kernel's free func, not the class's forwarder
    (issue #50). A wrapper or shared arm for an inherited Kernel method
    used to call it through the receiver (`self.Sleep(...)`, or
    `recv.(interface{ Sleep(secs Float) Integer })`), and the pruner
    (decision 49) matches a selector or an interface literal by name on
    every reached class: with a computed `send` (decision 32) naming
    every method, ~17 Kernel names kept a one-line forwarder, each its own
    generic instantiation (`Kernel_Sleep[*Recorder]`), on all ~530 struct
    classes. `freeCall` now rewrites the call to `Kernel_Sleep[any](self,
    ...)`: one instantiation serves every class, and nothing names the
    forwarder. Only a universal owner: routing Module's methods through
    `Module_Name[ModuleI]` skipped the Go-level override a metaclass's
    synth `Name()` is (`klass.name` came back empty, `Integer === 1`
    false), and a module constraint (`Comparable_Self[Self]`) needs the
    concrete class; both keep the forwarder call. The pruner also no
    longer takes a std package's member as a selector (`time.Sleep` kept
    `Sleep` on every class, `os.Exit` kept `Exit`); it tells a package from
    a local of the same name (`net`) by go/parser's object resolution, as
    format.go's import scan does. Raw Go in `prelude/go` calls a generic
    primitive's free func (decision 86): `rbHTTPHeaderHash` called the
    `Op_idxSet` forwarder and built only while an unrelated interface
    happened to declare the name.
    - **Measured** (functions, `grep -c '^func '`): `64_observable`
      30,053 → 26,875 (176k → 167k lines), `30_dynamic_send` 29,947 →
      26,781, `32_resty_reflective` 17,052 → 15,792, `dynamic_test`
      38,153 → 33,973, `object_test` 38,894 → 37,688. The empty
      dispatchers #50 also lists (a name no reached class can be sent)
      are 129 of 1,062 and ~1k lines in `64_observable`: not worth code.
    - **What is left, and why:** these programs still keep ~12k per-class
      methods. `Inspect`, `ToS` and `Op_eq` are asserted by Go helpers
      (`rbInspect`, `rbEq`) and are the object protocol. `FrozenQ`,
      `Op_not`, `Exit`, `InstanceOfQ`, `EqualQ`, `Op_eqq` and Module's
      `PublicInstanceMethods` are kept by typed prelude code calling them
      on a concrete receiver (`Boolean.Op_not` in REXML, String's own
      `FrozenQ` in its wrapper): the pruner has no types, so a selector
      matches on every class. Scoping it would mean emitting such calls
      as method expressions (`Boolean.Op_not(x)`), which the pruner could
      read as that type's method alone; that changes the shape of every
      generated call on a primitive and is a separate decision. The
      metaclass tables (`_Consts`, `_Methods`, `_Descendants`, ... × 318)
      are Module reflection, reached because a computed send names
      `const_get` and friends. The dispatchers themselves (1,062 names ×
      dispatcher + `respond_to?` + marker) are the cost of `send` with a
      name nothing bounds.
122. Calls on a statically known receiver are method expressions, so the
    pruner can scope them (issue #50). `x.Upcase()` with `x` a String is
    emitted `String.Upcase(x)`; a call on a struct-typed value, or on
    `self` inside a struct class's method (`Self` constrained by `FooI`),
    is `FooI.Bar(x)`; `klass.new` on a class object is
    `(*Foo_Meta).New(Foo_class, ...)`, since `new` is not in the
    metaclass's interface. The pruner (decision 49) used to take every
    selector by name on every reached class: `Boolean.Op_not` in REXML
    kept `Op_not` on 531 classes, String's own `frozen?` wrapper kept
    `FrozenQ` on 539. A method expression selects the method on that type
    alone: on a struct, on the implementers of an interface (the types
    with its `_Foo` marker, own or through the embedded `super_Foo_`), and
    on the struct a subclass promotes it from (a call through
    `Minitest_ResultI` reaches `AssertionsSet` on `Minitest_Runnable`).
    A name selected on an ancestor's interface stays declared in every
    descendant's, so a `Self` constrained by `SignalExceptionI` still
    satisfies `ExceptionI` in a super call, and a plain selector
    (raw Go in `prelude/go`, `self.x` on a module's `Self`, which has no
    method expressions) still selects unscoped. A struct's Dyn wrapper
    body keeps the plain form (`plainCalls`), since decision 85's shared
    arms and decision 121's `freeCall` read it textually.
    - **Measured** (functions): hello world 91 → 78; `58_logger` 719 →
      633; `array_test` 7,837 → 6,144; `string_test` 14,349 → 10,904;
      `object_test` 37,688 → 25,622; `dynamic_test` 33,973 → 31,209;
      `64_observable` 26,875 → 24,774. The computed-send programs move
      least: with every name dispatchable, the dispatchers' interface
      literals (asserted, decision 89) and Module reflection keep most of
      what they keep.
    - **Why method expressions and not a side table:** the pruner reads
      the emitted Go, and a method expression is the one call form that
      carries the receiver's static type in the syntax, lint-clean and
      without comments to correlate. The generated code reads
      `Integer.ToS(i)` where it read `i.ToS()` (README sample updated).
123. Instance-variable reflection (#49) reads the tables the closed world
    already has. `instance_variables` lists what `inspect` lists (decision
    82's `_Ivars`): a non-optional ivar that is still nil reads as never
    assigned and is left out, while a `T?` ivar is listed even before its
    first write, where MRI would omit it. `instance_variable_get` returns
    the value untyped (nil for an ivar the class lacks);
    `instance_variable_set` goes through a generated `_IvarSet` that
    converts the value to the field's type, raising TypeError when it
    cannot. A class cannot gain an ivar at run time, so setting one it
    lacks is a NameError. `remove_instance_variable` is not supported.
    (`testdata/test/object_test.rb` `ObjectRubySpecIvarTest`.)
124. Class variables (#49) are package variables. `@@x = v` in a class or
    module body declares one, typed and initialized like a constant
    (`#: T` annotates it) where the body runs; methods, class methods and
    subclasses read and assign it through the class's ancestors, so a
    subclass shares its parent's, as in MRI. A second assignment in the
    same body is a compile error. One first assigned in a method (`@@n =
    0 #: Integer`, `@@tags ||= [] #: Array[String]`; a literal value needs
    no annotation) is declared on the method's class with a set flag:
    main does not initialize it, and a read before any assignment ran is
    MRI's NameError, "uninitialized class variable @@n in C" (#52).
    *(Revised: that was a compile error.)* (`testdata/test/object_test.rb`
    `ObjectRubySpecClassVarTest`.)
125. Kernel and BasicObject have class objects (#49), like every other
    class and module, so `Kernel` and `BasicObject` are values, print
    their names, and `singleton(Kernel)` is a type. They still take no
    class methods. `Module#ancestors` reads a generated per-class table:
    the class, its modules (last included first, each with its own), then
    its superclass's, through Object, Kernel and BasicObject; a module an
    ancestor already includes keeps only that place, as MRI skips
    re-including it. `included_modules` is its modules. rb2go's own
    prelude modules (File's IOReadable and IOWritable) are left out, as
    MRI has none (decision 139). *(Revised: the table listed them.)*
    `Kernel.puts` and the other module functions are decision 139's.
    (`testdata/test/object_test.rb`
    `ObjectRubySpecAncestorsTest`.) A top-level `include M` is Object's,
    as MRI's main object includes into Object: M's methods are on every
    object and in every ancestors list. *(Revised: it was a dynamic call
    that raised NoMethodError when run.)*
    (`testdata/run/object_top_include.rb`.)
126. An optional block (`?{ ... }`) in a def is a `Proc?` local (the
    `&blk` name, or a hidden one for `yield`), so the type system guards
    it rather than a nil func crashing at run time: `blk.call` where the
    block may be missing is a compile error (`yield` there was one too,
    until decision 132 made it MRI's run-time LocalJumpError); `block_given?`, `if blk` and
    `return x unless block_given?` narrow it to present, as any optional
    value narrows; `blk&.call` answers nil without one. `block_given?` is
    a constant true in a def with a required block and false in one
    without a block. (`testdata/test/control_test.rb`
    `ControlOptionalBlockTest`, `testdata/errors/control.txtar`.)
127. ruby/spec runs under rb2go through `cmd/rubyspec`, one minitest
    program per spec directory, compiled in the runner's process (Prism
    and the prelude parse once). `cmd/rubyspec/mspec.rb` is mspec's
    expectations over minitest (`x.should == y`, `x.should.equal?(y)`,
    `-> { }.should.raise(E)`); the runner makes static what mspec decides
    at run time: `context` is `describe`, version and platform guards are
    decided for MRI 4.0 on this machine (kept bodies are unwrapped,
    others removed), `it_behaves_like` inlines the shared describe with
    `@method`/`@object` substituted, `require_relative`d fixtures load
    first, and a lambda's `.should` becomes `ProcExpectation.new(...)`
    (a Proc has no methods, decision 47). `rb2go.CompileTestsSkipping`
    dry-emits every user `test_` method and spec form (`before`, `let`)
    after type inference and gives one that fails `skip "rb2go: <error>"`
    for a body, so one compile reports every unsupported example; errors
    elsewhere (a fixture method, a top-level statement) and `go build`
    errors (mapped back by `//line`) are cut from the source by the
    runner, which compiles again. Rewrites keep line numbers: removed
    text keeps its newlines. A crash ends a whole run, so the test
    minitest `-v` named last is excluded by name and the same binary
    runs again; a run silent for 3 s is a hung test, sent SIGQUIT so a
    nameless one is found in the goroutine dump. The report counts pass, fail, error and
    skip per directory, and the commonest unsupported reasons, which is
    the work list.
128. Exception classes rb2go never raises exist so `rescue` clauses
    naming them compile (#43): `SystemStackError` (Go's stack overflow
    is a fatal runtime error `recover` cannot catch, so infinite
    recursion exits with Go's `goroutine stack exceeds` message and
    status 2; counting call depth in every function would tax every
    call to report a bug), `LoadError` (an unknown `require` is a no-op,
    decision 78; a `require_relative` of a missing file is a compile
    error, decision 130), `SyntaxError` (no `eval`),
    `SecurityError`, `EncodingError` (its subclasses are raised since
    decision 136), and `LocalJumpError` (raised since decision 132 by a
    `yield` whose optional block is missing). *Revised (#56):* a Hash or Array
    holding itself no longer reaches that overflow through `==`, `eql?`
    or `hash`: a paired guard (`rbRecurseEnter`, beside inspect's) answers
    a re-entered pair as equal and hashes it as a constant, as MRI's
    `rb_exec_recursive_paired` does. *Revised:* no global set or lock
    (a process-wide lock serializes threads like MRI's GVL, which
    compiling to Go exists to avoid). Each top-level `inspect`, `==`,
    `eql?` or `hash` owns an `rbSeen`, passed down through the
    containers' `_inspect_rec`/`_eq_rec`/`_eql_rec`/`_hash_rec` and made
    only when a nested container is reached, so a flat Array allocates
    nothing. Objects whose `inspect` is Kernel's carry it into their
    ivars (a generated `rbKernelInspect` table by class ID), so a
    parent/child cycle prints `...` as in MRI. Known difference: a cycle
    through a user-defined `inspect` or `==` starts a fresh set there and
    is not caught (MRI's set is per thread; Go has no goroutine-locals).
129. `Warning` and `Kernel#warn(*msgs, uplevel:, category:)` (#44).
    `warn` builds one string as MRI's `rb_warn_m`: messages flatten
    (`warn []` prints nothing), each gets a newline unless it has one,
    and the whole goes to `Warning.warn(str, category:)`, which writes it
    to `$stderr` as is. A `category:` whose switch is off prints nothing;
    the switches start at MRI 4.0's (`:deprecated` false, `:experimental`
    true, `:performance` and `:strict_unused_block` false), a Hash in the
    prelude, and an unknown category raises `ArgumentError: unknown
    category: x`. minitest's `autorun` turns `:deprecated` on, as MRI's
    does. A user `module Warning; def self.warn` replaces the prelude's
    (the closed world sees it, so every `warn` reaches it) but needs its
    own `#: (String, ?category: Symbol?) -> nil`: an unannotated
    redefinition inherits from an ancestor, here `Kernel#warn`, not from
    the method it replaces, and `category:` is always passed (MRI checks
    the override's arity; rb2go would need two call shapes). `super`
    from it is not supported (MRI's reaches `Warning#warn`, rb2go's
    `Warning` has only the singleton). `uplevel: n` is the n-th
    user-code frame above `warn`'s caller from the Go stack
    (`rbCallerLocN`, as decision 106 labels frames): `file:line:
    warning: `, the file as `//line` names it (the path rb2go was given
    relative to the module, where MRI shows the path as run), and a
    level past the stack drops the location as MRI does. A block counts
    as a frame only where it is a Go closure. Category Strings
    (`category: "deprecated"`, which MRI converts) and `$VERBOSE = nil`
    silencing are not done. (`testdata/test/stdlib_test.rb`
    `WarningTest`, `testdata/run/io_warning_hook.rb`.)
130. `require_relative` in user code loads the file at compile time,
    building on decision 84's closed world of several files:
    - **Resolution.** The path is joined to the requiring file's
      directory, `.rb` added unless present, and keyed by real path
      (symlinks resolved), as MRI's loaded features are. A file already
      loaded, by an earlier `require_relative` or as one of the given
      files, is a no-op, which also ends require cycles. MRI does not
      list the main script as loaded, so a library requiring `main.rb`
      back would run it twice there; here it is a no-op.
    - **Run order.** A required file's top level runs where its
      `require_relative` stands, as a Go block nested in the requirer's
      (a `loadFile` marker in the statement list), so output before and
      after the require interleaves as in MRI, and its locals stay its
      own. Its code is generated once, before its requirer's (it comes
      later in load order), and spliced into every pass of the
      requirer's analysis.
    - **Names.** A required file's `__FILE__`, `//line` and messages
      use its real path, as MRI's do; `$0` stays the first file as
      given, so `__FILE__ == $0` works in both.
    - **Static only.** The argument must be a string literal and the
      call a top-level statement of its file (not inside `if`, `begin`,
      a def or a class body): a closed world cannot load a file
      conditionally. Otherwise, or for a missing file (`cannot load such
      file -- path`, MRI's `LoadError` message), it is a compile error.
      Before this, a nested `require_relative` compiled to nothing.
    - **Where files come from.** `Source.Path` (the path on disk;
      `Compile` passes the name it was given, `CompileFiles` each
      `Name`), read with `os.ReadFile`. The wasm playground has no file
      system, so its picker leaves out examples that `require_relative`.
    - `cmd/rubyspec` keeps its own fixture loading (decision 127): it
      rewrites fixtures before compiling, so the compiler must not read
      them from disk.
    - Not done: `load`, `autoload`, and top-level `return` in a
      required file. `require` through `-I` is decision 131.
    ([example 89](../examples/89_require_relative/main.rb),
    [testdata/multi/require](../testdata/multi/require),
    `testdata/errors/require.txtar`.)
131. `-I dir` (repeatable, on `build`, `run`, `gen` and `test`; the
    `loadPath` variadic of `Compile`/`CompileFiles`) is `ruby -I`: a
    user file's top-level `require "x"` searches each directory in order
    for `x.rb` and loads the first found as decision 130's
    `require_relative` does (real-path key, once, run where required),
    so the two share one loaded set. The `-I` directories come before the
    prelude's require hooks (decision 78), as MRI puts `-I` ahead of the
    standard library on `$LOAD_PATH`; a name found in neither stays a
    no-op. A `require` nested in `if`/`begin`/a def is a compile error
    only when it names a file on the path: `begin; require "x"; rescue
    LoadError; end` around a library still compiles. `$LOAD_PATH` and
    `$:` themselves are not modeled: a program that pushes onto them at
    run time needs `-I` instead. The test harness's `# load_path: dir`
    directive (relative to the file) passes the same directories to
    `ruby -I` and the compile, and `rbs validate` skips `-r` for a
    `require` it finds there. Examples' cached MRI output is keyed on
    every `.rb` under the example's directory, not just `main.rb`.
    ([example 90](../examples/90_load_path/main.rb),
    `testdata/errors/require.txtar`.)
132. The rest of #43 and #44.
    - **`yield` with a missing optional block** raises MRI's
      `LocalJumpError: no block given (yield)` at run time, amending
      decision 126 (where it was a compile error) and 128 (where
      LocalJumpError was never raised). A guard was a stricter rule than
      Ruby's: code whose logic guarantees the block (`yield` on a path
      only taken when one was passed) is valid Ruby. The arguments are
      evaluated first, as MRI does, then a nil check panics, and the call
      after it sees the block narrowed. `blk.call` on a `Proc?` stays a
      compile error (MRI's is a NoMethodError on nil, ordinary nil
      typing), as does `yield` in a method whose signature has no block.
    - **`IO#readline(chomp: true)`** is written once in `IOReadable`
      (IO, `$stdin`, File) and in StringIO; `chomp` is `String#chomp`,
      which removes `\r\n` like MRI's.
    - **SingleForwardable** is decision 99's synthesis with the defs
      added to the class object (the metaclass) instead of the class. In
      a class or module body, `extend SingleForwardable` enables
      `def_single_delegator(s)`/`single_delegate` (a compile error
      without it) and makes `def_delegator(s)`/`delegate` define class
      methods; when both modules are extended, those follow whichever was
      extended last, since MRI's are the most recently extended module's.
      `def_instance_delegator(s)`/`instance_delegate` always define
      instance methods. Accessors, for both modules: an ivar (for a
      class object's, declared with `# @rbs self.@x: T`, rbs-inline's
      class-ivar form, now read for any class, or `@x = ... #: T` in a
      class method), a method (a class method for SingleForwardable),
      `$stdin`/`$stdout`/`$stderr` (the only globals holding objects,
      decision 61), or a constant. MRI evaluates the accessor inside
      Forwardable, so a constant resolves from the top level only
      (`:"Outer::LIMITS"`, `:Math`); a name only the class body's lexical
      scope sees is a compile error, where MRI's NameError comes at call
      time. Accessors and names may be Strings as MRI allows. Also new
      for both: a target's arity overloads (decision 12's
      `__first_0` beside `first(n)`) are delegated too, so a delegated
      `first` takes 0 or 1 arguments. Not done: `obj.extend
      SingleForwardable` on an object at run time (a closed world defines
      no methods at run time; `extend` with a receiver is a compile
      error), `class << self` forms, and expression accessors
      (`"@a.b"`).
    - **ThreadGroup** is a mark on each Thread (`atomic.Pointer`, nil
      for Default) over decision 104's registry: a thread starts in its
      creator's group, `list` filters Thread.list (live threads, main
      first, creation order), and a thread keeps its group after it ends,
      as MRI 4.0's does. `add` raises MRI's `can't move to/from the
      enclosed thread group`; frozen groups are not modeled.
    (`testdata/test/control_test.rb` `test_yield_without_block`,
    `testdata/run/io_eof.rb`, `testdata/test/object_test.rb`
    `SingleForwardableTest`, `testdata/errors/objects.txtar`,
    `testdata/test/stdlib_test.rb` `ThreadGroupTest`.)

133. `DateTime` (issue #45) is a real subclass of `Date`, so `Date`
    dropped `@go_type` and became a struct class (a `@go_type` class
    cannot be subclassed; decision 27's `Net::HTTPResponse` precedent).
    - **State.** `Date` holds MRI's four numbers as Integer ivars: `jd`
      (the civil day in the object's own offset), `df`/`sf` (seconds and
      nanoseconds into that day) and `of` (UTC offset in seconds). A plain
      Date has `of = 0` and `df = sf = 0`, so all of `Date`'s readers
      work unchanged on a DateTime. `Date + 0.5` keeps a day fraction, as
      MRI's (its `inspect` shows `43200s`, `==` sees it), which
      decision 41's `Date` could not. Comparison, `==`, `eql?`, `hash`
      and `-` use the instant (UTC seconds, then ns), so
      `Date.new(2024, 1, 1) == DateTime.new(2024, 1, 1)` and they hash
      alike, as MRI's; `===` compares local days. `d - d2` is an exact
      Rational of days for any mix of Date and DateTime
      (`__minus_date`/`__minus_date_time`, decision 12); `+`/`-` take
      Integer, Float (exact `big.Rat` of the double) or Rational days,
      floored to the nanosecond; `>>`/`<<` move the local civil date and
      keep the time and offset. Before 1582 decision 41's proleptic
      Gregorian calendar still applies (`DateTime.new`'s jd is 38, not 0).
    - **Return types.** Each of `Date`'s methods that answers a Date has
      a `DateTime` override typed `-> DateTime` (decision 8's renamed
      override plus adapter), so `(dt + 1).hour` type-checks and a
      Date-typed DateTime still dispatches to it. Range iteration looks
      for `Succ() E`, which the renamed override does not provide, so
      `range.go` also accepts an `rbSuccAny` method, which `*DateTime`
      implements. `strftime`'s default is `nil` filled in by the callee
      (`__strftime_default`), since a literal default is filled in at the
      call site and a Date-typed DateTime would get `"%F"`.
    - **Arguments.** `DateTime.new`/`civil`/`jd` take Integer
      year..minute. Seconds and the offset are `untyped` (Time's zone
      precedent, decision 39): seconds Integer, Float or Rational; the
      offset a fraction of a day (Integer, Float, Rational, rounded to
      the second) or a String: `Z`/`UTC`/`GMT`/`UT`, `[GMT|UTC]±H`,
      `±HH`, `±HHMM`, `±HH:MM[:SS]`, a military letter (Time's
      table), or a fixed table of common
      abbreviations (`EST`, `PDT`, `JST`, `CET`, …; MRI knows more, and
      long names like `Eastern`). As MRI, an unreadable offset or one
      past a day is silently 0. Negative hour/minute/second wrap, `24:00`
      is the next day, anything else out of range is `Date::Error
      "invalid date"`. `DateTime.today` raises MRI's `NoMethodError`
      (MRI undefines it).
    - **Parsing.** `parse` is decision 41's date shapes followed by an
      optional `T`/space time (`H:MM[:SS[.frac]]`, `am`/`pm`) and zone;
      `iso8601`/`xmlschema` read extended and basic calendar forms;
      `rfc3339`, `httpdate` (RFC 1123 only, not RFC 850/asctime),
      `rfc2822`/`rfc822` and `jisx0301` (era date plus optional time;
      no era letter is Heisei, as MRI, for `Date.jisx0301` too) are fixed regexps, each raising
      `Date::Error "invalid date"` on a mismatch. `strptime`'s default
      is MRI's `%FT%T%z`; it adds `%H %k %I %l %M %S %L %N %p %P %z %Z
      %s %Q %a %A` and `%T %R %X %r %c %+` to Date's directives.
    - **Formatting.** `strftime` renders a Go time in a fixed zone
      named like the offset, so `%Z` is `+09:00` (Date's too, now) and
      `%Q` is added; `%::z`/`%:::z` were added to the shared
      `rbStrftime` (Time gets them too). `to_s`, `inspect` (MRI's
      `((jd j,s s,ns n),±of s,2299161j)` in UTC), `iso8601`/
      `xmlschema`/`rfc3339(n)` and `jisx0301(n)` with `n` fraction
      digits, `httpdate` (UTC) and `rfc2822` follow MRI.
    - **Conversions.** `to_time` keeps the offset (a fixed-zone Time,
      printed `+0000` for offset 0, as MRI), `to_date` drops the time,
      `Date#to_datetime` and `Time#to_datetime` (nanoseconds and offset
      kept) build DateTimes; `new_offset(of = 0)` is the same instant.
    - **Not done.** `ajd`/`amjd`, `commercial`/`ordinal` constructors,
      `_parse`/`_strptime`, `deconstruct_keys`, and the calendar-reform
      `start` argument (compile errors as undefined methods or arity).
      `Date#day_fraction` is `(0/1)` where MRI answers Integer `0` for a
      Date without a fraction. A dynamic (`untyped`) `-` between two
      dates, like Time's, does not reach decision 12's overloads and
      raises TypeError.
    ([example 94](../examples/94_datetime/main.rb),
    `testdata/test/date_test.rb`, `testdata/errors/dates.txtar`.)

134. open-uri (#46) is a prelude wrapper over `Net::HTTP` (decision 27)
    plus one compile-time step. MRI's `URI.open(name, *rest)` takes one
    trailing options Hash mixing request headers (String keys) and
    options (Symbol keys); a closed world cannot type that Hash, so
    `genOpenURI` (`internal/compiler/openuri.go`) splits it at the call:
    String-keyed pairs become a `Hash[String, String]` of headers,
    Symbol-keyed ones keywords of `OpenURI::OpenOptions.new`, and the
    call goes to `OpenURI.__open_name[_block]` (a String name) or
    `URI::Generic#__open[_block]`/`#__read` (a URI, including
    `URI.open(uri)`). Supported options are `read_timeout`,
    `open_timeout`, `redirect`, `max_redirects`,
    `http_basic_authentication`, `progress_proc`, `content_length_proc`
    and `ssl_verify_mode`; any other (`proxy:`, `encoding:`,
    `ssl_ca_cert:`, ...) or a `**splat` is a compile error, as are an
    Integer mode, extra arguments, `URI#read` with a mode, and a literal
    mode other than `r`/`rb`(`:enc`) (writing only makes sense for a
    local file: use `File.open`). A non-literal write mode raises MRI's
    `ArgumentError` for a URL. A Hash *variable* is taken as headers (a
    Symbol-keyed one is a compile error naming that, not a type mismatch).
    `Kernel#open` of a URL is not added: Ruby 3 removed it.
    - **The IO is a real `StringIO`.** MRI returns a `StringIO`
      extended with `OpenURI::Meta`; `extend` is run-time, so instead
      `StringIO`'s Go struct carries a nil-able `oum` (status, final
      URI, header fields) and `StringIO` gains `status`, `base_uri`,
      `meta`, `metas`, `content_type`, `charset` (with its optional
      block), `content_encoding` and `last_modified`. On a StringIO
      open-uri did not make they raise MRI's `NoMethodError`, but
      `respond_to?(:status)` is statically true for every StringIO, and
      `OpenURI::Meta` is not defined (naming it is a compile error rather
      than a wrong `is_a?` answer). `f.class`, `is_a?(StringIO)` and
      passing `f` where a `StringIO` is expected match MRI. MRI's switch
      to a `Tempfile` above 10 KB is not modeled. `content_type`/
      `charset` parse with `mime.ParseMediaType` (lowercase type,
      charset downcased, `application/octet-stream` and nil on a missing
      or malformed header, including a bare `text` with no subtype,
      `utf-8` for `text/*` without one; a repeated parameter, which MRI
      takes first-wins, is treated as malformed); header
      order in `meta`/`metas` is sorted, as `Net::HTTPResponse`'s is
      (Go's `http.Header` is a map). `URI#read` returns a plain `String`:
      MRI extends it with `Meta` too, but `String` is a Go `string`, so
      `.status` on it is a compile error.
    - **Redirects** follow `OpenURI.open_loop`/`open_http`: 301, 302,
      303, 307 and 308 follow (other non-2xx raise `HTTPError` with
      `"404 Not Found"` and the body as `io`), a relative `Location` is
      merged, `redirect: false` raises `HTTPRedirect` (`uri` is the
      target), a scheme change is allowed only http→https/ftp or
      ftp→http(s) (else `RuntimeError` `redirection forbidden: a -> b`),
      `http_basic_authentication` is dropped after the first hop, a URI
      seen twice is `HTTP redirection loop: <uri>` (the first URI is not
      in the set, as in MRI), and more than `max_redirects` (64) hops is
      `TooManyRedirects`. `base_uri` is the final URI and nil on an
      error's `io`. `progress_proc` is called once with the body size
      (the body is read whole; MRI calls it per chunk), only for a 2xx.
      A URL with userinfo raises MRI's `ArgumentError`. Not done:
      proxies, FTP (raises `NotImplementedError`), encodings from the
      mode, HTTPS against a test server (no TLS WEBrick here).
    - **Names that are not URLs** (no `scheme://`) are read whole into
      a `StringIO` with no metadata, so `URI.open` keeps one return type;
      MRI returns the `File` (`f.class` differs). Write modes there raise
      `NotImplementedError`.
    - `Net::HTTPResponse` gained `to_hash`/`get_fields` (repeated
      headers kept apart, for `metas`), `Net::HTTP#verify_mode=`
      (`OpenSSL::SSL::VERIFY_NONE` sets `InsecureSkipVerify`; only the
      two constants exist, no `OpenSSL` beyond them). A lambda literal
      where a `Proc?` is expected now takes its parameter types from the
      `Proc`, as it does where a `Proc` is. `URI.parse` keeps an empty
      authority (`file:///x` has host `""` and prints `file:///x`, as
      MRI's), which the forbidden-redirect message showed.
    ([example 95](../examples/95_open_uri/main.rb),
    `testdata/test/net_test.rb`, `testdata/errors/open_uri.txtar`.)

135. `socket` (#42) is MRI's class tree over Go's `net`: `BasicSocket`
    (with `IPSocket` < it, `TCPSocket`/`UDPSocket` < `IPSocket`,
    `TCPServer` < `TCPSocket`, `UNIXSocket` < it, `UNIXServer` <
    `UNIXSocket`, `Socket` < it), `Addrinfo`, `SocketError` and
    `Socket::ResolutionError`.
    - **Shape.** The socket classes are plain struct classes, so
      subclassing and `is_a?` work, each holding one `@h`, a
      `BasicSocket::Handle__` `@go_type` over a `net.Conn` or
      `net.Listener` plus a `bufio.Reader`. `BasicSocket` is no `IO`
      subclass, since `IO` is a `@go_type` (decision 62): it includes
      `IOWritable`/`IOReadable` as `File` does, so `puts`/`print`/
      `printf`/`each_line`/`readlines` are the shared ones. `accept`
      and `pair` build their objects in Go (`__wrap`), not through
      `initialize`. `Addrinfo` is a `@go_type` value (family, address,
      port or path, socktype, protocol, and the name MRI shows in
      parentheses when the host or service was not numeric).
    - **Real descriptors.** TCP connects and listens through
      `net.Dialer`/`net.Listen`. A `UDPSocket` is an unbound
      `socket(2)` wrapped by `net.FilePacketConn`, and `UNIXSocket.pair`
      a `socketpair(2)` wrapped by `net.FileConn`, so `bind`, `connect`,
      `send`/`sendto` (with flags), `recv`/`recvfrom` (with flags, e.g.
      `MSG_PEEK`), `setsockopt` and `getsockname`/`getpeername` are the
      kernel's own calls on the descriptor (`SyscallConn`): `addr` on an
      unbound UDP socket is `0.0.0.0:0`, sending on a connected one with
      a host is the kernel's `EISCONN`, and `setsockopt` takes any
      Integer level/option with `true`/`false`, an Integer or a packed
      String (nothing is ignored). Symbol levels and options
      (`:SOCKET, :REUSEADDR`) are not taken: a union parameter would be
      `untyped`, so they are a type error. `accept_nonblock` is one
      `accept(2)` inside `Control` (a listener's `RawConn.Read` is
      `EINVAL`): `IO::EAGAINWaitReadable` (< `Errno::EAGAIN`, including
      `IO::WaitReadable`) with MRI's message when nothing is pending; the
      `exception: false` form (a `TCPSocket | :wait_readable` union) is
      not built. `UNIXServer` keeps its socket file on close, as MRI
      (Go's listener would unlink it). `listen(n)` answers 0 without a
      second `listen(2)`: Go's listener already listens.
    - **Reads.** `gets`, `read`, `read(n)` (nil at EOF), `readpartial`
      (at most one `read(2)`, `EOFError` at EOF), `readline`, `eof?` go
      through the buffered reader. `recv` reads what the reader already
      holds first so a stream stays in order (MRI raises `recv for
      buffered IO` there; a `MSG_PEEK` leaves those bytes in place),
      and answers `nil` once a
      stream's peer has closed (MRI 3.3+; the rbs gem still says
      `String`). `recvfrom` is defined on `TCPSocket` and `UDPSocket`
      rather than `IPSocket`, since an override cannot narrow a result
      type: a stream's is `[String, nil]` and `nil` at EOF, as MRI's, a
      datagram's always carries the sender's address array, so UDP code
      needs no nil checks. Writes go straight to the descriptor, so `sync` is
      always true; `send` answers the bytes `sendmsg(2)` took, which on
      a full stream buffer is fewer than given, as MRI's. `close_read`/`close_write` are `shutdown(2)`, the
      socket closing once both are; reading or writing a closed half is
      MRI's `IOError` (`not opened for reading`, `closed stream`).
    - **Errors.** An errno becomes its `Errno::` class (`ECONNREFUSED`,
      `EADDRINUSE`, `EADDRNOTAVAIL`, `EPIPE`, `ECONNRESET`,
      `ECONNABORTED`, `EAGAIN`, `ENOTCONN`, `EISCONN`, `EDESTADDRREQ`,
      `ETIMEDOUT`, `EHOSTUNREACH`, `ENETUNREACH`, `EAFNOSUPPORT`, plus
      the file ones; others `SystemCallError`), its message Go's strerror
      text capitalized as libc's, then MRI's detail: `connect(2) for
      "host" port N` (`Socket.tcp`'s `connect(2) for host:N`), `bind(2)
      for ...`, `sendto(2) for ...`, `send(2)`, a Unix socket's
      `connect(2) for path` (MRI says `connect(2)` for `UNIXServer.new`'s
      bind too). A failed lookup is `Socket::ResolutionError` <
      `SocketError`, `getaddrinfo(3): ` from socket constructors and
      `getaddrinfo: ` from `Addrinfo`/`Socket.getaddrinfo`, then the
      platform's `EAI_NONAME` text: `nodename nor servname provided, or
      not known` on macOS and the BSDs, `Name or service not known` on
      Linux. A `connect_timeout:` that expires is `IO::TimeoutError`
      (< `IOError`) `user specified timeout for host:port`. Errno numbers
      are the platform's (`SystemCallError#errno`), but the classes have
      no `Errno` constant (`Errno::EAGAIN::Errno` is undefined).
    - **Lookups.** `Addrinfo.getaddrinfo`/`Socket.getaddrinfo` resolve
      with Go's resolver (a numeric host is not looked up, and a
      numeric `::ffff:a.b.c.d` stays IPv6; `""` and `"<any>"` are
      `0.0.0.0`, `"<broadcast>"` `255.255.255.255`, as MRI's
      `host_str`, though `""` shows no `()` in `inspect`) and repeat
      each address per socket type, stream/TCP, datagram/UDP, raw, as
      `getaddrinfo(3)` does without hints, filtered by family and
      socktype (Integers, or `:INET`/`"AF_INET6"`/`:STREAM`...); a named
      service is `net.LookupPort`. A hostname's address order is the
      resolver's, which may differ from MRI's. `Socket.ip_address_list`
      walks `net.Interfaces`, link-local IPv6 addresses carrying their
      zone. `Addrinfo#to_s`/`to_sockaddr` packs this platform's struct
      sockaddr (BSD's length byte on macOS); it is a plain String, so
      its `inspect` shows `\u0000` where MRI's binary String shows
      `\x00` (rb2go has no encodings).
    - **Constants** (`AF_*`, `PF_*`, `SOCK_*`, `SOL_SOCKET`, `SO_*`,
      `IPPROTO_*`, `TCP_NODELAY`, `SHUT_*`, `MSG_PEEK`/`MSG_OOB`,
      `SOMAXCONN`, `INADDR_ANY`) come from Go's `syscall` for the
      platform the program is built on, as MRI's come from its headers
      (`AF_INET6` is 30 on macOS, 10 on Linux). No `Socket::Constants`
      module.
    - **Compiler changes it needed.** RBS tuples go to 7 elements, not
      3 (`addr`'s `[String, Integer, String, String]` and
      `Socket.getaddrinfo`'s rows). A class that defines `send` shadows
      `Kernel#send` (`UDPSocket#send(msg, flags, host, port)`), as in
      MRI; this replaced Ractor's special case. A struct class's `new`
      honours decision 12's `self.__new_<n>` when `initialize` cannot
      take the call's argument count (`TCPServer.new(port)`).
    - **Not built.** `Socket.new` with `bind`/`connect` on packed
      sockaddr Strings is a compile error naming the classes to use, as
      is `.new` on any socket class without an `initialize`
      (`BasicSocket`, `IPSocket`, a user subclass of `Socket`), whose
      object would have no handle;
      `SOCKSSocket` and `IO.select` do not exist (uninitialized constant,
      undefined method). `Addrinfo.new(sockaddr)`, `Socket.unix`,
      `Socket.tcp_server_loop` and friends, `recvmsg`/`sendmsg`,
      `send_io`/`recv_io` and `getsockopt` are undefined. Reverse lookup
      (`addr(true)`) is not done: the host slot is the address, as with
      MRI's default `do_not_reverse_lookup`. Windows: the issue asked for
      a build-tag `unsupported` there, but the prelude already uses
      Unix-only `syscall` APIs (`Stat_t`, `Getrusage`, and now
      `Socketpair`), so generated programs are Unix-only as a whole and
      no tag is added.
    ([example 91](../examples/91_socket/main.rb),
    `testdata/test/socket_test.rb`, `testdata/errors/socket.txtar`.)

136. Encoding without a tag on String (#48). Decision 105 stands:
    Strings carry no encoding, every String is UTF-8 bytes, and what MRI
    keeps in the tag rb2go either derives or leaves out.
    - **Derived `encoding`.** `String#encoding` is `Encoding::UTF_8` when
      the bytes are valid UTF-8 and `Encoding::ASCII_8BIT` otherwise. So a
      binary String whose bytes happen to be valid UTF-8 (`"é".b`,
      `force_encoding("BINARY")`, `File.binread` of a UTF-8 file) reports
      UTF-8 where MRI says ASCII-8BIT and inspects as text where MRI shows
      `"\xC3\xA9"`; a UTF-8 literal with bad bytes (`"\xff"`) reports
      ASCII-8BIT where MRI says UTF-8; and `encode`'s result reports by the
      same rule, not as the target encoding (`"a".encode("UTF-16LE")` is
      UTF-8 here). `b` and `force_encoding` return the bytes unchanged
      (`force_encoding` still checks the name). Anything that would need
      the tag is one of these documented differences.
    - **The bytes are UTF-8 everywhere else.** `valid_encoding?`, `scrub`
      and `encode` without a source encoding read the bytes as UTF-8, the
      common case of a String MRI tags UTF-8 (a literal, `File.read`,
      `gets`). Invalid UTF-8 is cut into MRI's maximal invalid subparts
      (`"\xe3\x81"` is one bad sequence, `"\xff\xfe"` two), not Go's byte at
      a time, for `scrub`, its block, `encode(invalid: :replace)` and the
      error bytes.
    - **Ten encodings.** `Encoding` has UTF-8, ASCII-8BIT, US-ASCII,
      ISO-8859-1, UTF-16LE/BE, UTF-32LE/BE and the dummies UTF-16 and
      UTF-32, each with MRI's names, aliases and alias constants (`BINARY`,
      `ASCII`, `UCS_2BE`, ...), `inspect`, `ascii_compatible?` and
      `dummy?`; `list`/`name_list` hold only these, in MRI's order.
      `find` is case-insensitive; `find("internal")` with no
      default_internal raises (MRI returns nil, and an `Encoding?` result
      would make every `find` optional). `default_external=`/
      `default_internal=` are stored: `encode` with no target encodes to
      default_internal, as MRI's does, and IO never converts to either.
      `compatible?` is MRI's `rb_enc_compatible` over derived encodings.
    - **Compile-time names.** A string literal naming an encoding MRI has
      and rb2go lacks (`"Shift_JIS"`; the compiler holds MRI 4.0's
      `Encoding.name_list`) in `encode`, `force_encoding`, `Integer#chr`,
      `set_encoding`, `Encoding.find`, `default_external=`/
      `default_internal=` or a `File.open`/`File.new`/`CSV.open` mode is a
      compile error naming the ten; a name MRI lacks too stays MRI's run-time
      error. A non-literal name rb2go lacks fails at run time where MRI
      would succeed: `ArgumentError: unknown encoding name` from `find`,
      `ConverterNotFoundError` from `encode`.
    - **`encode(to, from, invalid:, undef:, replace:, xml:,
      universal_newline:, crlf_newline:, cr_newline:)`** converts through
      UTF-8 as MRI's converter path does, so its errors are MRI's: `U+00E9
      from UTF-8 to US-ASCII` for one step, `U+20AC to ISO-8859-1 in
      conversion from UTF-16BE to UTF-8 to ISO-8859-1` for two, `"\xFF" on
      UTF-8`, `incomplete "\xE3\x81" on UTF-8`, `"\xE3\x81" followed by "b"
      on UTF-8`, with the exceptions' `source_encoding(_name)`,
      `destination_encoding(_name)`, `error_char`, `error_bytes`,
      `readagain_bytes` and `incomplete_input?`. UTF-16/32 are decoded by
      MRI's byte tries: the first byte that cannot continue a character
      ends it, a fault inside the first unit takes the whole unit (or what
      is left), a later one keeps whole units and reads the rest again. A
      dummy UTF-16/UTF-32 source needs a BOM, unit by unit until one comes
      (a missing one is invalid, and replaceable); a dummy target is
      big-endian with a BOM unless empty. The replacement defaults to
      U+FFFD for a Unicode target and `?` otherwise; `xml:` writes an
      undefined character as `&#xE9;`. The same encoding on both sides
      copies the bytes (applying only the decorators), except that
      `invalid: :replace` scrubs. Not done: `fallback:`,
      `Encoding::Converter`, and MRI's handling of `universal_newline:`
      with a non-UTF-8 source, which ignores the source encoding.
    - **No in-place forms.** `encode!`, `scrub!` and `unicode_normalize!`
      stay undefined (Strings are immutable); every String bang method
      whose plain form exists now says so in its compile error.
    - **`unicode_normalize`/`unicode_normalized?`** (`:nfc`, `:nfd`,
      `:nfkc`, `:nfkd`) use tables generated at development time:
      `scripts/gen-unicode-normalize` dumps MRI's own
      `unicode_normalize/tables.rb` (so the Unicode version is MRI's, 17.0.0
      for Ruby 4.0) into `prelude/go/unicode_normalize_tables.go`, 88 KB of
      string constants parsed once on first use and pruned from programs
      that never normalize. The algorithm is UAX #15's (full decomposition,
      canonical ordering, composition, Hangul by arithmetic); it matched MRI
      on 216k normalizations of every code point below U+3400 and a sample
      above, alone and with combining marks. Invalid UTF-8 raises
      `ArgumentError: invalid byte sequence in UTF-8`; the form is a Symbol.
    - **`Integer#chr(encoding)`** with MRI's `RangeError`s; `Array#pack`
      gains `C`, `c` and `U` (*superseded by decision 138*: every directive
      but `P`/`p`, through the same `rbPack`/`rbPackU`).
    - **IO.** A mode's `b` is binmode (external ASCII-8BIT); `:ext[:int]`,
      with `BOM|`, sets the encodings. As in MRI, reads convert only when an
      internal encoding is given (`"r:ISO-8859-1"` hands back the file's
      bytes; `"r:ISO-8859-1:UTF-8"` converts), writes convert to any
      external encoding but UTF-8 and ASCII-8BIT (a dummy one writes one
      BOM), reading an ASCII-incompatible encoding without `b` or an
      internal one is `ArgumentError: ASCII incompatible encoding needs
      binmode`, and an unknown name warns `Unsupported encoding X ignored`.
      `set_encoding`, `external_encoding`, `internal_encoding`, `binmode`
      and `binmode?` work on File, the std streams and (decision 138) pipe
      and popen ends, whose reads convert as a File's; `$stdout.set_encoding`
      converts what `puts`/`print`/`write` send, and `$stdin` is never
      converted. The transcoder is linked only where needed: a File-using
      program carried ~60 more declarations (18%) for it, so `File.new`
      calls `rbFileEncHook`, which the compiler sets (`rbFileEncModes()`)
      before a `File.open`/`File.new`/`CSV.open` whose mode is not a literal
      without `:`; IO's conversions are func fields set by `set_encoding`. A
      mode with encodings that reaches `File.new` some other way (a call on
      a Class-typed receiver) raises NotImplementedError.
    - `EncodingError` (decision 128) gains `Encoding::CompatibilityError`
      (never raised), `ConverterNotFoundError`, `UndefinedConversionError`
      and `InvalidByteSequenceError`.
    ([example 96](../examples/96_encoding/main.rb),
    `testdata/test/encoding_test.rb`, `testdata/errors/encoding.txtar`,
    `testdata/run/string_output.rb`.)
137. `Marshal` (#47) round-trips the closed world's object graphs in
    rb2go's own bytes, never MRI's (`prelude/marshal.rb`,
    `prelude/go/marshal.go`, `internal/compiler/marshal.go`).
    - **Format.** `RB2GO\x04\x08`, the payload's length (8 bytes,
      little-endian) and one value: a kind byte and its body (`0` nil,
      `T`/`F`, `i` a varint, `f` a float's bits, `"` String, `:` Symbol,
      `@` a back-reference, and four records that name their type: `o` a
      reference value, `u` an object dumped through its `marshal_dump`,
      `v` a value without identity (a tuple), `c` a class object). Every
      reference value (a Go pointer) is numbered as its record starts, on
      both sides, so a shared object loads as one object and a cycle
      (`a = []; a << a`) stays a cycle. Not `gob` behind a header, as the
      issue suggested: gob has no back-references (a cycle never ends, a
      shared value is duplicated), only encodes exported fields, and names
      Go types, not the closed world's classes, so the reference table and
      a type tag per value were needed anyway and gob would only have
      wrapped them. The length is what lets `Marshal.load(io)` read exactly
      one dump from a `File`, `StringIO` or socket, so dumps written one after
      another load one at a time, as MRI's do. MRI's bytes start `\x04\x08`
      and get a `TypeError` saying so; MRI rejects ours ("format version
      4.8 required"). `MAJOR_VERSION`/`MINOR_VERSION` are MRI's 4 and 8.
    - **Types.** A container or tuple is named by its Go type
      (`Array[PointI]`, `Tuple2[Integer, String]`) and loads back as
      exactly that instantiation, so `copy == data` holds, a typed ivar
      takes it without conversion, and identity survives. A struct class
      (user classes, `Struct`, `Data`, exceptions, `Date`/`DateTime`,
      decision 133) is named by its Ruby name and holds its assigned ivars
      by name, read through `_Ivars` and written through `_IvarSet`
      (decision 123, which converts to each ivar's type); it is allocated
      without `initialize`, as MRI allocates. A class object is its name.
      `Time` (instant, nanoseconds, UTC flag, zone), `Rational`,
      `Complex`, `BigDecimal`, `Regexp` and `Random` (the MT19937 state,
      so the copy continues the sequence) are written by hand. Strings
      are values in rb2go, so `[s, s]` loads as two equal Strings where
      MRI keeps one. Frozen state is not kept, as MRI's `load` without
      `freeze:` does not.
    - **Which types.** The compiler collects every concrete
      `Array`/`Hash`/`Set`/`Range` instantiation and tuple type it
      renders and every expression's type (`genExpr`: an inferred local's
      type is never rendered). An instantiation only Go code builds
      (`JSON.parse`'s `Hash[String, any]`) has no case: it dumps through
      `_ToAny` as `Array[any]`/`Hash[any, any]`/`Set[any]`/`Range[any]`,
      keeping its identity, and loads back as that.
    - **Pruning.** `rbMDumpGen`/`rbMLoadGen` (containers, tuples, hooks)
      and `rbMDumpObjGen`/`rbMLoadObjGen` (struct classes, class objects)
      are generated switches whose every case names its type weakly: the
      dump side is a type switch (decision 49), and the load side's
      string switch writes each case as `rbKeyed[T](tag)`, which the
      pruner now treats like a type-switch case, dropping it unless
      something else keeps `T`. A program without `Marshal` keeps none of
      it. The container cases are rendered when the tables go out
      (rendering notes boxes and tuples they list) and emitted in a later
      round once `Marshal` is reached (minitest reaches test methods only
      through `_Call`, a table). The class cases go out last and only for
      the classes reached by then: thousands of pending cases slowed the
      pruner, and a skipped class reached later, or a value type noted
      after the cases were rendered, recompiles eagerly
      (`errPruneIncomplete`), where every class gets a case.
    - **Hooks.** A class defining `marshal_dump` dumps what it returns
      and loads by passing that, converted to `marshal_load`'s parameter
      type, to `marshal_load` on an object allocated without
      `initialize`. `marshal_dump` taking arguments or returning void, and
      `marshal_load` not taking exactly one argument, are compile errors;
      a missing `marshal_load` is MRI's `TypeError` at load (`instance of
      C needs to have method 'marshal_load'`). `_dump`/`self._load` are
      not supported.
    - **Errors.** MRI's messages: `no _dump_data is defined for class
      Proc` (likewise `Method`, decision 141, `Thread`, `Thread::Mutex`, `StringIO`, and a Go
      value behind a prelude ivar as `Object`), `can't
      dump IO`/`File`/`Thread::Queue`; `marshal data too short`
      (`ArgumentError`), `exceed depth limit` for `dump(obj, limit)`,
      `undefined class/module X` for a class the loading program does not
      keep. `singleton can't be dumped` and `can't dump anonymous class`
      cannot arise: a singleton method on an object (`def o.x`) is a
      compile error, so no object has a singleton class, and every class
      is a named constant of the closed world (`Class.new` only as a
      constant's value). A user generic class has no case and gets the
      `no _dump_data` message. rb2go names `Mutex` and the queues without
      MRI's `Thread::`; the messages add it.
    - **IO forms.** `dump(obj, io)` calls `io.write` and answers `io`;
      `dump(obj, io, limit)` too. `load(io)` reads exactly one dump from a
      `File`, an `IO` (`$stdin`, a pipe end, decision 138) or anything with a prelude `read(n)` (`StringIO`,
      sockets: an interface assertion on `__read_1`'s Go method, since
      dynamic dispatch does not reach decision 12's overloads), reading
      the body through a `LimitReader` so a corrupt length allocates
      nothing up front; an IO at its end is MRI's `EOFError`, so `loop {
      Marshal.load(f) }` ends with `rescue EOFError`. Any other object
      with `read` is read to its end; one with neither `read` nor `write`
      is MRI's `TypeError: instance of IO needed`. `Marshal.restore` is
      `load`. `load`'s proc argument and `freeze:` are not built.
      `File.binread`/`File.binwrite` and `File.new`'s `b`/`t` mode
      letters came along (Strings are bytes, so they change nothing), as
      did noting a proc literal's Go type for `rbIsProc`, which Marshal's
      `Proc` message needs.
    - **PStore** (#1's won't-do table) is now feasible as a prelude
      class over this: a `Hash[untyped, untyped]` root marshalled to its
      file inside `transaction`, `abort`/`commit` by `catch`/`throw`, and
      `read_only` checks. It is left for its own issue.
    ([example 97](../examples/97_marshal/main.rb),
    `testdata/test/marshal_test.rb`, `testdata/errors/marshal.txtar`.)
138. IO follow-ups (#54): pipe and popen ends are `IO`s, `IO.popen` writes
    and has a blockless form, `File.atime`, and `Array#pack` with the rest
    of `String#unpack`.
    - **Pipes are IO.** `IO` stays one `@go_type` (decision 62), now with
      an `own` flag: off, it is a standard stream (fd 0-2) as before; on,
      it holds a read end and/or a write end (`*os.File` plus a `bufio`
      reader/writer) and popen's `*exec.Cmd`. `IO.pipe` answers `[IO, IO]`
      and `IO.popen` yields or returns an `IO`, so `.class` is `IO` and
      `inspect` is MRI's `#<IO:fd N>` / `#<IO:(closed)>`. The number is
      the real descriptor (read through `SyscallConn`, since `Fd()` would
      switch the file to blocking mode), so it differs from MRI's run;
      tests match its shape. A subclass was not an option (a `@go_type`
      class can't be subclassed, which is also why sockets in decision
      135 sit beside `IO`); one struct with a flag keeps `STDOUT`, a pipe
      and a popen end the same static type, so a method taking `IO` takes
      all three. Read methods share `rbReader` (STDIN's reader or the
      pipe's, by pointer so `ungetc` can replace it). The pipe writer and
      every popen IO are `sync`, as MRI's; a write after the reader is
      gone is `Errno::EPIPE` (Go ignores `SIGPIPE` off stdout).
    - **popen.** Modes `r`, `w`, `r+`, `w+` (a `b`/`t` is ignored; anything
      else is `ArgumentError: invalid access mode`); a pipe goes on the
      child's stdout for reading and/or stdin for writing, and the other
      streams stay the program's (stdout is flushed before the spawn).
      `close` closes the pipes and then waits for the child, setting `$?`;
      the block form is that `close` in an `ensure`, and the blockless form
      (decision 12's `__popen_enum`) leaves it to the caller. `pid` is the
      child's (nil on a pipe). `close_read`/`close_write` follow MRI: on
      `r+` each closes one pipe and the second closes the whole IO (and
      reaps); on a one-way popen either closes it whole; on a pipe end the
      side it holds closes it and the other side raises `closing non-duplex
      IO for reading/writing`. `mode:`/env/option-hash forms are not built
      (a keyword the signature lacks is a type error).
    - **Standard streams** gain `close`/`closed?`/`close_read`/
      `close_write`/`pid`/`to_i`: closing `STDOUT` marks that object closed
      (its own writes raise `closed stream`) without closing fd 1, so
      Kernel#puts still writes where MRI raises. ponytail: close the real
      descriptor and route Kernel output through the object's state.
    - **atime.** The access time's `syscall.Stat_t` field is `Atim` on
      Linux, OpenBSD and Solaris and `Atimespec` on macOS and the other
      BSDs, and the prelude's Go is concatenated into one generated file,
      so build tags cannot split it. `rbAtime` embeds `*syscall.Stat_t`
      beside a struct holding zero `Atim` and `Atimespec` one level deeper:
      Go's shallowest-field rule resolves each selector to the platform's
      real field where it exists and to the zero stand-in where it does not,
      and the two are summed. One source, chosen by the Go compiler for the
      target, no reflection; checked to build for darwin, linux (amd64,
      arm64, 386, mips), the BSDs and solaris. `File.atime` and
      `File.mtime` raise with MRI's `rb_file_s_atime`/`rb_file_s_mtime`.
    - **pack/unpack** share one parser (`rbPackParse`, with MRI's
      `unknown pack directive 'y' in 'y'` and `'_' allowed only after types
      sSiIlLqQjJ` errors, whitespace and `#` comments skipped) and one
      sizing table, so every directive packs and unpacks the same way:
      `a A Z B b H h u M m` (`m0` strict), `U` (MRI's `rb_uv_to_utf8`, so
      surrogates and values up to 2**31-1 encode), `w`, `C c S s L l Q q J j I i
      n N v V` with `_`/`!` (native: `L!` and `J` are 8 bytes on 64-bit)
      and `<`/`>`, `D d F f E e G g`, and `x X @` with MRI's quirks
      (unpack's `@` defaults to 0 and its `*` counts are the bytes left).
      The u/M/m encoders and decoders are ports of MRI's `encodes`,
      `qpencode` and the lenient base64 loop (which stops at a `=` in a
      quad's third or fourth place, where the next `m` resumes). Elements
      convert as MRI's: Float to an integer directive truncates, `nil` is
      `""` for `a A Z B b H h` and a `TypeError` elsewhere, `M` takes any
      object's `to_s`. An unsigned 64-bit value or BER integer past 2**63
      raises `RangeError` (decision 35) where MRI makes a Bignum. `P`/`p`
      (C pointers) raise `ArgumentError` naming rb2go: no Ruby string has
      an address to hand out. A mixed literal like `[s, n].pack("a4N")` is
      a tuple, so `tupleCall` handles `pack` by passing the fields as one
      `[]any`. Binary results print with `\u0000` where MRI's binary
      String shows `\x00` (no encoding tag: bytes that are valid UTF-8
      count as UTF-8, decision 136), so tests compare
      `.bytes`.
    (`testdata/test/stdlib_test.rb` `IOPipeTest`/`FileAtimeTest`,
    `testdata/test/string_test.rb` `StringPackTest`.)
139. Core fidelity (#55): `to_set`, ancestors without rb2go's modules,
    `Kernel.puts`, `full_message`. (`Range#%` and blockless `step` are
    decision 140's.)
    - **`Enumerable#to_set`** is `Set.new(to_a)`, with a
      `__to_set_block` overload (decision 12) for `to_set { |x| ... }` /
      `to_set(&:m)`; `Set#to_set` stays `self`. Decision 44's
      instantiation cycle is gone since decision 86: a generic
      primitive's methods are free funcs, and the forwarder on `Array[E]`
      is only emitted when a kept interface names `To_set`, which nothing
      does. Like `tally` and every other Enumerable method on a generic
      primitive, `to_set` on an `untyped` Array raises NoMethodError when
      run (the dynamic tables list the class's own methods there).
    - **`Module#ancestors`** (and `included_modules`, which reads the same
      table) leaves out a prelude module marked `# @hidden`: rb2go's own
      helpers that MRI lacks, today `IOWritable` and `IOReadable`. Their
      methods stay on the includer. The marker is read in prelude files
      only. `File.ancestors` is still not MRI's (`File < Object` here, and
      there is no `File::Constants`).
    - **Kernel's module functions** are public on `Kernel` itself: a call
      on `singleton(Kernel)` may reach a private Kernel method when its
      name is one of MRI's `Kernel.singleton_methods` (a fixed list in
      `kernelModuleNames`), or an overload twin of one; `pp`,
      `initialize_copy` and the rest stay private, as MRI's NoMethodError
      says. `Kernel.raise`/`fail`/`lambda`/`proc`/`block_given?`/
      `__method__`/`__dir__` go to the intrinsic the receiverless call
      does, and `Kernel.raise` ends a statement list as `raise` does;
      `Kernel.loop` rescues StopIteration and `Kernel.block_given?`
      narrows an optional block as the bare calls do, and
      `Kernel.public_method` takes them.
      `respond_to?` answers true for those names. `Kernel.require` is
      not an intrinsic (rb2go loads files at compile time).
    - **`Exception#full_message(highlight:, order:)`** is MRI's
      `rb_error_write` in Ruby: the error line (`backtrace[0]: ` and
      `detailed_message(highlight:)`), the `\tfrom` lines (or, with
      `order: :bottom`, a `Traceback` header and numbered lines, widths
      padded), then each cause's report (before, with `:bottom`), each
      cause once. `highlight` defaults to `$stderr.tty?`, `order` to
      `:top`; another order raises MRI's ArgumentError. No error_highlight
      snippet is ever added: rb2go has no node locations at run time, and
      MRI only adds one for NameError/TypeError/ArgumentError raised with
      a location, so checks use other classes. An exception with no
      backtrace starts with MRI's `error_pos`, `file:line:in
      'full_message': `, the innermost user frame of the Go stack
      (`rbErrorPos`, the same `runtime.Callers` walk as decision 106, only
      when asked). A cause raised in a rescue that does not bind has no
      backtrace in rb2go (decision 106), so its line is that `error_pos`
      where MRI shows where it was raised. `detailed_message` now follows
      `rb_decorate_message` too: a lone trailing newline is dropped, an
      empty message is the class name (`unhandled exception` only for
      RuntimeError itself), and `highlight: true` bolds it with the class
      underlined.
    (`testdata/test/stdlib_test.rb` `SetTest#test_to_set`,
    `testdata/test/object_test.rb` `ObjectRubySpecAncestorsTest` and
    `ObjectKernelModuleFunctionTest`, `testdata/test/control_test.rb`
    `ControlRubySpecExceptionTest`, `testdata/errors/strings.txtar`
    `kernel_module_pp`.) `ObjectRubySpecAncestorsTest` subtracts what other
    libraries mix into Object (`PP::ObjectMixin`, `JSON::GeneratorMethods`)
    so the suite passes in one MRI process.

140. `Enumerator`, external iteration, `Enumerator::Lazy` and `Fiber`
    (#41), plus #55's `ArithmeticSequence`. All of it is sequences
    (`iter.Seq`), so nothing runs ahead of its consumer and only `Fiber`
    needs a goroutine.
    - **Enumerator.** `Enumerator[E]` is a generic `@go_type` holding its
      sequence, the receiver and method name `inspect` shows
      (`#<Enumerator: [1, 2]:each>`, `each_slice(2)`), a size function and
      the iteration's result. It includes Enumerable, so every Enumerable
      method works on it. The `__<name>_enum` overloads (decision 12) now
      return one instead of an Array (decision 58): Array's
      `each`/`each_index`, Enumerable's `each_with_index`/`each_slice`/
      `each_cons`, Integer's `times`/`upto`/`downto`, String's
      `each_char`/`each_line`, each with MRI's `size` (`nil` for
      `each_line`, as MRI). Enumerable's take the receiver's own size
      (Array, Hash, Set, an Integer Range, another Enumerator), else
      `nil`, never a count by iterating, which an endless source never
      ends; a user class's own `size` is not consulted yet (MRI's is). Any other prelude iterator called without a
      block (`(1..3).each`, `Hash#each` as `[k, v]` pairs, `Set#each`,
      `each_byte`, `reverse_each`) becomes an Enumerator over its sequence
      in the compiler (`iterEnum`, receiver and arguments evaluated once),
      size `nil`. A user's iterator without a block stays a compile error:
      MRI raises `LocalJumpError` there unless the method returns
      `to_enum`, which is not built. Blockless `map`/`select`/`reject`
      stay `Enumerator::Map`/`Select` (their `with_index` maps or
      filters), now with `next`/`peek`/`rewind`. `Array#with_index`, the
      old stand-in's helper, is gone. Enumerable gained `lazy`, `uniq`
      and `entries`, and Enumerator `to_h` (decision 92's `@self` forms).
      `Enumerable#first(n)` now stops after the nth element instead of
      pulling one more, which a generator with side effects shows.
    - **External iteration.** `next`/`peek`/`rewind` pull the sequence
      with `iter.Pull` (`rbExt`), started on the first `next`: a runtime
      coroutine, no goroutine and no channel. The end raises
      `StopIteration` (`iteration reached an end`) whose `result` is what
      the iteration returned: the receiver for an each-like method
      (`[1].each` → `[1]`); `nil` for `Enumerator.new`, whose block is
      void (below), where MRI's is the block's value.
      Further `next`s raise again until `rewind`, which stops the pull.
      An enumerator abandoned mid-iteration keeps its coroutine parked
      until the program exits, the same leak as a pull never stopped.
    - **`Kernel#loop` rescues `StopIteration`** (so `ClosedQueueError` and
      `Ractor::ClosedError`, its subclasses, too) and answers its
      `result`, as MRI's, but only for a loop whose block lexically may
      raise it: an external `next`/`peek`, a `receive`, or one of those
      constants. The compiler rewrites that loop into `begin; loop { };
      rescue StopIteration => e; e.result; end` (`loopRescue`); every
      other loop stays a plain Go `for` with no `recover`. A
      `StopIteration` raised by a method the body calls, with nothing
      lexical to see, escapes the loop where MRI's would end it.
    - **`Enumerator.new { |y| }`.** The block runs once per iteration,
      through decision 4's `rbSeq`: `y << v` (`Yielder#<<`, `yield`, or
      `&y`, the Yielder as a block) hands v to the consumer, and a
      consumer that stops (`take(3)`, `first`, `break`, `rewind`) unwinds
      the block with `rbStop`, so `ensure` runs and `rescue` passes it
      on. No goroutine: external iteration is the `iter.Pull` above. The
      element type comes from an annotation on the assignment
      (`#: Enumerator[Integer]`), else the compiler probes the block with
      `y` typed `Yielder[untyped]` and joins the types of what it feeds
      `y` (`inferYielder`); a block that feeds nothing is a compile error
      asking for the annotation. `Enumerator.new(size)` takes an Integer.
      The block is typed void so that any statement may end it, `arr.each
      { |x| y << x }` above all (an iterator call's value cannot be used);
      the price is that StopIteration#result is `nil`. The one iterator
      whose value may now be wanted untyped is `loop`, as loopRescue's
      begin: a loop that ends without StopIteration was broken out of.
      Caveat (decision 4's):
      a generator whose own `rescue` catches an exception raised by the
      consumer's block aborts, since Go forbids a range function to
      recover a loop body's panic.
    - **`Enumerator::Lazy[E]`** is a chain of sequence wrappers from
      `Enumerable#lazy`, inspected as MRI's chain
      (`#<Enumerator::Lazy: #<Enumerator::Lazy: 1..3>:map>`). `map`/
      `collect` (`lazy.map { }` keeps the block's type; an untyped block
      result stays `untyped`, as `map`'s does), `select`/`filter`,
      `reject`, `filter_map`, `flat_map` (an Array-returning block),
      `take`, `take_while`, `drop`, `drop_while`, `zip(array)`,
      `with_index` (blockless pairs, or with a block that sees each
      element and index while the elements pass on), `each_with_index`,
      `uniq` (eql?/hash, as Hash keys), `compact` (`@self Lazy[U?]`),
      `eager`, `force`/`to_a`, `each`, `first`/`first(n)`. It includes
      Enumerable, whose eager methods (`sum`, `include?`, `each_slice`)
      end a chain, pulling only what they need.
    - **Infinite ranges.** `1..Float::INFINITY` with an Integer begin is
      an endless `Range[Integer]` flagged `inf` so it inspects as
      `1..Infinity`; it used to join to `Range[Float]` and iterate
      nothing. Its `step` yields Integers where MRI's yields Floats; `end`
      raises RangeError (Infinity is no Integer) and `include?(2.5)` is a
      type error, both answered by MRI.
    - **`Enumerator::ArithmeticSequence[E]`** (#55) is what `Range#%` and
      blockless `Range#step`, `Integer#step` and `Float#step` return:
      `((1..10).%(3))`, `((1...10).step(3))`, `(1.step(10, 3))`
      (`(1.step(10))` when the step is 1), with `begin`/`end`/`step`/
      `exclude_end?`, `first`, `last`/`last(n)`, `size`, `==`, external
      iteration and Enumerable. Integers count by addition, Floats with
      MRI's counted `ruby_float_step`. It is not an Enumerator subclass (a
      `@go_type` class cannot have a `@go_type` parent), so
      `is_a?(Enumerator)` is false. A non-numeric range's step
      (`("a".."e").step(2)`) is the same class taking every nth element
      but inspects as MRI's plain Enumerator. A Float step on a Range is a
      type error (`Range#step` takes an Integer), and an endless
      sequence's `size` raises, as `Range#size` (no Infinity Integer).
    - **`Fiber`** is a goroutine started on the first `resume` and handed
      control over two unbuffered channels, so exactly one of a fiber and
      its resumer runs: `resume(*args)` sends the arguments (the block's,
      the first time; what the paused `Fiber.yield` returns, later) and
      waits for the next `Fiber.yield(*vals)` or the block's end; none is
      nil, one is itself, more an Array, as MRI passes them. Values are
      `untyped`, as Ractor messages are (decision 103): a fiber's resume
      and yield types are set by whichever call runs, not by a
      declaration. `Fiber.current` is a goroutine-id lookup (decision
      104), else the running thread's root fiber; a fiber's goroutine
      belongs to the thread and ractor that first resumed it. An exception
      ending the block re-raises in the resumer and leaves the fiber dead.
      `FiberError` carries MRI 4.0's messages: `attempt to resume a
      terminated fiber`, `attempt to resume the current fiber`, `attempt
      to resume a resuming fiber`, and `attempt to yield on a not resumed
      fiber` for a `Fiber.yield` outside any fiber. **Leak:** a fiber
      never resumed to its end keeps its goroutine blocked on its channel
      until the program exits, as `Timeout`'s abandoned goroutine does.
    - **Not done:** `to_enum`/`enum_for`, `Enumerator#feed`/`next_values`,
      `Enumerator::Chain` (`e1 + e2`), `Enumerator::Product`, `produce`,
      Lazy's own `chunk_while`/`slice_when`/`zip` of non-Arrays,
      `Fiber#raise`/`kill`/`transfer`, fiber storage (`Fiber[]`),
      fiber-local `Thread#[]`, the cross-thread resume check, and fiber
      schedulers.
    ([example 92](../examples/92_generators/main.rb),
    `testdata/test/enumerator_test.rb`, `testdata/test/fiber_test.rb`,
    `testdata/errors/enumerator.txtar`.)

141. `Method` and `UnboundMethod` (#40) are built at the call site from
    the closed world, never looked up by name at run time.
    `recv.method(:name)` (and `public_method`) needs a literal name, like
    decision 32's `send`; the compiler resolves the target on the
    receiver's static type and builds a `Method[F]`: F is a Proc type
    (decision 47) over the target's required positional parameters and
    its result, fn the typed closure over the receiver (held in a temp,
    since Go closures capture variables), and the struct also carries
    `dyn` (the same target called from an `...any` list, generated like
    decision 32's wrappers: arity check, argument conversion, one arm per
    optional count) and `info` (name, owner, parameters, arity, `file:line`).
    `call`, `.()`, `[]` and `===` with F's arguments call fn, typed;
    with more (optional or rest parameters) they go through dyn, with
    decision 32's warning and the result converted back. `to_proc` is fn
    itself, so `&m` is decision 47's `&proc`; `curry` nests one Proc per
    parameter. `Klass.instance_method(:name)` is an
    `UnboundMethod[^(Klass, ...) -> R]`; `bind` checks the object against
    Klass at compile time and partially applies fn, `bind_call` calls it.
    A user-defined struct method is bound statically (its free func), so
    `Base.instance_method(:m).bind_call(sub)` runs Base's `m`, as MRI's
    does; a bound Method dispatches through the receiver, and `owner`
    and `inspect` come from a type switch over the subclasses that
    override the name. `&method(:name)` is desugared to a block of the
    yielded arity calling `recv.__send__(:name, ...)`, so a target with
    optional parameters takes what is yielded (MRI's Hash#each, which
    yields one pair to a method proc whose minimum arity is 1, is not
    modelled: the pair is yielded as two values). Reflection follows MRI
    4.0: `parameters`/`arity` from the def for user methods; prelude
    methods stand for MRI's C methods, so they are anonymous (`(_)`, or
    `(*)` and -1 when any parameter is optional, a rest, a keyword, or
    decision 12 overloads the name); `inspect` is
    `#<Method: Recv(Owner)#name(params) file:line>`, `Recv.name` for a
    singleton method, whose owner is MRI's `#<Class:Foo>`; `==`, `eql?`
    and `hash` compare the definition and the receiver's identity.
    Values of different F join as `Method[untyped]`, and RBS's bare
    `Method` (`#: Hash[Symbol, Method]`) is `Method[untyped]` too: its
    calls go through dyn, with the warning, and a typed Method converts
    to it (`_to_any`/`rbFrom`, as Array's instantiations do).
    `Method#unbind` is an `UnboundMethod[untyped]`, since F does not
    carry the receiver's type; its `bind` raises MRI's TypeError for an
    object that is not an owner's instance. `method(:name)` on an
    `untyped` receiver is `Method[untyped]` over the dispatcher, with the
    warning; its `owner`, `arity` and `parameters` raise
    NotImplementedError, being unknown at compile time. Compile errors:
    a computed name, a target that needs a block, has required keywords
    or is generic, `instance_method` on a generic class (an
    UnboundMethod cannot carry its type arguments), and `to_proc`/`curry`
    on a `Method[untyped]`. Not done: `super_method`,
    `source_location`, keyword arguments or a block to `call`.
    `x.class` on a plain Object (main, `Object.new`) held untyped now
    answers Object (it raised NoMethodError).
    ([example 98](../examples/98_method_objects/main.rb),
    [testdata/test/method_test.rb](../testdata/test/method_test.rb),
    `testdata/errors/objects.txtar`.)

142. `Numeric` (#39) is a prelude module that `Integer`, `Float`,
    `Rational`, `Complex` and `BigDecimal` include, and that includes
    `Comparable`. A module, not a class, because a module type is
    already Go `any` holding whichever value it is, with calls on it
    dispatched by the generated `rbDyn` switches (decision 32) and
    typed by the module's declared signatures; a class above five
    `@go_type` classes would have needed a new kind of class. Ruby sees
    the difference only in reflection: `Numeric.class` is `Module` and
    `Integer.superclass` is `Object` (MRI: `Class`, `Numeric`);
    `ancestors` and `is_a?` match. User code cannot subclass or
    include it (compile errors), so its includers are a closed set.
    - **Signatures only.** `prelude/numeric.rb` declares the methods a
      `Numeric` value answers (`+`, `zero?`, `abs` → `self`, `divmod`
      → `Array[Numeric]`, `to_c`, ...), which type calls on one; their
      bodies raise and never run, since every number class defines or
      `undef`s each one (`checkNumeric` makes a miss a compile error).
      Inherited bodies would have needed decision 9's constraint to hold
      for all five classes, which Complex (no order) cannot meet. A
      `Self` result on a module value is the value's own type (`abs` on a
      Numeric is a Numeric; `clamp` on a Comparable a Comparable). `<=>`
      on a Numeric is `Integer?` (nil for a Complex), and calls on one
      give no dynamic-call warning: dispatch at run time is the type's
      point. A block on a Numeric (`n.step(3) { }`) is a compile error:
      narrow it first.
    - **`is_a?`, `when`, `===`.** `is_a?(Numeric)`/`kind_of?` on a value
      whose class only the run time knows (untyped, a module type, a
      type variable) asks the class ancestry table (`rbKindOf`, the
      table `Module#===` reads), and narrows an untyped local to
      `Numeric` (the same Go value). In a type switch `when Numeric`
      asks the same table in the `default` arm, and the five Go types
      leave any later arm (Go rejects a repeated case; Ruby's first
      match wins anyway): a case naming all five would keep BigDecimal
      and Complex in every program using it, since the pruner drops
      only single-type cases. Decision 21's compile error stays for
      other modules.
    - **Mixing, typed.** Decision 12's class twins (`__plus_rational`)
      go first. A call they miss whose parameter is the receiver's own
      class (arithmetic `+ - * / % modulo remainder div divmod fdiv
      quo`, order `<=> < <= > >=`, and `step`) converts the operand
      lower in the tower `Integer < Rational < Float < BigDecimal <
      Complex` to the higher one's class (`to_r`, `to_f`, `to_d`,
      `to_c`; a Rational to BigDecimal at the BigDecimal's coerce
      precision, `rbBDCoercePrec`, as MRI's `BigDecimal#coerce`) and
      calls that class's method, typed (`numericTower`):
      `Rational(1, 2) < 0.75` is `Rational(1, 2).to_f < 0.75`. Only a
      binary call raises its receiver; `step` converts its arguments
      down to the receiver (`Rational(1, 2).step(2)`), or widens an
      Integer receiver to Float, iterators included. Comparable's
      methods compare through `rbNum` across classes, as clamp answers
      the bound itself (`Rational(5, 2).clamp(1, 2)` is `2`). Integer
      with Float keeps numericMix's widening. A method named like an
      operator's twins (`div` against `/`'s `__div_integer`) no longer
      takes them, which had made `7.div(Rational(1, 2))` a Rational.
    - **Mixing, at run time.** A number's Dyn wrapper for a
      one-argument method picks a twin by the argument's class in a
      type switch, then, for the ops above, coerces an argument its
      parameter does not take with `self` up the same tower
      (`rbNumCoerce`) and sends the method again (`rbDyn<Op>`). Every
      case of those switches names one class, so the pruner drops the
      cases of classes the program never makes (decision 49): an
      untyped `+` costs four small functions, not Rational and
      BigDecimal code. Both paths were checked against MRI over all 25
      class pairs of each op (`testdata/test/number_test.rb`'s
      `NumberNumericTest` keeps a cross-section).
    - **What does not mix.** A BigDecimal with a Complex: rb2go's
      Complex parts are Integer, Rational or Float (decision 42), so a
      typed mix is a compile error and an untyped one a TypeError, and
      `BigDecimal#to_c`/`#i` are undefined (MRI makes a Complex with a
      BigDecimal part). `**` is not in the tower, because MRI's answer's
      class depends on the values (`4 ** Rational(1, 2)` is a Float,
      `Rational(1, 4) ** 2` a Rational): only the existing twins mix it,
      and `Rational ** Rational` is a compile error. Complex has no
      order, as in MRI: `<`, `between?`, `clamp`, `%`, `div`,
      `divmod`, `modulo`, `remainder`, `positive?`, `negative?`,
      `floor`/`ceil`/`round`/`truncate` and `i` are undefined on it;
      its `<=>` compares two real values only (nil otherwise).
    - **Filled gaps**, each as MRI: Integer `infinite?`, `coerce`
      (a Float pair unless both are Integers); Float `div`, `remainder`,
      `quo`, `coerce`, `step` without a block; Rational `nonzero?`,
      `finite?`, `infinite?`, `real?`, `magnitude`, `fdiv`, `div`, `%`,
      `modulo`, `divmod`, `remainder`, `step` (with a block or as an
      Array), `coerce`; Complex `<=>`, `zero?`, `nonzero?`, `integer?`,
      `infinite?`, `coerce`; BigDecimal `integer?`, `real?`,
      `magnitude`, `fdiv`, `step`, and its own `between?`/`clamp`
      (its comparisons take any number, which Comparable's
      `(self)`-typed ones cannot; a NaN raises, as Comparable's).
      Known differences: `BigDecimal#step` yields BigDecimals even
      with a Float limit or step (MRI: Floats), and `Rational#coerce`
      with a Complex is always a Complex pair (MRI: a Rational pair
      when the imaginary part is an exact zero): either answer's class
      would depend on a value.
    ([example 99](../examples/99_numeric/main.rb),
    `testdata/test/number_test.rb` `NumberNumericTest`,
    `testdata/errors/numbers.txtar` `numeric_*`.)

143. Pattern matching (#38): `case/in` (guards, `else`), `v => pat` and
    `v in pat`, compiled to static Go like `case/when` (decision 21).
    - **Shape of the code.** Each pattern is a chain of Go `if`s, one per
      check, with the rest of the match nested in the success branch;
      an arm's success sets a flag and the arms become
      `if ok { body } else { next arm }`, so tails, `return`, `next` and
      `break` work as in `if`. A guard is one more `if`. Bindings are
      ordinary local writes (Ruby locals of the enclosing scope, hoisted
      when the body reads them, decision 14); the unset pass sees each arm
      as a branch (a case/in without `else` raises rather than falling
      through) and `v in pat` as maybe-taken, and `if v in pat`
      narrows the locals it bound to non-nil values, except a `_x` an
      alternative binds. A subject that is a
      local is narrowed in the arm's body by a leading class
      (`in Circle`, `in Circle(r:)`), as `case/when`'s type switch does.
      The subject, and each checked element, is evaluated once.
    - **Types come from the pattern.** A binding takes the type of what
      it binds: `Array[E]`'s `E`, a tuple field's type, `Hash[K, V]`'s
      `V`, a Struct/Data member's type, the element or value type of a
      user `deconstruct`/`deconstruct_keys` signature; `Integer => n`
      narrows by type assertion; untyped stays untyped. A find
      pattern's and a rest's slices keep the Array's type.
    - **deconstruct.** `Array#deconstruct` and `Hash#deconstruct_keys`
      return self. Struct and Data get both, generated with the rest of
      decision 30 (MRI 4.0's Data has `deconstruct` too); `deconstruct_keys`
      is MRI's: nil gives `to_h`, more keys than members gives `{}`,
      otherwise the keys up to the first non-member; it takes
      `Array[Symbol]?` (MRI's String and Integer keys are not supported).
      On a statically typed Struct/Data the pattern reads the members
      directly, typed (unless a subclass overrides the method), and calls the generated methods only when it needs
      the Array or Hash itself (a bound `*rest` or `**rest`, `**nil`, a
      find pattern). A user class's `deconstruct` must be typed to return
      an Array or tuple (or untyped), and `deconstruct_keys` a Hash with
      Symbol or untyped keys; anything else is a compile error. As MRI,
      `deconstruct_keys` gets the pattern's keys, or nil when the
      pattern has `**rest` or `**nil` or no keys (`{}`). MRI caches `deconstruct` across a
      `case`'s arms; rb2go calls it per arm.
    - **Untyped subjects** (and Object, module types, a generic `T`, or a
      class only some subclasses define the method on) are checked at run
      time: `respond_to?` and the call through decision 32's dispatch
      tables, then the result converted to `Array[untyped]` or
      `Hash[untyped, untyped]` (`TypeError` when it is not one, worded
      as decision 20's, where MRI says `deconstruct must return Array`).
      A pattern's own dynamic calls do not warn; later calls on what it
      bound do.
    - **Order.** A hash pattern checks that every key is present before
      matching any value, as MRI's compiler does, so
      `{a: 1} => {a: 2, c:}` fails on the missing `:c`.
    - **Errors.** `NoMatchingPatternError < StandardError` and
      `NoMatchingPatternKeyError < NoMatchingPatternError`, whose `key`
      and `matchee` raise `ArgumentError` when unset, as MRI's. A
      `case/in` with several arms and no `else` raises the subject's
      inspect. `=>` and a one-arm `case/in` without `else` raise MRI's
      detailed message, `"<inspect>: <why>"`: `P === v does not return
      true`, `length mismatch (given n, expected m)` (`m+` with a rest),
      `does not respond to #deconstruct`, `key not found: :k` (a
      `NoMatchingPatternKeyError` whose matchee is the deconstructed
      Hash), `rest of {...} is not empty`, `{...} is not empty`, `does
      not match to find pattern`, `guard clause does not return true`;
      an alternation reports its last alternative, class included (a
      missing key in an earlier one does not make it a
      `NoMatchingPatternKeyError`). The message is built
      only on the failure path.
    - **Compile errors** for shapes that never match the static type: an
      array or find pattern on a class with no `deconstruct` in its
      hierarchy (`Integer`, `String`, `Hash`), a hash pattern on one with
      no `deconstruct_keys` (or on a tuple), on a Hash whose keys are not
      Symbols, or a `deconstruct(_keys)` typed to return something else
      (`testdata/errors/pattern.txtar`). A class check the static type
      decides false (`in String` on an Integer) makes a dead arm, which
      is dropped as `case/when` drops one; a later read of a local only
      it would bind is then a compile error where MRI reads nil. Not
      built: minitest's `assert_pattern` and `must_pattern_match`.
    ([example 93](../examples/93_pattern_matching/main.rb),
    `testdata/test/pattern_test.rb`.)

144. `Object.new` and `BasicObject.new` (#56) build a plain object, made
    to be unique: ruby/spec builds one to compare by identity, to use as a
    Hash key or to pass where any value goes. It is a `*Object` (Go
    `struct{ _ byte }`, not `struct{}`: Go may give every new zero-size
    value one address, which would make two of them `equal?`), typed
    `Object`, so calls on it are those on any `Object`-typed value: the
    intrinsics (`==`, `equal?`, `hash`, `inspect`, `to_s`, `eql?`, the
    last now static too: the value's own `eql?`, else identity) or dynamic
    dispatch, where a bare `*Object` answers Kernel's and Object's public
    methods as nil held untyped does. Object and BasicObject now have a
    class ID and count as heap objects, so `inspect` shows the address and
    `object_id` is the pointer's; `main` (the same Go type) prints `main`,
    as MRI's singleton `to_s` does. Arguments are a compile error with
    MRI's arity message.
145. `Class.new` and `Module.new` (#56) are declared at compile time, one
    class per literal, as `describe` is (decision 83). MRI makes the class
    when the call runs; the closed world sees every literal, so:
    - **Named.** `Name = Class.new(Super) do ... end` (top level or in a
      class or module body) is `class Name < Super`, and
      `Name = Module.new do ... end` is `module Name`: the constant names
      it, as in MRI. With no block it is an empty subclass
      (`NotFound = Class.new(StandardError)`).
    - **Anonymous.** Any other literal (a local, an argument, a method
      receiver as in `Class.new { ... }.new`, inside a def, block, `it`
      or `let`) is a hidden class the expression evaluates to, typed as
      its class object, so `c.new`, `c.new.foo` and `is_a?(c)` are
      static calls. Its `to_s` is `#<Class:file:line>` where MRI prints
      an address, and `name` is that same String where MRI's is nil
      (`Module#name` is `String`, not `String?`, so every `k.name.upcase`
      keeps compiling).
    - **The block is the class body**: the same `def`, `attr_*`,
      `include`, constants and `class << self` a `class` body takes. A
      block does not open a constant scope in Ruby, so its constants
      belong to the enclosing one; outer locals are not visible to its
      defs, as in MRI.
    - **One class per literal.** A `Class.new` in a loop or in a method
      called twice is the same class each time, where MRI makes a new
      one per run. A superclass must be a constant (a local holding a
      class is a compile error), and a block with parameters is one too.
    - **`inherited`.** A named literal runs `Super.inherited` where its
      constant is assigned; an anonymous one runs it each time the
      literal is evaluated, as MRI's does, though the class it passes is
      the same one each time (#75).
    - **Not done:** `define_method` in the body, and `include` of a module
      held in a local (no static form).
    - `Proc.new { ... }` is `proc { ... }` (decision 47).
    - **Errors stay local.** An anonymous literal whose body does not
      collect (a `define_method` in it) raises its error where the
      literal is generated, so under `CompileTestsSkipping` it skips the
      one test holding it rather than stopping the program.
    ([example 100](../examples/100_class_new/main.rb),
    `testdata/test/object_test.rb` `ClassNewTest`.)
146. Parameter types come from use (#56). ruby/spec, like most Ruby, has
    no annotations, and the closed world sees every call, so a parameter
    with no `#:` or `# @rbs` type takes the join of what the program
    passes it. This keeps everything typed: the Go is what the
    annotations would have produced. Missing types never mean `untyped`.
    - **What is inferred.** A def's positional, optional, `*rest` and
      keyword parameters (`**opts` is not), from every call, `new` (for
      `initialize`), `super` and an optional parameter's default. A def
      with no signature that `yield`s takes a block whose parameters are
      the join of what each yield passes (every yield must pass the same
      count; `block_given?` makes it optional); the block returns `void`,
      so a def that uses yield's value needs its block annotated. A
      lambda, `lambda { }` or `proc { }` with parameters and no expected
      type (decision 47) takes them from its `call`, `.()`, `[]`,
      `yield` and `===` sites, and from the block it stands in for as
      `&f`. `include Enumerable` with no type argument takes it from the
      class's own `each` (what that yields is the element), and that
      `each` keeps its own signature rather than the module's.
    - **Joins.** In source order (files in load order, then position).
      `nil` with `T` is `T?`; a subclass with its superclass is the
      superclass; classes with nothing in common but Object join to their
      union (`ident(1); ident("a")` types x `Integer | String`, Integer
      with Float `Integer | Float`, decision 150). A use that still does
      not join the ones before it (a module type, or two instantiations of
      one generic class) is left out, so the final compile reports it at
      its call: `String where Integer is expected; parameter x takes its
      type from the call at main.rb:3`. Uses whose type holds `untyped` count for nothing, and
      Object, BasicObject and modules (Go `any`) count only when nothing
      concrete is passed: one `Object.new` among Integers would otherwise
      join them all to Object, an untyped parameter by another name; it
      fails at its call instead. A parameter no call types keeps the
      missing-annotation error, which now says so. When a round fails
      outright (a link error elsewhere), that error is reported instead
      of the parameters it left untyped, since it is the cause.
    - **How.** A compile that meets an untyped parameter aborts before
      emitting. `compile` then runs rounds, each a fresh compile through
      return inference (about a tenth of a full build, no emit or
      format): pending parameters are `untyped` there, every user method
      body and file top level is dry-run, a statement that does not
      compile is skipped rather than ending its body, and each argument's
      type is recorded under the parameter's key (file, def offset,
      name). The joined types feed the next round, moved into its classes
      by name, so a parameter typed from another inferred one settles a
      round later; rounds stop when nothing changes (at most 8), and the
      final compile runs as an annotated program would. A program with
      every type written runs no round. `RB2GO_INFER_DEBUG=1` prints what
      each round inferred and what it could not compile. Keys are
      file:line:column:name, which survive `cmd/rubyspec`'s line-keeping
      cuts; it passes an `Inference` seed between the compiles of one
      program, which start from its types (and from the keys it found no
      type for), so a compile after a cut usually runs no round.
    - **Branches.** Inference surfaced `list.empty? ? [2, 5] : list` with
      `list` an `Array[Integer?]`: branches whose types do not join now
      try each branch's type as the expected type of all of them (a
      literal types itself from what is expected), and take the first
      that compiles. And `fits` no longer lets `Array[T?]` pass as an
      `Array[T]` (Go's type arguments are invariant), which had compiled
      to Go that does not build.
    - **Overrides.** An unannotated override inherits its parent's
      signature only when its parameter list has the same shape
      (positional count, rest, keyword names); otherwise it is a def of
      its own (`def initialize = super(4)` under `def initialize(sides)`),
      and pending parameters stay shared along a chain that does inherit.
    - **Not inferred:** a block's return type (above), `**opts`, a rest
      parameter no call passes anything to, and a parameter whose only
      uses are themselves untyped.
    ([example 101](../examples/101_inferred_params/main.rb),
    `testdata/test/infer_test.rb`, `testdata/errors/infer.txtar`.)
147. A module may hold instance variables (needed to drop the global,
    mutex-guarded side tables of decisions 74 and 108: rb2go's runtime
    takes no process-wide lock, which would serialize threads as MRI's
    GVL does). Each is declared in the module body, `# @rbs @x: T` (an
    undeclared one is a compile error: module methods are not dry-run for
    discovery). The module gets a state struct `M_Ivars`; each struct
    class that includes it, directly or through another module, holds an
    `M_Ivars` field, and a subclass shares its parent's through
    embedding; the module's constraint requires `_M() *M_Ivars`, so `@x`
    in a module method is `self._M().x`, the same shape as a class's own
    `self._Foo().x`, and an includer's own methods read `@x` the same
    way. Including such a module in a `@go_type` class or at the top level
    (Object) is a compile error, as is giving a generic module instance
    variables. Not yet: module ivars in `inspect`'s ivar list, and
    `extend` on a single object
    ([example 102](../examples/102_module_ivars/main.rb),
    `testdata/test/object_test.rb` `ModuleIvarTest`).
148. Instance variables narrow like locals (#75). After `@x ||= v`, a
    non-nil `@x = v`, `@x += v`, `if @x`, `@x.is_a?(T)` or a guard
    (`return unless @x`), a `T?` ivar reads as `T` in that scope, so
    `File.join(@dir, name)` compiles without a check rb2go would
    otherwise demand. The narrowed view is forgotten at a write to the
    ivar, at `yield` or `super`, and after any call through self
    (receiverless or `self.`) to a method the program defines; calls to
    prelude methods, attribute readers, and calls on other receivers keep
    it. That is unsound in two ways, both accepted: another object
    holding self can reset the ivar in between, and so can another
    thread (rb2go's runtime takes no process-wide lock, decision 147). A wrong narrowing
    dereferences a nil pointer where MRI raises NoMethodError; both are
    crashes. A flow analysis that knew every method's ivar writes would
    close the first gap and is not worth it yet.
    - **Read before any write.** A method that reads an ivar before
      anything typed it (`unless @names; @names = [...]`) no longer stops
      discovery: the read is nil, and the write that follows declares the
      ivar `T?`, since MRI's first read really is nil. Not for modules'
      ivars, which decision 147 declares by annotation.
    - **Redundant checks still compile.** `@x || d`, `if @x` and `@x&.m`
      on a narrowed ivar answer as MRI's (the left side, true, the call)
      without the always-true warning or the non-nilable `||` error a
      plain `T` gets: the program wrote the check before rb2go knew it
      was redundant, and it must keep compiling.
    - **Not done:** `x.nil? ? a : x.foo` narrowing (locals lack it too),
      and `defined?(@x)` (#53).
    ([example 103](../examples/103_ivar_narrowing/main.rb),
    `testdata/test/object_test.rb` `ObjectIvarNarrowTest`.)
149. An `each` that never yields (#75) gives its `include Enumerable`
    (or any module typed from each, decision 146) nil as the element
    type. ruby/spec's `EnumerableSpecs::Empty` is `def each = self`:
    there is nothing to infer from, and no element ever exists, so the
    choice cannot be observed. nil is a real rb2go type (Go `any`
    holding nothing), not `untyped`, which #56 ruled out. The each gets
    the block signature `{ (nil) -> void }`, so Enumerable's iterator
    adapter (seqAdapter) applies; a block that calls a
    method on an element compiles as a call on nil (a dynamic-call
    warning) and never runs. "Never yields" is read from the body: no
    `yield`, `&blk`, `block_given?`, `to_enum` or `enum_for`. Annotate
    `include Enumerable #[T]` for anything else.
    (`testdata/test/enumerator_test.rb` `EnumeratorNeverYieldsTest`.)
150. Union types: `A | B` is a Go `any` whose classes the compiler knows,
    and every decision about it is made at compile time. Before this a
    union annotation was an error and every place that would build one
    collapsed to `untyped`, which throws away a member list the closed
    world already has and sends each later call through a `rbDyn`
    dispatcher with boxed arguments.
    - **Representation.** `TUnion{Members}` in the sealed `Type` (decision
      119), built only by `unionOf`, so always normalized: flattened,
      deduplicated, a subclass absorbed by a superclass member, `untyped`
      absorbing everything, Object absorbing every class, one member left
      collapsing to that member (or `T?`), members sorted, nil last. nil
      is a member, never a box: `T?` stays `*T` (its narrowing is free,
      decision 7), and `A | B | nil` is the interface's own nil. The Go
      type is `any`, so `Array[A | B]` is `*Array[any]`, the same shape as
      `Array[untyped]`. Members are classes, tuples, procs and nil; a
      module or type variable is a compile error for now (a type switch
      cannot test either directly), as are two members of one Go type
      (`Array[Integer | String] | Array[Float | Symbol]`, both
      `*Array[any]`: Go rejects the duplicate case). A value of another
      instantiation into a union (`Array[Integer]` for an
      `Array[untyped]` member) is converted to the member's Go type, or the
      switch would miss it.
    - **Calls.** A call on a union is a Go type switch with one arm per
      member, each the typed call on that member (`case Integer:
      t = Integer.Size(x)`), and the arms' results join into the call's
      type. A member without the method is a compile error ("undefined
      method upcase for Integer (a member of Integer | String)"). nil, as
      a member, answers what nil answers (`to_s`, `inspect`, `==`, ...)
      and otherwise raises NoMethodError with a warning, as a nil `T?`
      does (decision 20). Iterator calls (`xs.each { }`) loop in the arms
      whose member's method is an iterator.
    - **Arguments.** A call whose argument is a union the method does
      not take as it is (a typed parameter, or a method with class twins
      like `split`'s `__split_regexp`, decision 12) is split the same way:
      the receiver and the arguments before it are evaluated first, as
      Ruby does, then a type switch on the argument makes the call once
      per member, so each arm picks its own overload (`10 * x` with
      `x: Integer | Float` is `Integer.Op_mul` or `Float.Op_mul`). The
      split runs only after the call failed to compile as it is (its
      output is rolled back), so ordinary calls pay nothing for it.
    - **Boundaries.** A member's value into a union is free (boxed as for
      untyped, a tuple keeping its Go type so the switch finds it); a union
      into a type every member fits is asserted; anything else needs the
      union narrowed first, as `T?` does. An `untyped` value into a union
      (an argument, a dynamic call's parameter) goes through a generated
      `rbUnion_<hash>` type switch that raises MRI's TypeError when the
      value is no member, converting other Array/Hash instantiations as
      `rbAs` does. The name hashes the members, so it does not depend on
      compile order (the build cache, decision 88). A union holding a tuple
      shows untyped code the Array it is (`rbUnion_<hash>Out`). A Boolean
      position tests truthiness. Array and Hash literals where a union is
      expected take the member they can be (a tuple of their length, else
      an Array; a Hash).
    - **Narrowing** keeps the member list. `is_a?(C)` true narrows to the
      members that are a C; false (an `else`, `unless`, `!`, `||`, and
      early exits like `return … if x.is_a?(C)` or `next if …`) to the
      rest; truthiness drops nil; `x.nil?` false drops nil. One class left
      is asserted (`x.(String)`); one class and nil open into a `T?`. On a
      union subject `when C` takes the members that are a C by their own
      Go types (`when Numeric` takes Integer and Float), the arm is typed
      as exactly those, the `else` as what is left, and a `case` whose
      arms take every member and has no `else` is exhaustive: it never
      yields nil. `case` on an ivar or attribute reader narrows it, as
      `if` does (decision 148). Patterns (`case`/`in`) test a union as
      untyped for now.
    - **Joins.** Where two types have no common class but Object (branch
      values, `&&`/`||`, a local assigned both, inferred returns and
      parameters, Integer with Float) the join is their union, where it
      was a compile error or `untyped`. A module type, Object, or two
      instantiations of one generic class still do not join. An
      inferred parameter (decision 146) keeps its union only while its
      method's body compiles with it: when a round's dry run of the body
      fails, the parameter's uses join as before unions from the next
      round on, so the odd call is reported (and ruby/spec skips that one
      example) instead of the shared helper failing for every caller
      (`def io_fixture(name, mode = "r")` passing mode on to a String
      parameter). A generic method's `E?` with `E` a union is the box
      `*any` its Go code returns, opened into the union at the call, as
      for `untyped` (decision 7). A union rebuilt for a later compile
      (`rehome`) drops a member whose class that compile lacks, rather
      than collapsing to untyped. Literal
      elements are the exception for now: `[1, "a"]` stays a tuple and
      `{a: 1, b: "x"}` a `Hash[Symbol, untyped]`, since a union element
      type would reject the later `h[:c] = 1.5` that MRI runs; making
      them unions needs decision 13's fall-back-to-untyped for a typed
      literal whose program fails. A known gap: pass 1 reads a local at
      the type of its first assignment, so after `x = nil; x = 1 if c;
      y = x` a `y ||= "s"` is checked against y's nil-then-String type,
      not x's; this predates unions (with `T?` it was a Go build error).
    - **What stays `untyped`.** Open-world values (JSON, Marshal, `send`
      with a computed name) and universal parameters (`==`, `puts`) are
      the universal union and keep the dispatchers. `T | untyped`
      parameters keep decision 20's meaning.
    *Revised (review):* a call on a union, or with an argument reading a
    union local, evaluates its non-literal arguments once into temps
    before the type switch (`pinArgs`): generated in every arm they cost
    members^depth for nested arithmetic (`x + (x + ...)` took 38s at depth
    6), and a local assigned in an argument (`x * (n = 2)`) was declared
    only in an arm. Literals stay per arm, typed by each member's
    parameter. A union passed into another whose members are other Go
    types (`Array[Integer]` into `Array[untyped]`, or such a `T?`) goes
    through the target's checker, which converts it; a generic `T` is
    checked at run time, as untyped is. A proc member matches only its
    own signature (a `^(Integer) -> Float` matched any proc member and
    then no switch case). Helper names hash to 64 bits, and a collision is
    a compile error, never a shared checker.
    (`examples/104_union_types`, `testdata/test/union_test.rb`,
    `testdata/errors/union.txtar`.)
151. Hot core methods are shaped for Go's inliner (budget 80), measured
    against MRI 4.0.7 on small loops:
    - `Integer#/` and `#%` drop their explicit zero check: Go's own
      divide panic becomes `ZeroDivisionError` through `rbWrapPanic`,
      which `rbTopRecover` now also applies, so an uncaught one prints
      MRI's `divided by 0 (ZeroDivisionError)`. The check alone cost the
      call (a 15% slower Integer loop).
    - `==` between two Integers, Floats, Strings or Symbols goes to a
      typed `__eq_<class>` (decision 12's class overload): a Go compare,
      where `==(untyped)` boxed the argument (an allocation past 255) and
      type-switched, 8x slower in a loop.
    - `sum` with an Integer or Float element (or block) type adds unboxed;
      Float still with MRI's Kahan-Babuska (`rbSummer.addFloat`).
    - The frozen check inlines (one field load since decision 96's revision).
    - A struct class's own method is forwarded to its free func with
      `Self` = `*C`, not `CI`, so `self._C()` and self calls are direct.
      Not when its parameters, block or return name the class or `self`:
      the forwarder holds those as `CI` (a `self?` return is `**C`).
    - `rbExitStatus` wraps every panic it is handed, so an at_exit handler's
      divide by 0 also ends as `ZeroDivisionError`.
    Not done: a default `GOGC` (200 halves GC time on string-heavy code but
    nearly doubles peak memory on collections); a mutable String for `<<`
    (the frozen-strings rule; `s += x` stays quadratic).
