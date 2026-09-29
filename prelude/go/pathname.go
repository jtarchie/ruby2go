//go:build ignore

package prelude

// rbPathJoin joins two components like File.join: trim one slash at the seam, don't collapse "..".
func rbPathJoin(a, b string) string {
	switch {
	case a == "":
		return b
	case strings.HasSuffix(a, "/"):
		return a + strings.TrimLeft(b, "/")
	case strings.HasPrefix(b, "/"):
		return a + b
	default:
		return a + "/" + b
	}
}
