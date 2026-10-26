# Command line

`rugaroo` is shaped like the `go` command. Flags go before the Ruby file;
anything after the file is passed to the program. Tiers are as in
[support.md](support.md).

| Command | Tier | What it does |
|---|---|---|
| `rugaroo run [flags] main.rb [args...]` | stable | Compile, build and run; exits with the program's status. |
| `rugaroo build [flags] [-o prog] main.rb` | stable | Write a binary (default: the file's name without `.rb`). |
| `rugaroo gen [flags] main.rb > main.go` | stable | Print the generated Go. Needs no `go` command. |
| `rugaroo test [flags] [paths...] [minitest flags]` | stable | Compile minitest files into one binary and run it. |
| `rugaroo web [-addr host:port]` | experimental | A local playground: Ruby in, Go out, a Run button. |
| `rugaroo web -static dir -assets URL` | experimental | Write the same playground as static files that compile in the browser. |

## Build flags

For `run`, `build` and `gen`; `test` takes `-I`, `-int-overflow`, `-race`,
`-gcflags` and `-work`.

| Flag | Tier | What it does |
|---|---|---|
| `-o prog` | stable | `build` only: the output binary. |
| `-I dir` | stable | A directory `require` searches for your own files, as `ruby -I`. Repeatable. |
| `-int-overflow mode` | stable (`raise`, `wrap`), experimental (`promote`) | What an Integer result past 64 bits does: `raise` a `RangeError` (the default), `wrap` around as two's complement, or `promote` to a Bignum as MRI does. A `# int_overflow: mode` line in a file does the same for the arithmetic written in that file, whatever the flag says (decision 381). |
| `-race` | stable | Build with Go's race detector. |
| `-gcflags flags` | stable | Passed to `go build`; `-gcflags=-e` shows every Go error when generated code fails to build (please report those). |
| `-work` | stable | Keep the temporary module (`main.go`, `go.mod`) and print its path as `WORK=...`. |
| `-trace` | experimental | Run the program once under MRI, with the arguments after the file, and compile every file that run loaded. For gems that `require` files by computed name. **The run executes the program.** |
| `-trace-also "args"` | experimental | With `-trace`: one more traced run with these arguments, merged into the same binary, for a program that serves several commands. Repeatable. |
| `-rubyopt opt` | experimental | With `-trace`: an option for the MRI run, as the program's own launcher would pass it. Repeatable. |
| `-main file` | experimental | Compile from a traced entry file (the requires an MRI run loaded) instead of tracing again; `file` is the program that entry runs. |

## `rugaroo test`

Test files are found as minitest finds them (`test_*.rb`, `*_test.rb`,
`spec_*.rb`, `*_spec.rb`); with no path, `test/`. All of them load into one
program, as a Ruby test run loads them into one process. minitest's own flags
pass through: `-v`, `-s seed` (`--seed`), `-n pattern` (`--name`), `-e
pattern` (`--exclude`), `-S kinds` (`--skip`), `--show-skips`.

## `rugaroo web`

| Flag | What it does |
|---|---|
| `-addr host:port` | Listen address (default `127.0.0.1:8080`). The Run button executes code, so keep it on loopback; requests from other hosts and origins are refused. |
| `-static dir` | Instead of serving, write a static playground to `dir`: `index.html` for any web host, plus the files for `-assets`. |
| `-assets URL` | With `-static`: the URL, ending in `/`, that serves `rugaroo.wasm`, `wasm_exec.js` and `examples.json`. |

## Environment variables

For a built program:

| Variable | What it does |
|---|---|
| `RUGAROO_BACKTRACE=1` | Report an uncaught exception as MRI does, with `file:line` and `from` lines. |
| `RUGAROO_GO_STACK=1` | A Go runtime panic turned into a Ruby exception also carries its Go stack. |

For the compiler:

| Variable | What it does |
|---|---|
| `RUGAROO_INT_OVERFLOW=mode` | The same as `-int-overflow`. |
| `RUGAROO_GEM_WARNINGS=1` | Also print warnings from gem files (hidden by default, since they are not yours to fix). |
| `RUGAROO_UNTYPED_WHY=1` | Each dynamic-call warning also names where the untyped value came from. |
| `RUGAROO_INFER_DEBUG=1` | Show what each round of parameter inference typed and what it could not compile. |

The repository's [CLAUDE.md](../CLAUDE.md) lists the variables for working on
rugaroo itself.
