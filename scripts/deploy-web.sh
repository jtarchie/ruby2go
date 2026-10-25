#!/bin/sh
# Static playground (decision 102): the wasm and examples go to the public R2 bucket under a content-hash prefix, the page that names it to a Top Banana site.
set -eu
cd "$(dirname "$0")/.." # paths below are repo-relative
node scripts/topbanana.check.mjs >/dev/null # the deploy guards (empty set, token) must hold before anything is uploaded
slug=${TOPBANANA_SLUG:-rugaroo}
out=dist/web
bucket=rugaroo
public=https://pub-372f595b0c2c4d7ebf282a54bd539231.r2.dev # ponytail: r2.dev is rate-limited; a custom domain on the bucket for real traffic

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o "$tmp/rugaroo.wasm" ./cmd/rugaroo-wasm
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$tmp/"
go run ./cmd/rugaroo web -static "$tmp/static" -assets "$public/x/"
cp "$tmp/static/examples.json" "$tmp/"
hash=$(cd "$tmp" && cat rugaroo.wasm wasm_exec.js examples.json | shasum -a 256 | cut -c1-12)

put() { # name, content type: stored gzipped, so the browser downloads a fraction and decodes it itself
	gzip -9 -c "$tmp/$1" >"$tmp/$1.gz"
	wrangler r2 object put "$bucket/$hash/$1" --file "$tmp/$1.gz" --ct "$2" --ce gzip --cc "public, max-age=31536000, immutable" --remote
}
put rugaroo.wasm application/wasm
put wasm_exec.js "text/javascript; charset=utf-8"
put examples.json "application/json"

go run ./cmd/rugaroo web -static "$tmp/static" -assets "$public/$hash/"
rm -rf "$out"
mkdir -p "$out"
cp "$tmp/static/index.html" "$out/"
echo "assets: $public/$hash/"
node scripts/topbanana.mjs deploy "$slug" "$out"
