# Roundhouse's Blog, compiled

`blog/` is a Rails app after [roundhouse](https://rubys.github.io/roundhouse/)
turned it into plain Ruby for [Spinel](https://github.com/matz/spinel):
articles with nested comments, validations, partials, JSON views and Turbo
Stream broadcasts, with no Rails left in it. rugaroo compiles it into one Go
binary. `main.rb` serves it on a local port and sends it fifteen requests, and
the binary's answers equal MRI's on the same files.

Nothing in `blog/` is edited. Its few Spinel-only files are replaced by the
shims in `shims/`, and `sig/` adds the types roundhouse's own `.rbs` files leave
to Spinel's inference ([decisions 404](../../docs/decisions/404.md) and
[405](../../docs/decisions/405.md)).

```sh
ruby main.rb                     # MRI
rugaroo run -trace main.rb       # compiled; -trace because blog/runtime compiles as a gem does
```

## Shims

`main.rb` marks a Spinel-only file loaded before anything requires it
(`$LOADED_FEATURES << File.expand_path("blog/runtime/db.rb", __dir__)`), then
requires the shim that stands in for it. MRI and rugaroo both then skip the
original.

| Shim | Stands in for | Why |
|---|---|---|
| `shims/tep_shim.rb` | `blog/runtime/tep/` (tep, Spinel's HTTP and WebSocket server) | tep reads sockets through Spinel's FFI. The shim keeps the part of Tep the blog calls (`Tep::Request`, `Tep::Response`, `Tep::APP.dispatch`, `Tep::Server::Threaded#run`) on `TCPServer`, one thread per connection, and parses requests with tep's own `Tep::Parser`. A WebSocket upgrade (Action Cable) answers 501 for now. |
| `shims/db_shim.rb` | `blog/runtime/db.rb` (sqlite through Spinel's FFI) | The same `Db` API on the `sqlite3` gem, which rugaroo compiles as [sqlite3-fiddle](../../docs/gems.md#vendored-gems). |
| `shims/ffi_shim.rb` | the `ffi_func` lines in `blog/runtime/message_digest.rb` and `blog/runtime/csrf_token.rb` | Those files stay as they are: `ffi_func` does nothing here, and the shim writes the four crypto functions and the random token they declare in Ruby. |

## Types (`sig/`)

Roundhouse writes a `.rbs` beside most files. rugaroo reads them, and `sig/`
here, which wins where both speak, adds what they leave out:

| File | Says |
|---|---|
| `sig/roundhouse.rbs` | `Roundhouse::ParamValue`, a type the sidecars name but never declare. |
| `sig/active_record.rbs` | The adapter slot holds the `SqliteAdapter` module, and `Base.find` returns the model it is called on (`-> instance`). |
| `sig/main.rbs` | `blog/main.rb` has no sidecar; its request path's types, as its comments describe them. |
| `sig/broadcasts.rbs` | `Broadcasts::TRANSPORTS` holds two unrelated classes in turn (a seed, then Cable's). |

## `main.rb`

- `# trace:` and `$LOAD_PATH`: `blog/runtime/` and `shims/` are on the load
  path, so rugaroo compiles them as it compiles a gem: only what the blog
  reaches. `blog/app/` stays the program's own code, strictly typed.
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
