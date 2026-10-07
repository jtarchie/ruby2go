package compiler

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// preludeLibs maps what a user `require` names to the prelude file that defines it (decision 155). Anything not here is
// core: loaded by prelude.rb whether required or not, as MRI 4.0 has it without a require (Set, Pathname, pp, Monitor,
// Process::Status, GC). A lib file require_relatives what MRI's own file loads, so `require "csv"` defines Date and
// StringIO as it does there. A name MRI has and this table lacks stays a no-op, as before.
var preludeLibs = map[string]string{
	"json":             "prelude/json.rb",
	"stringio":         "prelude/stringio.rb",
	"fileutils":        "prelude/fileutils.rb",
	"tempfile":         "prelude/tempfile.rb",
	"tmpdir":           "prelude/tmpdir.rb",
	"find":             "prelude/find.rb",
	"logger":           "prelude/logger.rb",
	"base64":           "prelude/base64.rb",
	"digest":           "prelude/digest.rb",
	"digest/md5":       "prelude/digest.rb",
	"digest/sha1":      "prelude/digest.rb",
	"digest/sha2":      "prelude/digest.rb",
	"zlib":             "prelude/zlib.rb",
	"securerandom":     "prelude/securerandom.rb",
	"cgi":              "prelude/cgi.rb",
	"cgi/escape":       "prelude/cgi.rb",
	"cgi/util":         "prelude/cgi.rb",
	"strscan":          "prelude/strscan.rb",
	"shellwords":       "prelude/shellwords.rb",
	"abbrev":           "prelude/abbrev.rb",
	"benchmark":        "prelude/benchmark.rb",
	"csv":              "prelude/csv.rb",
	"singleton":        "prelude/singleton.rb",
	"etc":              "prelude/etc.rb",
	"timeout":          "prelude/timeout.rb",
	"uri":              "prelude/uri.rb",
	"ipaddr":           "prelude/ipaddr.rb",
	"net/http":         "prelude/net_http.rb",
	"net/https":        "prelude/net_http.rb",
	"webrick":          "prelude/webrick.rb",
	"open-uri":         "prelude/open_uri.rb",
	"socket":           "prelude/socket.rb",
	"open3":            "prelude/open3.rb",
	"tsort":            "prelude/tsort.rb",
	"observer":         "prelude/observable.rb",
	"weakref":          "prelude/weakref.rb",
	"erb":              "prelude/erb.rb",
	"bigdecimal":       "prelude/bigdecimal.rb",
	"bigdecimal/util":  "prelude/bigdecimal.rb",
	"forwardable":      "prelude/forwardable.rb",
	"optparse":         "prelude/optparse.rb",
	"optionparser":     "prelude/optparse.rb",
	"minitest":         "prelude/minitest.rb",
	"minitest/autorun": "prelude/minitest.rb",
	"minitest/spec":    "prelude/minitest.rb",
	"minitest/mock":    "prelude/minitest.rb",
	"minitest/pride":   "prelude/minitest.rb",
	"rexml":            "prelude/rexml.rb",
	"rexml/document":   "prelude/rexml.rb",
	"ostruct":          "prelude/ostruct.rb",
	"delegate":         "prelude/delegate.rb",
	"date":             "prelude/date.rb",
	"time":             "prelude/time_parse.rb",
}

// scanRequires parses the user sources and walks them, and every file they require_relative or require off the -I
// path, for the literal `require "x"` names the program mentions anywhere (top level, in a method, inside
// `begin … rescue LoadError`): each loads at compile time before any user file is collected, so a lib's classes exist
// wherever its require stands, as decision 130 does for files. A non-literal argument at the top level is a compile
// error; inside a method it is a no-op, since a gem's computed require (Rack::Builder.parse_file's `require path`) is
// reached only at run time, when the closed world already holds what it names. A file with
// `__END__` needs StringIO for DATA. The parsed files come back keyed by real path, for the collect pass to reuse.
func (c *Compiler) scanRequires(ctx context.Context, sources []Source) (libs []string, parsed map[string]*File, err error) {
	sc := &requireScan{c: c, ctx: ctx, parsed: map[string]*File{}, seen: map[string]bool{}, top: true}
	for _, src := range sources {
		path := src.Path
		if path == "" {
			path = src.Name
		}
		path = realPath(path)
		if sc.seen[path] {
			continue
		}
		sc.seen[path] = true
		uf, perr := parseFile(ctx, c.parser, src.Name, src.Src, false)
		if perr != nil {
			return nil, nil, perr
		}
		uf.path = path
		sc.parsed[path] = uf
		sc.walk(uf)
	}
	return sc.libs, sc.parsed, nil
}

