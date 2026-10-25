# Benchmarks

Wall-clock times for the workloads in [`benchmarks/`](benchmarks/), measured
on one Apple-silicon machine (MRI 4.0.7, Crystal 1.22.1, Go 1.27.3, spinel
`baea3e28`) on 2026-10-25, after decision 372. Each cell is the median of 9 runs.
To reproduce:

```sh
RUNS=9 MARKDOWN=1 scripts/benchmark          # this table; SPINEL=/path/to/spinel adds that column
COLS=ruby,rb2go scripts/benchmark fib array  # some columns, some workloads
```

| workload | MRI | MRI+YJIT | rb2go | spinel | Crystal | Go |
|---|---:|---:|---:|---:|---:|---:|
| `fib(32)` | 0.27s | 0.092s | 0.017s | 0.014s | 0.017s | 0.014s |
| `Hash` counting, 500k | 0.11s | 0.096s | 0.015s | 0.007s | 0.012s | 0.009s |
| `Array#map`/`select`/`sum`, 1M | 0.15s | 0.14s | 0.021s | 0.011s | 0.012s | 0.010s |
| string build/split, 100k | 0.11s | 0.11s | 0.030s | 0.018s | 0.024s | 0.017s |
| `Math.sin`/`cos`, 200k | 0.086s | 0.072s | 0.009s | 0.006s | 0.008s | 0.007s |
| classes, polymorphic calls, Comparable | 0.26s | 0.17s | 0.036s | - | 0.013s | 0.012s |
| binary trees (allocation), depth 10 | 0.082s | 0.068s | 0.010s | 0.008s | 0.010s | 0.007s |
| splay tree | 0.21s | 0.12s | 0.013s | 0.019s | 0.018s | 0.012s |
| blocks, procs, lambdas, enumerator chains | 0.36s | 0.34s | 0.064s | 0.16s | 0.037s | 0.012s |
| raise/rescue/retry, catch/throw | 0.28s | 0.25s | 0.12s | 0.076s | 2.51s | 0.018s |
| regexp match/gsub/scan/tr, StringScanner | 0.76s | 0.74s | 0.21s | 0.83s | 0.20s | 0.28s |
| sort_by/group_by/tally, Set, Struct | 0.55s | 0.53s | 0.12s | 1.12s | 0.053s | 0.068s |
| format, StringIO, case mapping | 0.49s | 0.44s | 0.24s | 0.20s | 0.14s | 0.10s |
| JSON generate + parse | 0.14s | 0.15s | 0.12s | 0.23s | 0.13s | 0.25s |
| Bignum: Fibonacci, 2000!, 1,200 digits of pi | 0.21s | 0.21s | 0.059s | 0.12s | 0.080s | 0.038s |
| pairing_heap gem: Dijkstra + heapsort | 0.68s | 0.34s | 0.21s | - | 0.038s | 0.034s |
| liquid gem: a template rendered 400 times | 0.38s | 0.30s | 0.17s | - | - | - |
| kramdown gem: a 20-section document to HTML, 30 times | 0.45s | 0.55s | 0.41s | - | - | - |
| total (rows each one ran) | 5.58s | 4.72s | 1.87s | 2.82s | 3.30s | 0.89s |
| vs MRI on those rows | 1.0x | 1.2x | 3.0x | 1.4x | 1.4x | 5.3x |

The rows other than Bignum are within noise of the previous run (2026-10-25,
before decision 372): the default build is the same machine code, and
`TestCompileCost` compiles the same 1,962 / 7,663 / 30,275 functions for
05_word_count / 64_observable / 116_cuba_web. kramdown and liquid are the
noisiest rows here (MRI's own time moved 0.44s to 2.66s between two runs), so
those two were rerun on their own.

## What the columns are

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

## Reading the numbers

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

## Integer overflow modes

`benchmarks/bignum.rb` carries `# int_overflow: promote`, so rb2go compiles it
with Bignums ([decision 372](docs/decisions/372.md)) and `scripts/benchmark`
hands spinel the same `--int-overflow=promote`. A Bignum is a Go `math/big`
value only past 64 bits; smaller values stay machine words. That puts rb2go
ahead of MRI's bignum code (3.6x) and spinel (2x), and between the Crystal
(GMP) and Go (`math/big`) ports.

What the default `raise` mode's overflow checks cost, measured as rb2go built
with `--int-overflow=wrap` (no checks on `+`, `-`, `*`, `-@`) against the
default, median of 9 paired runs per workload:

| workload | raise | wrap | wrap vs raise |
|---|---:|---:|---:|
| array | 0.020s | 0.023s | +14.0% |
| binary_trees | 0.008s | 0.008s | -2.7% |
| blocks | 0.065s | 0.062s | -3.6% |
| collections | 0.125s | 0.125s | +0.3% |
| exceptions | 0.117s | 0.115s | -1.4% |
| fib | 0.016s | 0.016s | +3.7% |
| format | 0.237s | 0.238s | +0.2% |
| hash | 0.014s | 0.015s | +0.3% |
| json | 0.114s | 0.114s | +0.1% |
| kramdown | 0.405s | 0.401s | -1.0% |
| liquid | 0.164s | 0.164s | +0.0% |
| math | 0.008s | 0.008s | -0.7% |
| objects | 0.035s | 0.035s | +1.0% |
| pairing_heap | 0.205s | 0.203s | -0.8% |
| regexp | 0.212s | 0.211s | -0.2% |
| splay | 0.013s | 0.012s | -2.3% |
| strings | 0.029s | 0.029s | +2.1% |

No workload gets faster without the checks beyond run-to-run noise (the
`array` row is a 20 ms run). The checked operators inline into one compare and
a branch the CPU predicts, so `raise` stays the default at no measurable cost
and `wrap` is for programs whose arithmetic is meant to wrap (hashes,
checksums, random number generators), not for speed.
