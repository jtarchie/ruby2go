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
- **Mixins are just Ruby** — `Comparable#<` and `#clamp` are written once;
  `String` only supplies `<=>`. Compiles to `Comparable_Lt[T Comparable_Self[T]]`.
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
  works because strings are immutable. `frozen?` uses the same identity: a
  string is frozen if it shares a literal's bytes (the compiler emits a table
  of every literal, `rbStringLits`) or was passed to `freeze`; strings built
  at run time are not, as in MRI. Other values: immediates, `Regexp` and
  `Data` are frozen; objects, `Array`, `Hash` and `Struct` are not.
- **Literals need wrapping only for interface targets** — Go's untyped
  constants convert to `String` when the parameter is `String`
  (`s.Lt("world")` compiles), but become Go `string` when the parameter is
  `any`. Always emit `String("...")` when the target type is an interface or
  `untyped` (see [06_puts](examples/06_puts/)).

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

`Enumerable` written once in the prelude against `each`; Go generics inference.

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
  `!`; `T?` → `!= nil`; everything else → constant true (warn).
- **Methods on `nil`.** `x.inspect` where `x: String?` → static branch:
  nil → `NilClass_Inspect()`, else `x.Inspect()`. No runtime `NilClass` object.
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
  `rescue` needs a named result.
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
- **`puts(*a)` splat** → `Kernel_Puts(a.ToAAny()...)`.
- **Output is buffered** (`bufio.Writer`), flushed by `defer` in `main` —
  runs on return and on panic, so partial output before a crash matches Ruby.
  `os.Exit` skips defers: `Kernel#exit` must flush first. Interleaving with
  `$stderr` would need `$stdout.sync`-style flushing; not handled yet.
- **Return type `nil` → no Go return value.** `puts`'s result is never
  meaningfully used; if it is, the call site substitutes `nil`.
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
2. `TrueClass`/`FalseClass` vs. `Boolean`: **decided, one `Boolean`** (a Go
   `bool`). `true`/`false` literals are untyped constants that convert to
   `Boolean`, and get wrapped (`Boolean(true)`) only when the target is
   `untyped`. `inspect`/`to_s` live on `Boolean`.
3. Operator name table: **decided** — `==`→`Eq`, `!=`→`Ne`, `<=>`→`Cmp`,
   `<`→`Lt`, `<=`→`Le`, `>`→`Gt`, `>=`→`Ge`, `+`→`Plus`, `-`→`Minus`,
   `*`→`Mul`, `/`→`Div`, `%`→`Mod`, `**`→`Pow`, unary `-`→`Neg`, `+@`→`Pos`,
   `!`→`Not`, `~`→`Inv`, `<<`→`Shl`, `>>`→`Shr`, `&`→`BitAnd`, `|`→`BitOr`,
   `^`→`BitXor`, `=~`→`EqTilde`, `!~`→`NotTilde`, `===`→`Eqq`, `[]`→`Idx`,
   `[]=`→`IdxSet`. Everything else camel-cases with `?`→`Q`, `!`→`Bang`,
   `=`→`Set`; leading underscores are kept (`__write`→`__Write`). The table
   has to stay injective against camel-cased names too: `[]` is not `Index`
   because `String#index` exists, and `=~` is not `Match` because `#match`
   exists.
4. Non-local `return`/`break`/`next` in blocks: **decided, inline loops
   only.** A method whose block returns `void` compiles to a Go iterator
   (`iter.Seq`/`iter.Seq2`) and every call site with a block becomes a
   `for range` loop, so `return`, `break` and `next` are plain Go. Blocks
   passed to value-returning methods (`map`, `select`, `then`, …) are Go
   closures; `next` is `return`, and `return`/`break` inside them is a
   compile error. The sentinel-panic fallback is not implemented.
   A method is an iterator only when the block *and* the method return
   nothing, the block is only yielded to, and nothing rescues around the
   yield (Go forbids a range function from recovering a panic raised in
   the loop body). A `%x{}` leaf is an iterator only if its Go builds one
   (`func(yield ...`). Everything else takes a closure: `Thread.new { }`
   returns a Thread, `mount_proc(path) { }` stores its block.
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
8. Dispatch shape: struct classes get an interface (`ShapeI`) of their full
   method set plus `_Shape() *Shape` accessors for every struct in the
   chain (ivar access from free functions, and the marker `rescue` matches
   on). Methods defined on struct classes and modules are free functions
   generic over `Self`, forwarded by a Go method on every concrete class.
   `BasicObject`, `Object` and `Kernel` are "universal": their `Self` is
   `any`, since primitives inherit from them too.
