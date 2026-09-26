package compiler

import (
	"golang.org/x/tools/imports"
)

func formatGo(src []byte) ([]byte, error) {
	return imports.Process("main.go", src, &imports.Options{Comments: true, TabIndent: true, TabWidth: 8, FormatOnly: false})
}
