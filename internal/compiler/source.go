package compiler

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// File is one parsed Ruby source file.
type File struct {
	Name    string // as shown in //line directives and errors
	Src     []byte
	Root    *parser.ProgramNode
	lines   []int // byte offset of each line start
	prelude bool

	// standalone annotation comments by the line they precede (first code line after)
	leading map[int][]comment
	// trailing `#: T` comments by line
	trailing map[int]string
}

type comment struct {
	line int
	text string // without leading `#`
}

func parseFile(ctx context.Context, p *parser.Parser, name string, src []byte, prelude bool) (*File, error) {
	res, err := p.Parse(ctx, src)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	f := &File{Name: name, Src: src, Root: res.Value, prelude: prelude}
	f.lines = []int{0}
	for i, b := range src {
		if b == '\n' {
			f.lines = append(f.lines, i+1)
		}
	}
	for _, e := range res.Errors {
		return nil, fmt.Errorf("%s:%d: syntax error: %s", name, f.line(e.Location.StartOffset), e.Message)
	}
	f.indexComments(res.Comments)
	return f, nil
}

func (f *File) line(off int) int {
	return sort.Search(len(f.lines), func(i int) bool { return f.lines[i] > off })
}

func (f *File) text(loc parser.Location) string {
	return string(f.Src[loc.StartOffset : loc.StartOffset+loc.Length])
}

func (f *File) lineText(n int) string {
	start := f.lines[n-1]
	end := len(f.Src)
	if n < len(f.lines) {
		end = f.lines[n] - 1
	}
	return string(f.Src[start:end])
}

// indexComments splits comments into "trailing" (code before them on the
// same line) and "leading" blocks that attach to the next code line.
func (f *File) indexComments(cs []parser.Comment) {
	f.leading = map[int][]comment{}
	f.trailing = map[int]string{}
	isComment := map[int]bool{}
	for _, c := range cs {
		ln := f.line(c.Location.StartOffset)
		before := strings.TrimSpace(string(f.Src[f.lines[ln-1]:c.Location.StartOffset]))
		text := strings.TrimPrefix(f.text(c.Location), "#")
		if before != "" {
			if strings.HasPrefix(text, ":") {
				f.trailing[ln] = strings.TrimSpace(text[1:])
			} else if strings.HasPrefix(text, "[") {
				f.trailing[ln] = text // include Foo #[E]
			}
			continue
		}
		isComment[ln] = true
	}
	// Group consecutive standalone comment lines; attach to next code line.
	var block []comment
	for _, c := range cs {
		ln := f.line(c.Location.StartOffset)
		if !isComment[ln] {
			continue
		}
		text := strings.TrimPrefix(f.text(c.Location), "#")
		block = append(block, comment{line: ln, text: text})
		next := ln + 1
		if isComment[next] {
			continue
		}
		// skip blank lines
		for next <= len(f.lines) && strings.TrimSpace(f.lineText(next)) == "" {
			next++
		}
		f.leading[next] = append(f.leading[next], block...)
		block = nil
	}
}

// sigComment returns the `#: ...` / `# @rbs <type>` annotation attached to
// the code line ln, or "".
func (f *File) sigComment(ln int) (string, bool) {
	var sig string
	found := false
	for _, c := range f.leading[ln] {
		t := c.text
		switch {
		case strings.HasPrefix(t, ":"):
			sig = strings.TrimSpace(t[1:])
			found = true
		case strings.HasPrefix(strings.TrimSpace(t), "@rbs "):
			rest := strings.TrimSpace(strings.TrimSpace(t)[5:])
			// only method-type shaped annotations: start with ( or [
			if strings.HasPrefix(rest, "(") || strings.HasPrefix(rest, "[") {
				sig = rest
				found = true
			}
		case strings.HasPrefix(t, "|"):
			found = true
			sig = "" // overload: unsupported; caller errors
		}
	}
	return sig, found
}

// annotations returns the `@rbs ...` and `@go_type ...` directives attached
// to the code line ln.
func (f *File) annotations(ln int) map[string][]string {
	out := map[string][]string{}
	for _, c := range f.leading[ln] {
		t := strings.TrimSpace(c.text)
		switch {
		case strings.HasPrefix(t, "@rbs "):
			fields := strings.Fields(t[5:])
			if len(fields) == 0 {
				continue
			}
			key := fields[0]
			if strings.HasPrefix(key, "@") { // @rbs @x: T
				out["ivar"] = append(out["ivar"], strings.TrimSpace(t[5:]))
				continue
			}
			out[key] = append(out[key], strings.TrimSpace(strings.TrimPrefix(t[5:], key)))
		case strings.HasPrefix(t, "@go_type "):
			out["go_type"] = append(out["go_type"], strings.TrimSpace(t[9:]))
		}
	}
	return out
}