9. Module constraints are derived from the module body, as the README
   says: `Comparable_Self[Self]` lists what `Comparable`'s methods call on
   `self` (including `self.X(` inside `%x{}`), not every module method.
   Primitive classes get forwarders only for those; everything else is
   called through the free function. This is also what avoids Go's
   "instantiation cycle": a forwarder such as `Hash[K,V].Tally` would
   instantiate `Hash[[K,V], Integer]`, whose forwarders instantiate the
   next size up, forever.
10. Type parameters are all constrained `comparable`. Every generated Go
    type satisfies it (strings, ints, pointers, interfaces, tuples of
    those), and it is what `map[K]` and `tally` need; deriving the
    constraint per parameter bought nothing.
11. `raise` is an intrinsic: `raise Klass, msg` ≡ `raise Klass.new(msg)`,
    `raise "msg"` ≡ `RuntimeError.new`. Uncaught exceptions flush stdout,
    print `message (Class)` to stderr and exit 1, like MRI. Go runtime
    panics (`index out of range`, divide by zero) are wrapped into
    `IndexError`/`ZeroDivisionError`/`StandardError` on the way into a
    `rescue`.
12. Overloads (`#|`) are not supported, so `first`/`take` require a count
    (`arr.first(3)`; use `arr[0]` for the head) and `Array#[]` takes one
    Integer. `split` takes an optional separator through `?String?`.
13. Empty `[]`/`{}` literals without an annotation are `Array[untyped]` /
    `Hash[untyped, untyped]`, which is what Ruby's are; any other missing
    type is an error, and an unannotated override inherits the parent's
    signature (never `untyped`).
14. Locals are inferred from their assignments (joined across branches:
    `nil` + `String` → `String?`) and hoisted to a `var` at the top of the
    function when Go's block scoping would otherwise hide them. Lifted
    temporaries for `&.`, `||`, ternaries and `case`-expressions are
    computed before the statement they belong to, so their side effects run
    slightly earlier than MRI would run them.
15. Instance variables are typed from `attr_*` annotations, `# @rbs @x: T`,
    or a dry run of the class's method bodies (`initialize` first); an ivar
    that is only ever assigned `nil` needs an annotation.
16. `%x{}` bodies are Ruby xstrings, so Ruby escape processing applies to
    the Go inside them: write `\\n` for a Go `\n`, `\#{` for a literal `#{`,
    and keep braces balanced (no `"{"` in Go strings). A one-line body of a
    non-void method gets `return` prepended.
17. Namespaces: a class's Go name joins its constant path with `_`
    (`Resty::Actions::Show` → `Resty_Actions_Show`); a generated map gives
    messages and `inspect` the Ruby name back. Constants resolve as Ruby
    does: the lexical scope innermost-out (`Module.nesting`), then the
    innermost class's ancestors, then top level; `class A::B` compact form
    does not put `A` in scope. RBS names in annotations resolve the same way.
