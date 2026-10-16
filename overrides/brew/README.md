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

```sh
./trace info info wget      # MRI run under a load tracer -> entries/info.rb, dirs.txt
./mricheck info wget        # the patches change nothing: unpatched fork mode == patched spawn mode under MRI
./gen info                  # rb2go gen -> work/info.go (~2 min, ~3 GB)
./build info                # go build -> work/info-bin (~3 min, ~10 GB; killed past 14 GB)
./run info info wget        # run it in the container; compare with expected/info-wget.txt (MRI, warm API cache)
```

`brew info wget` output equals MRI's (decision 291).
