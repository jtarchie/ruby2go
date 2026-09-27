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
cmd/rb2go/            CLI: rb2go [-o out.go] main.rb
internal/compiler/    Ruby → Go: declarations, types, codegen
internal/rbs/         the RBS type-syntax subset the compiler understands
prelude.rb            core library entry point; require_relatives prelude/*.rb
prelude/              core library, written in Ruby, compiled by the same transpiler;
                      includes net_http.rb and webrick.rb over Go's net/http
examples/NN_*/main.rb  one feature per program; runs on MRI unchanged
testdata/run/*.rb     smaller behaviour cases, same MRI oracle, no rbs/lint gate
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
`go run ./cmd/rb2go main.rb` when needed.

`TestRun` holds `testdata/run/*.rb` to the same MRI comparison but skips the
rbs and lint gates, so each file costs one `go build`. `TestErrors` compiles
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
  `os.Exit` skips defers: `Kernel#exit` must flush first. Interleaving with
  `$stderr` would need `$stdout.sync`-style flushing; not handled yet.
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
    `T`, untyped ones pass unasserted and the method handles them. Regexp
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
    e.g. `1e+20`, `0.0000123`) are ported. Generation only; no parsing.
    `to_json(opts)` takes the generator options `indent`, `space`,
    `space_before`, `object_nl`, `array_nl`, `depth`, `script_safe`
    (`escape_slash`), `ascii_only` and `allow_nan`; `sort_keys`, `strict`
    and `as_json` raise `NotImplementedError`, other keys are ignored as
    the gem ignores unknown ones, and `max_nesting` is not checked.
    `JSON.generate` takes no options. Like the gem, the generator calls
    every value's `to_json` with one (opaque) state argument: a user
    `to_json` declared other than `(*untyped) -> String` is reached
    through its dynamic wrapper, so `(?untyped)` gets the state and `()`
    raises `ArgumentError`. *(Revised: options were ignored, and such a
    `to_json` was skipped for the JSON of its `to_s`.)*
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
    goroutine and servlet instance.
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
