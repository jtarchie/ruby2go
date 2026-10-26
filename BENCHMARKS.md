# Benchmarks

Wall-clock times for the workloads in [`benchmarks/`](benchmarks/), measured
on one Apple-silicon machine (MRI 4.0.7, Crystal 1.22.1, Go 1.27.3, spinel
`baea3e28`) on 2026-10-26, after decisions 377 and 378. Each cell is the median of 9 runs.
To reproduce:

```sh
RUNS=9 MARKDOWN=1 scripts/benchmark          # this table; SPINEL=/path/to/spinel adds that column
COLS=ruby,rugaroo scripts/benchmark fib array  # some columns, some workloads
```

| workload | MRI | MRI+YJIT | rugaroo | spinel | Crystal | Go |
|---|---:|---:|---:|---:|---:|---:|
| `fib(32)` | 0.27s | 0.094s | 0.018s | 0.014s | 0.018s | 0.016s |
| `Hash` counting, 500k | 0.11s | 0.096s | 0.017s | 0.007s | 0.014s | 0.010s |
| `Array#map`/`select`/`sum`, 1M | 0.15s | 0.13s | 0.018s | 0.011s | 0.012s | 0.011s |
| string build/split, 100k | 0.11s | 0.11s | 0.025s | 0.018s | 0.025s | 0.018s |
| `Math.sin`/`cos`, 200k | 0.086s | 0.072s | 0.010s | 0.006s | 0.009s | 0.008s |
| classes, polymorphic calls, Comparable | 0.26s | 0.17s | 0.036s | - | 0.015s | 0.013s |
| binary trees (allocation), depth 10 | 0.081s | 0.068s | 0.010s | 0.008s | 0.011s | 0.008s |
| splay tree | 0.22s | 0.12s | 0.015s | 0.019s | 0.019s | 0.013s |
| blocks, procs, lambdas, enumerator chains | 0.36s | 0.33s | 0.045s | 0.16s | 0.036s | 0.012s |
| raise/rescue/retry, catch/throw | 0.28s | 0.25s | 0.12s | 0.076s | 2.50s | 0.019s |
| regexp match/gsub/scan/tr, StringScanner | 0.76s | 0.73s | 0.21s | 0.83s | 0.21s | 0.28s |
| sort_by/group_by/tally, Set, Struct | 0.57s | 0.54s | 0.12s | 1.12s | 0.052s | 0.069s |
| format, StringIO, case mapping | 0.48s | 0.45s | 0.12s | 0.20s | 0.14s | 0.10s |
| JSON generate + parse | 0.14s | 0.15s | 0.12s | 0.23s | 0.13s | 0.25s |
| Bignum: Fibonacci, 2000!, 1,200 digits of pi | 0.20s | 0.20s | 0.058s | 0.12s | 0.070s | 0.038s |
| pairing_heap gem: Dijkstra + heapsort | 0.68s | 0.34s | 0.10s | - | 0.039s | 0.036s |
| liquid gem: a template rendered 400 times | 0.37s | 0.30s | 0.16s | - | - | - |
| kramdown gem: a 20-section document to HTML, 30 times | 0.45s | 0.55s | 0.40s | - | - | - |
| total (rows each one ran) | 5.58s | 4.69s | 1.60s | 2.82s | 3.30s | 0.91s |
| vs MRI on those rows | 1.0x | 1.2x | 3.5x | 1.4x | 1.4x | 5.2x |