// requireScan is scanRequires' state: the files parsed so far by real path, and the lib names found in order.
type requireScan struct {
	c      *Compiler
	ctx    context.Context //nolint:containedctx // a short-lived walker
	parsed map[string]*File
	seen   map[string]bool // real paths walked, and "lib:name" for libs found
	libs   []string
	top    bool // at a file's top level (not inside a method): a computed require there is an error
}

func (sc *requireScan) walk(f *File) {
	if f.data != nil {
		sc.lib("stringio")
	}
	sc.top = true
	sc.visit(f, f.Root)
}

func (sc *requireScan) lib(name string) {
	if !sc.seen["lib:"+name] {
		sc.seen["lib:"+name] = true
		sc.libs = append(sc.libs, name)
	}
}

func (sc *requireScan) visit(f *File, n parser.Node) {
	if n == nil {
		return
	}
	if call, ok := n.(*parser.CallNode); ok && call.Receiver == nil && (call.Name == "require" || call.Name == "require_relative") {
		sc.require(f, call, sc.top)
		return
	}
	if _, ok := n.(*parser.DefNode); ok { // a method body: a computed require there is reached only at run time
		prev := sc.top
		sc.top = false
		for _, ch := range n.CompactChildNodes() {
			sc.visit(f, ch)
		}
		sc.top = prev
		return
	}
	for _, ch := range n.CompactChildNodes() {
		sc.visit(f, ch)
	}
}

// require handles one require/require_relative call: a lib name is recorded; a file is read, parsed and walked.
func (sc *requireScan) require(f *File, call *parser.CallNode, top bool) {
	args := callArgs(call)
	var str *parser.StringNode
	if len(args) == 1 {
		str, _ = args[0].(*parser.StringNode)
	}
	if str == nil {
		if top {
			sc.c.errorf(f, call, "%s needs a string literal: rb2go loads libraries at compile time", call.Name)
		}
		return // a computed require in a method: a no-op at run time
	}
	name := str.Unescaped.Value
	var target string
	if call.Name == "require_relative" {
		target = filepath.Join(filepath.Dir(f.path), filepath.FromSlash(name))
		if filepath.Ext(target) != ".rb" {
			target += ".rb"
		}
	} else if target = sc.c.findRequire(f, call); target == "" {
		sc.lib(name)
		return
	}
	path := realPath(target)
	if sc.seen[path] {
		return
	}
	sc.seen[path] = true
	src, err := os.ReadFile(target) //nolint:gosec // the user's program names it
	if err != nil {
		return // the collect pass reports it where it stands
	}
	uf, err := parseFile(sc.ctx, sc.c.parser, path, src, false)
	if err != nil {
		panic(compileError{msg: err.Error()})
	}
	uf.path = path
	sc.parsed[path] = uf
	sc.walk(uf)
}

// loadLibs loads the prelude files the program's requires name, in require order; every lib when all is set (the stub
// generator, decision 154, and RB2GO_ALL_LIBS=1 for comparing against the old always-loaded prelude).
func (c *Compiler) loadLibs(ctx context.Context, libs []string, all bool) {
	if all {
		for _, lib := range slices.Sorted(maps.Keys(preludeLibs)) {
			c.loadPrelude(ctx, preludeLibs[lib])
		}
		return
	}
	for _, lib := range libs {
		if file := preludeLibs[strings.TrimSuffix(lib, ".rb")]; file != "" {
			c.loadPrelude(ctx, file)
		}
	}
}
