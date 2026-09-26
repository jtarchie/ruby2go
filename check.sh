#!/usr/bin/env bash
# Runs each example under MRI and Go; fails if stdout differs.
set -u
cd "$(dirname "$0")"
go vet ./... || exit 1
fail=0
for d in examples/*/; do
  want=$(ruby "$d/main.rb" 2>&1); rc_rb=$?
  got=$(go run "./$d" 2>&1);     rc_go=$?
  if [[ "$want" == "$got" && $rc_rb -eq $rc_go ]]; then
    echo "ok   $d"
  else
    echo "FAIL $d"; diff <(echo "$want") <(echo "$got"); fail=1
  fi
done
exit $fail
