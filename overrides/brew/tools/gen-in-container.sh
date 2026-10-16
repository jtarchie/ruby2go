#!/bin/bash
# container: apply patches/, then rb2go gen -main brew.rb entries/$1.rb > work/$1.go; peak RSS on stdout.
while IFS= read -r line; do export "$line"; done < /w/env.txt
export PATH="/usr/local/go/bin:/home/linuxbrew/.linuxbrew/Homebrew/Library/Homebrew/vendor/portable-ruby/current/bin:$PATH"
I=""
while read -r d || [ -n "$d" ]; do I="$I -I $d"; done < /w/dirs.txt
for p in /w/patches/*.patch; do git -C /home/linuxbrew/.linuxbrew/Homebrew apply "$p" || { echo "patch failed: $p"; exit 1; }; done
cd /home/linuxbrew/.linuxbrew/Homebrew/Library/Homebrew
mkdir -p /w/work/gotmp; export TMPDIR=/w/work/gotmp GOTMPDIR=/tmp
start=$(date +%s)
/w/work/rb2go gen $I -main "$PWD/brew.rb" "/w/entries/$1.rb" > "/w/work/$1.go" 2> "/w/work/$1.go.log" &
pid=$!; peak=0
while kill -0 $pid 2>/dev/null; do r=$(awk '/VmHWM/{print $2}' /proc/$pid/status 2>/dev/null); [ -n "$r" ] && peak=$r; sleep 1; done
wait $pid; echo "gen exit $? elapsed $(( $(date +%s)-start ))s peakRSS ${peak}kB"