18. Constants are Go package variables, typed by `#: T` or by their
    initializer, and assigned in `main` in source order (prelude first), as
    MRI evaluates them. *(Revised: they used to initialize before `main` in
    Go's dependency order.)*
19. Class methods: every class and module (except `BasicObject` and
    `Kernel`) gets a metaclass — a struct class holding the
    class methods, inheriting from the parent's metaclass — and one
    instance of it is the class object. Class methods therefore inherit and
    dispatch virtually like instance methods; `singleton(C)` is the
    metaclass's interface; `self.class` is a per-class accessor. Each
    metaclass gets generated `new`, `name`, `to_s` and `inspect`. `new` is
    kept off the shared interface because subclasses may change
    `initialize`: `klass.new(...)` through `singleton(Base)` type-asserts
    for a matching `New`, failing at run time where Ruby would raise
    `ArgumentError`. `Foo.new` on a constant stays a direct constructor
    call. `class << self` is not supported. Metaclasses inherit from the
    parent's metaclass, else from prelude `Class` (modules: `Module`), so
    a value typed `Module` can hold any class object.
20. `T?` where `T` is expected is a compile error (check it first:
    `if x`, `return unless x`, `x ||= …`, `&.`). `untyped?` is untyped:
    passing it on asserts the type. *Calling a method* on `T?` raises
    `NoMethodError` when it is nil, as in Ruby, and the compiler warns.
    Narrowing follows `if x`, `if x.is_a?(C)`, `&&`, and early-exit guards
    (`return … unless cond`, `return if x.nil?`) for the rest of the
    block; attribute reads on `self` narrow like locals; reassigning drops
    the narrowings.
21. `is_a?`/`kind_of?` is a constant when static types decide it and a Go
    type assertion otherwise. There is no runtime record of included
    modules, so `is_a?(SomeModule)` on an untyped value, or on a struct
    class that might have a subclass including it, is a compile error.
    Narrowing an untyped local to `Array` views it as `Array[untyped]`.
22. Unannotated literals infer by joining their parts; when parts share
    no type the element type is `untyped`. A 2–3 element mixed array with
    nothing expected of it is a tuple (sort keys, multiple returns).
23. Symbols are a named Go string distinct from `String`. `f(a: 1)` on a
    method without keyword parameters passes a Hash, as Ruby 3 does;
    keyword parameters themselves are not supported.
24. Regexps are Ruby syntax on Go's RE2. Every pattern gets `(?m)` (Ruby's
    `^`/`$` are line anchors), Ruby `/m` becomes `(?s)`, `\h` is expanded;
    lookaround, backreferences, `\Z` and `/x` are rejected with `file:line`
    at transpile time. Static patterns compile once into package
    variables; interpolated ones compile at run time and raise
    `RegexpError`. `$~`/`$1` are not supported; use `match`.
25. JSON matches the json gem: escapes (quotes, backslash, control
    characters; `/` and non-ASCII as-is) and floats (its `fpconv` rules,
    e.g. `1e+20`, `0.0000123`) are ported. Generation only; no parsing.
26. Threads are goroutines. An exception ends only its thread (reported on
    stderr) and `join` re-raises it. There is no GVL: stdout writes are
    locked, other shared state is the program's problem.
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
    required), keyword `new`, `==`, `to_h`, `members`, `inspect`, `to_a`
    and `with` are generated as Ruby and compiled like user code.
    `keyword_init` is not supported.
31. `method_missing` on a typed receiver: an unknown method compiles to
    `method_missing(:name, *args)`, typed by its signature.
    `respond_to?(:name)` folds to a constant, or asks `respond_to_missing?`.
32. Dynamic dispatch: a method called on an `untyped` value, an unknown
    method on a `Module`-typed class object, or a method only subclasses
    define compiles to `rbDynName(recv, args...)`. Each such name gets a
    `DynName(args ...any) any` wrapper on every class with a public,
    non-generic, block-less method of that name: MRI's `ArgumentError` for
    arity, `TypeError` for argument types, then the typed call. Without a
    wrapper the call goes to `method_missing`, then `NoMethodError` (or
    `NameError` for a bare name). `send`/`public_send` with a literal name
    are ordinary calls; a computed name switches over every method name
    and makes the output larger, so it is generated only when used. On
    generic classes, methods whose signatures nest the type parameters in
    another type get no wrapper: wrapping them makes Go instantiation
    cycles. Blocks cannot cross a dynamic call.
    *Revised:* `<=>` differs twice. Its wrappers are always generated:
    `sort`/`min`/`max`/`sort_by` on untyped values reach them through
    `rbCmp` inside generic prelude code, where the compiler cannot see
    the instantiation. A wrong argument type answers `nil`, as MRI's `<=>`
    does, and `rbCmp` turns `nil` into MRI's `ArgumentError: comparison
    of X with Y failed` (which pair MRI names depends on its sort order).
    `hash` on an untyped value calls the class's `hash`, else hashes
    `inspect`, so plain objects are not identity-hashed.
33. Ruby semantics for looser code: `expr rescue fallback`; `return` in
    `ensure` discards the pending exception; `&&`/`||` return values of
    any types (unions become `untyped`) and evaluate the right side only
    when Ruby would; locals first assigned in a branch or `begin` body are
    visible after it (Ruby scopes are methods and blocks); `rescue` and
    `ensure` are generated after the body; `untyped` in a `bool` position
    is truthiness.
34. `Hash#inspect` prints symbol keys as labels (`{a: 1, "a b": 2}`), as
    Ruby 3.4 does.
