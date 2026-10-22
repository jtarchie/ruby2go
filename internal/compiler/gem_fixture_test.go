package compiler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// compileGemApp compiles app against the gem files (name -> source, under lib/) in a temp dir, as a boot-mode gem program.
func compileGemApp(t *testing.T, gem map[string]string, app string) (string, error) {
	t.Helper()
	_, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("boot needs ruby")
	}
	dir := t.TempDir()
	lib := filepath.Join(dir, "lib")
	err = os.MkdirAll(lib, 0o750)
	if err != nil {
		t.Fatal(err)
	}
	for name, src := range gem {
		err = os.WriteFile(filepath.Join(lib, name), []byte(src), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}
	app = "# load_path: lib\n" + app
	appPath := filepath.Join(dir, "app.rb")
	err = os.WriteFile(appPath, []byte(app), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	fsys := os.DirFS("../..")
	out, _, err := CompileWithBoot(context.Background(), fsys, fsys, []Source{{Name: "app.rb", Src: []byte(app), Path: appPath}}, []string{lib})
	return string(out), err
}
