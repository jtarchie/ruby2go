package compiler

import "fmt"

// srcPos is a file and line (a declaration's, or File.shownLine for generated source), held as data so compileError.Error composes the `name:line` text once (decision 276).
type srcPos struct {
	File *File
	Line int
}

// String is `name:line`, or empty when there is no file.
func (p srcPos) String() string {
	if p.File == nil {
		return ""
	}
	return fmt.Sprintf("%s:%d", p.File.Name, p.Line)
}

// errorAt aborts compilation with a message located at pos. A pos without a file gives a message with no location.
func (c *Compiler) errorAt(pos srcPos, format string, args ...any) {
	panic(compileError{pos: pos, msg: fmt.Sprintf(format, args...)})
}

// warnAt is warn for a message located at pos: it is prefixed `name:line: `.
func (c *Compiler) warnAt(pos srcPos, format string, args ...any) {
	c.addWarning(pos.File, pos.String()+": "+fmt.Sprintf(format, args...))
}
