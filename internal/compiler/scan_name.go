package compiler

// Hand-written scanners for two regexps that ran over every verbatim Go block of the prelude on each compile; scan_name_test.go keeps both regexps as the oracle.

// isGoSpace is the regexp class \s.
func isGoSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }

func isGoWord(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isGoAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// declaredGoNames yields each package-level name a top-level Go block declares: `(?m)^\s*(?:func|type|var|const)\s+([A-Z]\w*)`, non-overlapping, leftmost.
func declaredGoNames(code string, yield func(string)) {
	n := len(code)
	for i := 0; i < n; { // i is a line start, where `^` can match; \s* may run over later lines
		k := i
		for k < n && isGoSpace(code[k]) {
			k++
		}
		from := k // without a match every line start up to k fails alike
		if name := declAt(code, k); name.start >= 0 {
			yield(code[name.start:name.end])
			from = name.end
		}
		nl := indexByteFrom(code, '\n', from)
		if nl < 0 {
			return
		}
		i = nl + 1
	}
}

type span struct{ start, end int }

// declAt matches `(?:func|type|var|const)\s+([A-Z]\w*)` at k: the name's span, or start -1.
func declAt(code string, k int) span {
	n := 0
	switch {
	case hasPrefixAt(code, k, "func"), hasPrefixAt(code, k, "type"):
		n = 4
	case hasPrefixAt(code, k, "var"):
		n = 3
	case hasPrefixAt(code, k, "const"):
		n = 5
	default:
		return span{-1, -1}
	}
	p := k + n
	s := p
	for p < len(code) && isGoSpace(code[p]) {
		p++
	}
	if p == s || p >= len(code) || code[p] < 'A' || code[p] > 'Z' {
		return span{-1, -1}
	}
	q := p + 1
	for q < len(code) && isGoWord(code[q]) {
		q++
	}
	return span{p, q}
}

func hasPrefixAt(s string, i int, p string) bool { return len(s)-i >= len(p) && s[i:i+len(p)] == p }

func indexByteFrom(s string, c byte, from int) int {
	for i := from; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// goCallNames yields the name of each Go call in code: `(?:\.|_)([A-Z][A-Za-z0-9]*)[(\[]` (`.Name(` or `Owner_Name(`; a field is not a call).
func goCallNames(code string, yield func(string)) {
	n := len(code)
	for i := 0; i < n; i++ {
		if c := code[i]; c != '.' && c != '_' {
			continue
		}
		p := i + 1
		if p >= n || code[p] < 'A' || code[p] > 'Z' {
			continue
		}
		q := p + 1
		for q < n && isGoAlnum(code[q]) {
			q++
		}
		if q < n && (code[q] == '(' || code[q] == '[') {
			yield(code[p:q])
			i = q // the bracket is consumed
		} else {
			i = q - 1 // a '_' ending the run can still start a match
		}
	}
}
