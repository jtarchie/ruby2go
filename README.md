# Ruby → Go transpiler

I want to compile a subset of Ruby to Golang. The runtime of Golang supports
enough that I believe it to be possible.

Requirements:

- use this type syntax (https://github.com/soutaro/rbs-inline,
  https://github.com/soutaro/rbs-inline/wiki/Syntax-guide)
- we should be able to support `class`, `module`, `includes`, and inheritance

Anti-goals:

- we don't need to support eval, or *reflective* dispatch (`send`,
  `method_missing`, `define_method`). Class-based virtual dispatch on `self`
  **is** required — inheritance doesn't work without it (see
  [01_inheritance](examples/01_inheritance/)).

I want to support all the native types of Ruby as Golang primitives, but with
methods. These are ideas, and not limited to or the strict implementation.

## Layout

```
prelude.rb        core library, written in Ruby, compiled by the same transpiler
examples/NN_*/    main.rb (user code, runs on MRI) + main.go (what the transpiler would emit)
check.sh          runs every example under ruby and go, diffs stdout
```

Every `main.go` is hand-written today, but it is the *target output*: it
compiles, passes `go vet`, and prints byte-for-byte what MRI prints. The
examples double as golden tests once the transpiler exists — `./check.sh`.

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
[main.rb](examples/00_string_hierarchy/main.rb),
[main.go](examples/00_string_hierarchy/main.go).

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
  works because strings are immutable.
- **Literals need wrapping only for interface targets** — Go's untyped
  constants convert to `String` when the parameter is `String`
  (`s.Lt("world")` compiles), but become Go `string` when the parameter is
  `any`. Always emit `String("...")` when the target type is an interface or
  `untyped` (see [06_puts](examples/06_puts/)).

## Examples

Each pair produces identical output under `ruby` and `go run`; `./check.sh`
verifies. User code is plain Ruby that runs on MRI, so the transpiler can be
tested by diffing against MRI output.

### 01 — Inheritance, `super`, overriding

[main.rb](examples/01_inheritance/main.rb) ·
[main.go](examples/01_inheritance/main.go)

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

[main.rb](examples/02_enumerable/main.rb) ·
[main.go](examples/02_enumerable/main.go)

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

[main.rb](examples/03_nil/main.rb) ·
[main.go](examples/03_nil/main.go)

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

[main.rb](examples/04_exceptions/main.rb) ·
[main.go](examples/04_exceptions/main.go)

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

[main.rb](examples/05_word_count/main.rb) ·
[main.go](examples/05_word_count/main.go)

Three mechanism tests prove feasibility; one small real program shows what a
user would actually write, and surfaces what the toys don't.

- **Hash ordering.** Go maps are unordered *and randomized per run*; Ruby
  preserves insertion order. This example happens not to depend on it (the full
  sort key decides), but any program that prints or iterates an unsorted hash
  would have nondeterministic output — which also kills MRI-diff testing.
  Recommended: ordered `Hash` as `map[K]int` index into `[]entry` with
  tombstones (Python-dict style, ~40 lines, O(1) ops). Omitted from `main.go`
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
[main.go](examples/06_puts/main.go) ·
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

## Open decisions

1. Ordered `Hash`: recommended (determinism), see 05. Confirm.
2. `TrueClass`/`FalseClass` vs. a single `Boolean` (affects `nil`/truthiness
   and `inspect`). Prelude currently uses `Boolean`.
3. Operator name table (`==`→`Eq`, `<=>`→`Cmp`, `+`→`Plus`, `?`→`Q`, `!`→`Bang`,
   `=`→`Set`, unary `-`→`Neg`) — must be injective (`upcase` vs `upcase!`,
   `==` vs `equal?`).
4. Non-local `return`/`break`/`next` inside blocks: inline-loop vs. sentinel
   panic, and where the boundary is.
5. `Hash.new(default)` / `Hash#[]` typing.
6. Unwrapped-prelude coverage: `prelude.rb` currently covers examples 00 and 06 (`puts`);
   the rest carry their prelude subset inline in `main.go`. Fold them in
   as the prelude grows.
