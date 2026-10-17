# overrides/brew

Compiles Homebrew's `brew` with rb2go (#100). This is the pattern for compiling any large upstream Ruby program:
pin the upstream, keep a small series of upstream-shaped patches, and compare against MRI.

- `Dockerfile`: the upstream program (`ghcr.io/homebrew/brew`, checked out at the commit the patches apply to) plus Go.
  Build it with `docker build -t rb2go-brew-dev .` (arm64; Colima on macOS).
- `patches/*.patch`: `git apply` series against that commit, applied before every compile and MRI check. Each one
  replaces a dynamic Ruby feature brew leans on (per-object `extend`, untyped blocks, ...) with ordinary typed Ruby
  that behaves the same under MRI. They are written to be upstream PRs (ask before opening one).
- `entries/<name>.rb`: what to compile for one command: the `require`s MRI loaded running it, in order (decision 197).
  `brew.rb` itself is the program's main (`rb2go gen -main`, decision 288).
- `overlay/`: whole-file replacements still named by `# rb2go_replace:` in the entries. To be removed.
- `env.txt`: the environment brew's bash launcher exports, plus `HOMEBREW_SPAWN_SYSTEM=1` (Go cannot fork).
- `dirs.txt`: `-I` load path for the vendored gems (written by `./trace`).
- `work/` (ignored): rb2go binary, generated Go, built binaries, logs.
- `host` + `tools/ruby-wrapper`: brew on this machine (macOS or Linux), no container and no entries. `./host` clones
  your Homebrew at the pinned commit into `work/homebrew`, applies the patches, and puts the wrapper in place of its
  portable Ruby. Its `bin/brew` then runs `rb2go run -trace brew.rb <args>`: rb2go runs the command once under MRI to
  learn what it loads, compiles that, and runs the binary (decision 296).

```sh
./trace info info wget      # MRI run under a load tracer -> entries/info.rb, dirs.txt
./mricheck info wget        # the patches change nothing: unpatched fork mode == patched spawn mode under MRI
./gen info                  # rb2go gen -> work/info.go (~2 min, ~3 GB)
./build info                # go build -> work/info-bin (~3 min, ~10 GB; killed past 14 GB)
./run info info wget        # run it in the container
./check info                # each targets.txt line: compiled output == MRI's (same container, same API data)
./hits info info --json=v2 wget  # work list: stubs (gem methods/arms that did not compile) MRI reaches running it
```

```sh
./host                                              # once: work/homebrew, a patched copy of $(brew --repository)
export HOMEBREW_NO_AUTO_UPDATE=1
work/homebrew/bin/brew info wget                    # compiled: trace, gen, go build (~5-15 min, ~9 GB), run
HOMEBREW_RB2GO_MRI=1 work/homebrew/bin/brew info wget          # the same copy under MRI
HOMEBREW_RB2GO_BIN=$PWD/work/mac-bin work/homebrew/bin/brew info wget  # build once into that path, then rerun it (<1 s)
HOMEBREW_RB2GO_CMD=gen HOMEBREW_RB2GO_GEM_WARNINGS=1 work/homebrew/bin/brew info wget > work/mac.go  # the Go only
./host-hits info wget                               # ./hits for this copy: stubs MRI reaches (work/hits-mac.txt)
```

`HOMEBREW_RB2GO_X` reaches rb2go as `RB2GO_X` (brew.sh passes on only `HOMEBREW_*`); `HOMEBREW_RB2GO_FLAGS` adds rb2go
flags (`-work`, `-gcflags=all=-dwarf=false`). A binary built with `HOMEBREW_RB2GO_BIN` is the closed world of the
command it traced; another command can reach code it never compiled.

All eight targets.txt commands (`brew info` for four formulae, `--json=v2 wget` and three casks) equal MRI's output
in the container (decision 295), and on macOS `brew info wget` does through `./host` (decision 296).
