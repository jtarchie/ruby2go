# Roundhouse's Blog, compiled

`blog/` is a Rails app after [roundhouse](https://rubys.github.io/roundhouse/)
turned it into plain Ruby for [Spinel](https://github.com/matz/spinel):
articles with nested comments, validations, partials, JSON views and Turbo
Stream broadcasts, with no Rails left in it. rugaroo compiles it into one Go
binary. `main.rb` serves it on a local port and sends it fifteen requests, and
the binary's answers equal MRI's on the same files.

Nothing in `blog/` is edited. Its few Spinel-only files (the tep HTTP server,
sqlite and crypto over Spinel's FFI) are replaced by the
[roundhouse-rugaroo](https://github.com/jtarchie/roundhouse-rugaroo) gem, which
`Gemfile` pulls from GitHub, and the gem's `sig/` adds the types roundhouse's own
`.rbs` files leave to Spinel's inference ([decisions 404](../../docs/decisions/404.md)
and [405](../../docs/decisions/405.md)).

```sh
bundle install --gemfile examples/135_roundhouse_blog/Gemfile   # once
ruby main.rb                     # MRI
rugaroo run -gem blog/runtime main.rb   # compiled: blog/runtime is library code the app carries, compiled as a gem is
```

`main.rb` marks the Spinel-only files loaded before anything requires them
(`$LOADED_FEATURES << File.expand_path("blog/runtime/db.rb", __dir__)`), then
requires the gem, whose files stand in for them. MRI and rugaroo both then skip
the originals.

## `main.rb`

- `# gem: blog/runtime` (the harness's `rugaroo build -gem blog/runtime`,
  [decision 406](../../docs/decisions/406.md)) and Bundler: roundhouse's runtime
  is a library the app carries, and the gem comes through Bundler
  (`ENV["BUNDLE_GEMFILE"]` and `require "bundler/setup"`, as a Rails app's
  `config/boot.rb` does), so rugaroo compiles both as it compiles a gem: only
  what the blog reaches. `blog/app/` stays the program's own code, strictly
  typed. The `$LOAD_PATH.unshift` line is for `ruby main.rb`.
- `# nil_check: runtime`: the emitted code uses possibly-nil values where it
  knows they are not nil, as Ruby lets it. rugaroo checks those at run time
  instead of refusing to compile them.
- The two importmap files load before `blog/main.rb`. Roundhouse's boot loads
  the views (whose layout computes the importmap tags once) before the app's
  importmap, so under MRI the layout would get the runtime's empty default.
  Spinel, rugaroo and Rails all give it the app's pins.
- Each run seeds a fresh database in a temporary directory from
  `blog/db/seed.sql`, so ids and pages match between runs.

## Regenerating `blog/`

```sh
# the Rails app: roundhouse's scripts/create-blog (rails new + two scaffolds + its edits), Rails 8.1.4
RAILS_NEW_VERSION=8.1.4 path/to/roundhouse/scripts/create-blog blog
roundhouse --target spinel -o out blog   # roundhouse 2026.9.18 (2e286e6f)
cp -R out/{app,config,db,runtime,public,main.rb,boot.rb} examples/135_roundhouse_blog/blog/
find examples/135_roundhouse_blog/blog -name '*.map' -delete
```

The tests, end-to-end specs, tools and build files roundhouse also writes are
left out. Roundhouse 2026.9.18 drops the `format.json` arms of the
controllers' `respond_to` with a warning; the `.json` routes are served by the
JSON views.
