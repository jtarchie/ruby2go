package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var flagDef = regexp.MustCompile(`\.(?:Bool|String|Int|Duration|Func)(?:Var)?\((?:&[\w.]+, )?"([\w-]+)"`)

// a flag the CLI defines cannot ship without its docs/cli.md row (decision 373)
func TestCLIDoc(t *testing.T) {
	doc, err := os.ReadFile("../../docs/cli.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"main.go", "web.go"} {
		src, err := os.ReadFile(f) //nolint:gosec // a constant list of this package's files
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range flagDef.FindAllStringSubmatch(string(src), -1) {
			if !strings.Contains(string(doc), "`-"+m[1]) {
				t.Errorf("docs/cli.md does not document -%s (defined in %s)", m[1], f)
			}
		}
	}
}
