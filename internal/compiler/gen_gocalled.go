package compiler

import "bytes"

// goSelectorCalls replaces the regexp `(\b[A-Za-z_]\w*)?\.([A-Z_]\w*)\(` (14-28% of every compile in its backtracker); recv is empty unless a word-bounded identifier precedes the dot.
func goSelectorCalls(src []byte, fn func(recv, name []byte)) {
	for i := 0; i < len(src); {
		d := bytes.IndexByte(src[i:], '.')
		if d < 0 {
			return
		}
		d += i
		i = d + 1
		if i >= len(src) || (src[i] < 'A' || src[i] > 'Z') && src[i] != '_' {
			continue
		}
		j := i + 1
		for j < len(src) && isASCIIWord(src[j]) {
			j++
		}
		if j >= len(src) || src[j] != '(' {
			continue
		}
		s := d
		for s > 0 && isASCIIWord(src[s-1]) {
			s--
		}
		recv := src[s:d]
		if len(recv) > 0 && recv[0] >= '0' && recv[0] <= '9' {
			recv = nil // \b then [A-Za-z_]: a digit-led run is no identifier
		}
		fn(recv, src[i:j])
		i = j + 1
	}
}

func isASCIIWord(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_'
}
