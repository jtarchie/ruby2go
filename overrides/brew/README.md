# overrides/brew

Compiles Homebrew's `brew` with rugaroo (#100). This is the pattern for compiling any large upstream Ruby program:
pin the upstream, keep a small series of upstream-shaped patches, and compare against MRI.

- `Dockerfile`: the upstream program (`ghcr.io/homebrew/brew`, checked out at the commit the patches apply to) plus Go.
  Build it with `docker build -t rugaroo-brew-dev .` (arm64; Colima on macOS).
- `patches/*.patch`: `git apply` series against that commit, applied before every compile and MRI check. Each one
  replaces a dynamic Ruby feature brew leans on (per-object `extend`, untyped blocks, ...) with ordinary typed Ruby
  that behaves the same under MRI. They are written to be upstream PRs (ask before opening one).
- `tools/ruby-wrapper`: put in place of brew's portable Ruby by `tools/install-wrapper`. brew's own `bin/brew` sets
  up its environment as always, then runs `brew.rb` through `rugaroo run -trace`: rugaroo runs the command once under MRI
  to learn what it loads, compiles that, and runs the binary (decision 296). No entry file, load-path list or
  replacement files: brew's boot files compile as they are.
- `gem-patches/<gem>-<version>.patch`: applied with `patch -p1` to that vendored gem before every compile and MRI check.
  Each one is a few lines where a gem relies on what rugaroo's value Strings cannot do (an edit through an alias, a
  character count of binary data); none changes behaviour under MRI (decision 324).
- `targets.txt`: the commands `./check` compares. `./gen` traces all of them into one program (`-trace-also`).
  `TARGETS=/w/install-targets.txt ./gen` (and `./check`) uses another list: `install-targets.txt` holds
  `install wget`. `./check` keeps both outputs in `work/mri-*.txt` and `work/rb-*.txt`.
- `work/` (ignored): rugaroo binary, generated Go, built binaries, logs, and `work/homebrew` (`./host`).

In the container (Linux):

```sh
./mricheck info wget        # the patches change nothing: unpatched fork mode == patched spawn mode under MRI
./gen                       # trace every targets.txt line, rugaroo gen -> work/brew.go (~2 min, ~3 GB)
./build                     # go build -> work/brew-bin (~6 min, ~9 GB; killed past 22 GB)
./check                     # each targets.txt line: compiled output == MRI's (same container, same API data)
./run info wget             # one command through work/brew-bin
./hits info --json=v2 wget  # work list: stubs (gem methods/arms that did not compile) MRI reaches running it
```

On this machine (macOS or Linux), `./host` clones your Homebrew at the pinned commit into `work/homebrew`, applies the
patches and installs the wrapper; the installed Homebrew is never touched:

```sh
./host                                              # once: work/homebrew, a patched copy of $(brew --repository)
export HOMEBREW_NO_AUTO_UPDATE=1
work/homebrew/bin/brew info wget                    # compiled: trace, gen, go build (~5-15 min, ~9 GB), run
HOMEBREW_RUGAROO_MRI=1 work/homebrew/bin/brew info wget          # the same copy under MRI
HOMEBREW_RUGAROO_BIN=$PWD/work/mac-bin work/homebrew/bin/brew info wget  # build once into that path, then rerun it (<1 s)
HOMEBREW_RUGAROO_CMD=gen HOMEBREW_RUGAROO_GEM_WARNINGS=1 work/homebrew/bin/brew info wget > work/mac.go  # the Go only
./host-hits info wget                               # ./hits for this copy: stubs MRI reaches (work/hits-mac.txt)
```

`HOMEBREW_RUGAROO_X` reaches rugaroo as `RUGAROO_X` (brew.sh passes on only `HOMEBREW_*`); `HOMEBREW_RUGAROO_FLAGS` adds rugaroo
flags (`-work`, `-gcflags=all=-dwarf=false`). A binary built with `HOMEBREW_RUGAROO_BIN` is the closed world of the
commands it traced; `HOMEBREW_RUGAROO_ALSO` (one argument list per line) traces more of them into it.

All eight targets.txt commands (`brew info` for four formulae, `--json=v2 wget` and three casks) equal MRI's output
in the container (decision 295), and on macOS `brew info wget` does through `./host` (decision 296).

`brew install wget` on macOS (decision 323): wget's bottles pour only into `/opt/homebrew`, so `./macgen` links
`/opt/homebrew/bin/brew-rugaroo` to `work/homebrew/bin/brew` (the prefix comes from the link's directory, the repository
from its target) and uninstalls wget with your Homebrew before each run; the trace re-installs it under MRI.

```sh
./macgen build      # trace `install wget`, build work/mac-bin (~3 min)
./macrun            # uninstall wget, then `brew install wget` through work/mac-bin: work/mac-install-rb.txt
./macgen-hits       # the reached-stub work list for that command (work/hits-mac.txt)
rm /opt/homebrew/bin/brew-rugaroo                    # when done
```

`brew tap jtarchie/rugaroo https://github.com/jtarchie/rugaroo`, `brew trust jtarchie/rugaroo` and `brew install rugaroo`
(this repository's own cask) on macOS (decision 374): the tap loads every brew command file (its completion list), and the
install reads the cask with a run-time `instance_eval`, which the trace records and the binary runs compiled. Only the
traced cask text compiles in; a new release of the tap needs a new build. `tools/git-https` is the git brew runs here:
it skips the global gitconfig, whose `url.insteadOf` rewrites GitHub https to ssh.

```sh
./tapgen build      # untap, untrust, uninstall; trace the three commands, build work/rg-bin (~8 min)
./taprun            # the three commands under MRI, then through work/rg-bin, each from the same state; per-stream diffs
rm /opt/homebrew/bin/brew-rugaroo                    # when done
```