This run follows decisions 377 (module funcs specialized per container, literal format strings compiled, String `+`
chains in one allocation) and 378 (a non-leaf or generic class's `C?` is its interface, nil its nil): rugaroo's total went from
1.87s to 1.60s, pairing_heap 0.21s to 0.10s, format 0.24s to 0.12s, blocks 0.064s to 0.045s. No spinel build was at
hand, so its column is the 2026-10-25 run's (same spinel commit); its "vs MRI" is computed against this run's MRI times.
`TestCompileCost` compiles 2,014 / 7,730 / 30,289 functions for 05_word_count / 64_observable / 116_cuba_web.

## What the columns are

- **rugaroo** and **spinel** ([matz/spinel](https://github.com/matz/spinel), a
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

Where rugaroo trails the native columns, the cost is mostly in its Ruby
semantics: a frozen check on every mutation, a Hash
that keeps insertion order, and objects that are always heap pointers (decisions 359, 360 and 378 removed the box around a nilable object, which put splay ahead
of spinel and Crystal and halved pairing_heap). It is ahead of spinel on
regexp, collections, blocks, format and JSON, where spinel's runtime library is
thinner. Against Crystal it wins on exceptions, which Crystal unwinds with a
backtrace per raise.

The closest rows to MRI are JSON and kramdown. MRI's `json` is a C extension; rugaroo's
generator writes into one buffer instead of a String per nested value
(decision 346), and its parser builds Hashes on an entry array (decision 350). kramdown spends its regex time partly on
patterns RE2 cannot match (lookbehinds, atomic groups, backreferences), which
run on the regexp2 backtracker (decisions 242, 332, 347).

`Math` calls Go's `math` package directly, so a printed transcendental can
differ from MRI in the last digit — Go and MRI's libm are both within one
ulp and neither is uniquely correct ([decision 43](docs/decisions/043.md)).
Ahead-of-time compilation is the other trade: `rugaroo build` is ~0.3s warm
on hello-world, but the resulting binary starts in under 10ms (MRI ~60ms).

## Integer overflow modes

`benchmarks/bignum.rb` carries `# int_overflow: promote`, so rugaroo compiles it
with Bignums ([decision 372](docs/decisions/372.md)) and `scripts/benchmark`
hands spinel the same `--int-overflow=promote`. A Bignum is a Go `math/big`
value only past 64 bits; smaller values stay machine words. That puts rugaroo
ahead of MRI's bignum code (3.6x) and spinel (2x), and between the Crystal
(GMP) and Go (`math/big`) ports.

What the default `raise` mode's overflow checks cost, measured as rugaroo built
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

## spinel's benchmark suite

spinel ships its own benchmarks: 74 programs in
[`benchmark/`](https://github.com/matz/spinel/tree/master/benchmark), mostly
micro-benchmarks of one VM path (ivar reads and writes, sends, allocation,
loops) plus a few small applications (a template engine, a CSV pipeline, a
Lisp). Neither compiler was tuned against the other's suite, so this is the
away game for rugaroo, as `benchmarks/` is for spinel. Same machine and
versions as above, spinel `baea3e28`, 2026-10-26, median of 5 runs:

```sh
S=/path/to/spinel
INT_OVERFLOW=promote:bm_bigint_fib,bm_pidigits RUNS=5 MARKDOWN=1 BENCH_DIR=$S/benchmark \
  SPINEL=$S/spinel COLS=ruby,yjit,rugaroo,spinel scripts/benchmark
```

`bm_bigint_fib` and `bm_pidigits` build with `--int-overflow=promote` in both
compilers: spinel promotes their growing locals in its default mode too, and
the files carry no `# int_overflow:` line (#117). Every other row is the
default `raise`.

| workload | MRI | MRI+YJIT | rugaroo | spinel |
|---|---:|---:|---:|---:|
| bm_ackermann | 0.49s | 0.12s | 0.074s | 0.068s |
| bm_ao_render | 2.69s | 1.15s | 0.16s | 0.11s |
| bm_attr_accessor | 1.60s | 1.58s | 0.069s | 0.054s |
| bm_bigint_fib | 0.053s | 0.055s | 0.006s | 0.004s |
| bm_binary_trees | 0.081s | 0.067s | 0.010s | 0.008s |
| bm_csv_build | 1.88s | 1.70s | - | 0.39s |
| bm_csv_process | 0.80s | 0.69s | 0.33s | 0.24s |
| bm_fannkuch | 0.059s | 0.060s | - | 0.005s |
| bm_fasta | 0.059s | 0.058s | - | 0.006s |
| bm_fib | 0.61s | 0.15s | 0.035s | 0.029s |
| bm_gcbench | 3.17s | 1.65s | 0.24s | 0.29s |
| bm_getivar | 0.18s | 0.19s | 0.013s | 0.005s |
| bm_getivar_module | 1.23s | 1.24s | - | 0.005s |
| bm_handle_read_loop | 3.04s | 1.93s | - | 0.19s |
| bm_handle_readonly_param | 2.61s | 1.22s | - | 0.072s |
| bm_histogram | 18.65s | 18.61s | - | 0.40s |
| bm_huffman | 0.11s | 0.11s | - | 0.012s |
| bm_inline | 1.72s | 1.16s | 0.099s | 0.032s |
| bm_io_wordcount | 0.13s | 0.12s | 0.057s | 0.034s |
| bm_jekyll_lite | 0.054s | 0.055s | - | 0.005s |
| bm_json_parse | 0.41s | 0.30s | - | 0.058s |
| bm_keyword_args | 0.26s | 0.25s | 0.016s | 0.005s |
| bm_life | 0.86s | 0.41s | 0.16s | 0.030s |
| bm_linked_list | 0.35s | 0.28s | 0.037s | 0.045s |
| bm_loops_times | 1.07s | 0.36s | 0.19s | 0.038s |
| bm_mandel_term | 0.072s | 0.064s | 0.014s | 0.006s |
| bm_matmul | 0.40s | 0.40s | - | 0.012s |
| bm_micro_lisp | 0.054s | 0.068s | - | 0.006s |
| bm_nbody | 0.060s | 0.067s | 0.007s | 0.005s |
| bm_nested_loop | 0.65s | 0.64s | 0.018s | 0.005s |
| bm_nil_recv_loop | 1.32s | 0.28s | 0.069s | 0.069s |
| bm_nqueens | 0.32s | 0.32s | - | 0.013s |
| bm_object_new | 0.17s | 0.17s | 0.007s | 0.005s |
| bm_object_new_init | 0.25s | 0.19s | 0.008s | 0.006s |
| bm_object_new_no_escape | 0.30s | 0.18s | 0.009s | 0.005s |
| bm_partial_sums | 1.27s | 1.28s | 0.43s | 0.025s |
| bm_pidigits | 0.054s | 0.055s | 0.007s | 0.005s |
| bm_point_in_polygon | 1.01s | 0.35s | 0.18s | 0.059s |
| bm_poly_cells | 0.053s | 0.055s | - | 0.006s |
| bm_range_each | 22.47s | 14.67s | 2.15s | 0.44s |
| bm_rbs_items | 0.23s | 0.19s | 0.013s | 0.026s |
| bm_rbs_items_control | 0.20s | 0.22s | 0.012s | 0.032s |
| bm_rbtree | 0.51s | 0.18s | 0.025s | 0.031s |
| bm_reader_index | 11.54s | 12.55s | - | 0.23s |
| bm_ruby_xor | 1.44s | 0.38s | - | 0.088s |
| bm_send_bmethod | 0.31s | 0.29s | - | 0.005s |
| bm_send_cfunc_block | 1.08s | 1.08s | 0.031s | 0.17s |
| bm_send_rubyfunc_block | 1.79s | 1.07s | 0.091s | 0.070s |
| bm_setivar | 0.29s | 0.29s | 0.016s | 0.014s |
| bm_setivar_object | 0.29s | 0.30s | 0.017s | 0.014s |
| bm_setivar_young | 0.29s | 0.29s | 0.016s | 0.014s |
| bm_sieve | 0.64s | 0.65s | 0.099s | 0.021s |
| bm_sinatra_mini | 0.052s | 0.052s | - | 0.004s |
| bm_so_lists | 0.42s | 0.26s | 0.34s | 0.030s |
| bm_so_mandelbrot | 1.66s | 1.58s | 0.12s | 0.033s |
| bm_sort_by | 0.10s | 0.083s | 0.019s | 0.016s |
| bm_spectral_norm | 0.11s | 0.087s | - | 0.007s |
| bm_splay | 0.22s | 0.12s | 0.015s | 0.020s |
| bm_stark_field | 1.24s | 0.92s | - | 0.10s |
| bm_str_concat | 0.057s | 0.056s | 0.006s | 0.005s |
| bm_str_readonly_param | 0.056s | 0.057s | - | 0.009s |
| bm_structaref | 0.24s | 0.24s | 0.014s | 0.008s |
| bm_structaset | 0.45s | 0.46s | 0.064s | 0.016s |
| bm_sudoku | 0.18s | 0.11s | - | 0.009s |
| bm_tak | 0.55s | 0.14s | 0.036s | 0.027s |
| bm_tarai | 0.45s | 0.13s | 0.030s | 0.022s |
| bm_template | 0.92s | 0.71s | - | 0.15s |
| bm_template_render | 1.39s | 1.21s | - | 0.28s |
| bm_threaded_render | 6.40s | 6.00s | 0.69s | 0.58s |
| bm_throw | 0.29s | 0.24s | - | 0.006s |
| bm_tree_walker | 2.12s | 0.47s | - | 0.18s |
| bm_tree_walker_frames | 0.40s | 0.13s | - | 0.043s |
| bm_wordfreq | 0.058s | 0.059s | 0.010s | 0.006s |
| total (rows run) | 106.60s | 82.87s | 5.36s | 5.09s |
| vs MRI | 1.0x | 1.3x | 9.5x | 21.0x |

rugaroo compiles 45 of the 74 unmodified. On the 44 rows both ran, the
totals are MRI 51.2s, YJIT 33.5s, rugaroo 5.36s (9.5x MRI), spinel 2.19s
(23.3x MRI). The footer's ratios above are over different row sets, so spinel's
21.0x there includes the 29 programs rugaroo does not build.

- **rugaroo ahead of spinel:** `bm_send_cfunc_block` (5.5x), `bm_rbs_items*`
  (2-2.6x), `bm_splay`, `bm_rbtree`, `bm_linked_list`, `bm_gcbench`: the
  allocation and block-call rows, as in the main table.
- **Furthest behind:** `bm_partial_sums` (17x spinel: Float `**`,
  `Math.sin`/`cos`/`sqrt` in a `while` loop), `bm_so_lists` (11x: Array
  `shift`/`push`/`pop` and whole-Array `==`; the only row near MRI),
  then `bm_life`, `bm_loops_times`, `bm_range_each`, `bm_sieve` (4.7-5.3x):
  tight loops over Integers, Ranges and Arrays. These rows have not been
  profiled yet.
- Rows under 10 ms are mostly process start-up for both binaries.

Why the other 29 do not compile, by the first error each reports:

| cause | programs | |
|---|---:|---|
| a possibly-nil value used as non-nil (`Array#[]`, `Hash#[]` (decision 5) and friends return `T?`) | 17 | fannkuch, fasta, handle_readonly_param, histogram, jekyll_lite, json_parse, matmul, nqueens, reader_index, ruby_xor, sinatra_mini, spectral_norm, stark_field, str_readonly_param, sudoku, template, tree_walker |
| `String#<<`: Strings are immutable (decision 58) | 6 | csv_build, handle_read_loop, huffman, str_concat, template_render, threaded_render |
| a recursive method with no typed non-recursive path (decision 355) | 2 | micro_lisp, tree_walker_frames |
| an ivar with no assignment rugaroo can type | 1 | getivar_module |
| an ivar assigned several types (`Array \| Integer \| String`) | 1 | poly_cells |
| `define_method` | 1 | send_bmethod |
| `return` out of a block (decision 4) | 1 | throw |

spinel compiles and runs all 74. The nil row is the design choice that costs
rugaroo the most programs here: it makes a program prove `a[i]` is not `nil`
before using it as the element type.
