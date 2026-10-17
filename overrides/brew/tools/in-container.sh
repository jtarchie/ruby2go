#!/bin/bash
# container: brew behind tools/ruby-wrapper (decision 296). usage: in-container.sh gen | check | run <args> | hits <args>
#   gen    rb2go gen -trace of every targets.txt line (one binary serves them all) -> /w/work/brew.go
#   check  each targets.txt line under MRI (unpatched, fork mode) and under /w/work/brew-bin; diffs in /w/work
#   run    brew <args> through /w/work/brew-bin
#   hits   brew <args> under MRI with tools/calltrace.rb (patched, spawn mode) -> /w/work/calls.txt, lines.txt
R=/home/linuxbrew/.linuxbrew/Homebrew B=/home/linuxbrew/.linuxbrew/bin/brew
export HOMEBREW_NO_AUTO_UPDATE=1 PATH="/usr/local/go/bin:$PATH"
cd /home/linuxbrew
cmd=$1; shift
$B info wget > /dev/null 2>&1 # download the API once: its banner prints only then
if [ "$cmd" = check ]; then
  while IFS= read -r t || [ -n "$t" ]; do
    # shellcheck disable=SC2086
    $B $t > "/tmp/mri-${t// /_}.txt" 2>&1; echo "exit $?" >> "/tmp/mri-${t// /_}.txt"
  done < /w/targets.txt
  # the MRI pass installed wget: reset so the compiled pass installs (and downloads) it afresh
  $B uninstall --force --ignore-dependencies $($B list --formula) > /dev/null 2>&1
  rm -rf "$HOME/.cache/Homebrew/downloads"
fi
for p in /w/patches/*.patch; do git -C $R apply "$p" || { echo "patch failed: $p"; exit 1; }; done
/w/tools/install-wrapper $R
case $cmd in
gen)
  mkdir -p /w/work/gotmp
  export TMPDIR=/w/work/gotmp GOTMPDIR=/tmp HOMEBREW_RB2GO_EXE=/w/work/rb2go HOMEBREW_RB2GO_CMD=gen
  export HOMEBREW_RB2GO_ALSO="$(tail -n +2 /w/targets.txt)" HOMEBREW_RB2GO_GEM_WARNINGS=$RB2GO_GEM_WARNINGS
  export HOMEBREW_RB2GO_UNTYPED_WHY=$RB2GO_UNTYPED_WHY HOMEBREW_RB2GO_GOPOISON_DUMP=$RB2GO_GOPOISON_DUMP
  start=$(date +%s)
  # shellcheck disable=SC2046
  $B $(head -1 /w/targets.txt) > /w/work/brew.go 2> /w/work/brew.go.log
  rc=$?; cp /tmp/rb2go-bad-output-*.go /w/work/gotmp/ 2>/dev/null # the TMPDIR brew hands on is not ours
  echo "gen exit $rc elapsed $(( $(date +%s)-start ))s"
  ;;
check)
  fail=0
  while IFS= read -r t || [ -n "$t" ]; do
    s=${t// /_}
    # shellcheck disable=SC2086
    HOMEBREW_RB2GO_BIN=/w/work/brew-bin HOMEBREW_RB2GO_BACKTRACE=1 timeout 300 $B $t > /tmp/rb.txt 2>&1; echo "exit $?" >> /tmp/rb.txt
    if diff "/tmp/mri-$s.txt" /tmp/rb.txt > "/w/work/diff-$s.txt"; then echo "ok   brew $t"; else echo "FAIL brew $t"; fail=1; fi
  done < /w/targets.txt
  exit $fail
  ;;
run) HOMEBREW_RB2GO_BIN=/w/work/brew-bin HOMEBREW_RB2GO_BACKTRACE=1 exec timeout 300 $B "$@" ;;
hits) HOMEBREW_RB2GO_MRI=1 HOMEBREW_RB2GO_MRI_REQUIRE=/w/tools/calltrace.rb HOMEBREW_RB2GO_CALLTRACE_DIR=/w/work $B "$@" > /dev/null 2>&1 ;;
esac
