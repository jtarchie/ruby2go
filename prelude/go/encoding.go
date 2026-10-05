//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbStdoutConv is $stdout's write conversion for Kernel#puts/print/p once `$stdout.set_encoding` names one; a func var so other programs carry no transcoder.
var rbStdoutConv func(string) string

// nil is MRI's start: default_external answers UTF-8, default_internal nil.
var rbEncDefaultExternal, rbEncDefaultInternal *Encoding

// Strings carry no encoding (decision 105), so these names matter only to encode, Integer#chr and IO (decision 136).
var rbEncAliases = map[string]string{
	"ASCII-8BIT": "ASCII-8BIT", "BINARY": "ASCII-8BIT",
	"UTF-8": "UTF-8", "CP65001": "UTF-8", "LOCALE": "UTF-8", "EXTERNAL": "UTF-8", "FILESYSTEM": "UTF-8",
	"US-ASCII": "US-ASCII", "ASCII": "US-ASCII", "ANSI_X3.4-1968": "US-ASCII", "646": "US-ASCII",
	"UTF-16BE": "UTF-16BE", "UCS-2BE": "UTF-16BE", "UTF-16LE": "UTF-16LE",
	"UTF-32BE": "UTF-32BE", "UCS-4BE": "UTF-32BE", "UTF-32LE": "UTF-32LE", "UCS-4LE": "UTF-32LE",
	"UTF-16": "UTF-16", "UTF-32": "UTF-32",
	"ISO-8859-1": "ISO-8859-1", "ISO8859-1": "ISO-8859-1",
}

// rbEncCanon is name's canonical spelling, "" for an encoding rb2go lacks.
func rbEncCanon(name string) string { return rbEncAliases[strings.ToUpper(name)] }

// rbEncNameArg is the name an encoding argument gives: a String as is, an Encoding's own.
func rbEncNameArg(x any) string {
	switch v := rbUnbox(x).(type) {
	case String:
		return string(v)
	case *Encoding:
		return v.name
	}
	panic(rbConvError(x, "String"))
}

// rbEncUnicode reports whether enc is one of the Unicode encodings (U+FFFD is encode's default replacement there, "?" elsewhere).
func rbEncUnicode(enc string) bool { return strings.HasPrefix(enc, "UTF-") }

// rbEncASCIICompat is Encoding#ascii_compatible?: ASCII bytes mean ASCII characters.
func rbEncASCIICompat(enc string) bool {
	return !strings.HasPrefix(enc, "UTF-16") && !strings.HasPrefix(enc, "UTF-32")
}

// What decoding one character found.
const (
	rbDecOK         = iota
	rbDecInvalid    // n bytes no character starts with; again more were read to see it
	rbDecIncomplete // the input ends inside a character
	rbDecUndef      // a byte with no Unicode character (ASCII-8BIT's upper half)
)

// rbDecode1 never sees a dummy UTF-16/UTF-32: the BOM picks LE or BE first.
func rbDecode1(enc string, b []byte) (r rune, n, st, again int) {
	switch enc {
	case "UTF-8":
		return rbDecodeUTF8(b)
	case "ISO-8859-1":
		return rune(b[0]), 1, rbDecOK, 0
	case "US-ASCII", "ASCII-8BIT":
		if b[0] < 0x80 {
			return rune(b[0]), 1, rbDecOK, 0
		}
		if enc == "US-ASCII" {
			return 0, 1, rbDecInvalid, 0
		}
		return 0, 1, rbDecUndef, 0
	case "UTF-16LE", "UTF-16BE", "UTF-32LE", "UTF-32BE":
		return rbDecodeUnits(enc, b)
	}
	panic("rb2go: no decoder for " + enc)
}

// rbDecodeUnits follows MRI's byte tries: the first byte that cannot continue the character makes it invalid, and a fault inside the first unit still takes the whole unit (or what is left) as the bad bytes.
func rbDecodeUnits(enc string, b []byte) (r rune, n, st, again int) {
	unit := 2
	if strings.HasPrefix(enc, "UTF-32") {
		unit = 4
	}
	size := unit
	surr := func(c byte) bool { return c >= 0xD8 && c <= 0xDF }
	for k := 0; k < size; k++ {
		if k >= len(b) {
			return 0, len(b), rbDecIncomplete, 0
		}
		c, ok := b[k], true
		switch enc {
		case "UTF-16BE":
			ok = k != 0 && k != 2 || k == 0 && (c < 0xDC || c > 0xDF) || k == 2 && c >= 0xDC && c <= 0xDF
			if k == 0 && c >= 0xD8 && c <= 0xDB {
				size = 4
			}
		case "UTF-16LE":
			ok = k != 1 && k != 3 || k == 1 && (c < 0xDC || c > 0xDF) || k == 3 && c >= 0xDC && c <= 0xDF
			if k == 1 && c >= 0xD8 && c <= 0xDB {
				size = 4
			}
		case "UTF-32BE":
			ok = k == 0 && c == 0 || k == 1 && c <= 0x10 || k == 2 && (b[1] != 0 || !surr(c)) || k == 3
		case "UTF-32LE":
			ok = k < 2 || k == 2 && c <= 0x10 && (c != 0 || !surr(b[1])) || k == 3 && c == 0
		}
		if !ok {
			if k+1 <= unit {
				return 0, min(unit, len(b)), rbDecInvalid, 0
			}
			discard := k / unit * unit
			return 0, discard, rbDecInvalid, k + 1 - discard
		}
	}
	if unit == 2 {
		u := rbUnit16(enc, b)
		if size == 4 {
			return utf16.DecodeRune(rune(u), rune(rbUnit16(enc, b[2:]))), 4, rbDecOK, 0
		}
		return rune(u), 2, rbDecOK, 0
	}
	if enc == "UTF-32LE" {
		return rune(binary.LittleEndian.Uint32(b)), 4, rbDecOK, 0
	}
	return rune(binary.BigEndian.Uint32(b)), 4, rbDecOK, 0
}

func rbUnit16(enc string, b []byte) uint16 {
	if enc == "UTF-16LE" {
		return binary.LittleEndian.Uint16(b)
	}
	return binary.BigEndian.Uint16(b)
}

// rbDecodeUTF8 reports each maximal invalid subpart once, as MRI's transcoder and scrub do, where Go's decoder goes a byte at a time.
func rbDecodeUTF8(b []byte) (r rune, n, st, again int) {
	c := b[0]
	if c < 0x80 {
		return rune(c), 1, rbDecOK, 0
	}
	var size int
	lo, hi := byte(0x80), byte(0xBF)
	switch {
	case c >= 0xC2 && c <= 0xDF:
		size = 2
	case c == 0xE0:
		size, lo = 3, 0xA0
	case c == 0xED:
		size, hi = 3, 0x9F
	case c >= 0xE1 && c <= 0xEF:
		size = 3
	case c == 0xF0:
		size, lo = 4, 0x90
	case c == 0xF4:
		size, hi = 4, 0x8F
	case c >= 0xF1 && c <= 0xF3:
		size = 4
	default:
		return 0, 1, rbDecInvalid, 0
	}
	for i := 1; i < size; i++ {
		if i >= len(b) {
			return 0, i, rbDecIncomplete, 0
		}
		if b[i] < lo || b[i] > hi {
			return 0, i, rbDecInvalid, 1
		}
		lo, hi = 0x80, 0xBF
	}
	r, _ = utf8.DecodeRune(b[:size])
	return r, size, rbDecOK, 0
}

// rbEncode1 appends r in enc, or reports that enc has no such character.
func rbEncode1(out []byte, enc string, r rune) ([]byte, bool) {
	switch enc {
	case "UTF-8":
		return utf8.AppendRune(out, r), true
	case "US-ASCII", "ASCII-8BIT":
		return append(out, byte(r)), r < 0x80
	case "ISO-8859-1":
		return append(out, byte(r)), r < 0x100
	case "UTF-16LE", "UTF-16BE", "UTF-16":
		var units []uint16
		units = utf16.AppendRune(units, r)
		for _, u := range units {
			if enc == "UTF-16LE" {
				out = binary.LittleEndian.AppendUint16(out, u)
			} else {
				out = binary.BigEndian.AppendUint16(out, u)
			}
		}
		return out, true
	case "UTF-32LE":
		return binary.LittleEndian.AppendUint32(out, uint32(r)), true
	case "UTF-32BE", "UTF-32":
		return binary.BigEndian.AppendUint32(out, uint32(r)), true
	}
	panic("rb2go: no encoder for " + enc)
}

// rbBinDump is String#dump of bytes tagged ASCII-8BIT, as MRI's encoding errors quote them.
func rbBinDump(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := range len(s) {
		c := s[i]
		switch {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '#' && i+1 < len(s) && strings.IndexByte("{$@", s[i+1]) >= 0:
			b.WriteString("\\#")
		case c >= 0x20 && c < 0x7f:
			b.WriteByte(c)
		default:
			if e, ok := map[byte]string{'\n': "n", '\t': "t", '\r': "r", '\f': "f", '\v': "v", '\b': "b", '\a': "a", 0x1b: "e"}[c]; ok {
				b.WriteString("\\" + e)
			} else {
				fmt.Fprintf(&b, "\\x%02X", c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// rbEncOpts are encode's keyword options: invalid:/undef: :replace, replace:, xml: and the newline decorators.
type rbEncOpts struct {
	invalid, undef bool
	replace        *string
	xml            string // "", "text" or "attr"
	newline        string // "", "universal", "crlf" or "cr"
}

func (o rbEncOpts) decorated() bool { return o.xml != "" || o.newline != "" }

// rbEncPath is MRI's conversion path: through UTF-8 unless either end is UTF-8.
func rbEncPath(from, to string) []string {
	if from == "UTF-8" || to == "UTF-8" {
		return []string{from, to}
	}
	return []string{from, "UTF-8", to}
}

// rbEncBOM: a dummy UTF-16/UTF-32 source must start with a BOM, which picks the byte order.
func rbEncBOM(enc string, s []byte) (rest []byte, order string, ok bool) {
	if enc == "UTF-16" {
		switch {
		case bytes.HasPrefix(s, []byte{0xFE, 0xFF}):
			return s[2:], "UTF-16BE", true
		case bytes.HasPrefix(s, []byte{0xFF, 0xFE}):
			return s[2:], "UTF-16LE", true
		}
		return s, "", false
	}
	switch {
	case bytes.HasPrefix(s, []byte{0, 0, 0xFE, 0xFF}):
		return s[4:], "UTF-32BE", true
	case bytes.HasPrefix(s, []byte{0xFF, 0xFE, 0, 0}):
		return s[4:], "UTF-32LE", true
	}
	return s, "", false
}

// rbTranscode raises MRI's Encoding errors with MRI's messages.
func rbTranscode(s, from, to string, o rbEncOpts) string {
	if from == to && (from == "ASCII-8BIT" || !o.invalid && !o.decorated()) {
		return s
	}
	if from == to && rbEncASCIICompat(from) && !o.invalid {
		return rbEncDecorate(s, o) // no transcoder: the decorators alone, on the bytes as they are
	}
	path := rbEncPath(from, to)
	src := []byte(s)
	dec, enc := from, to
	if to == "UTF-16" || to == "UTF-32" {
		enc = to + "BE"
	}
	repl := string(utf8.RuneError)
	if !rbEncUnicode(to) {
		repl = "?"
	}
	if o.replace != nil {
		repl = *o.replace
	}
	var replBytes []byte
	for _, r := range repl {
		replBytes, _ = rbEncode1(replBytes, enc, r)
	}
	out := make([]byte, 0, len(src))
	if o.xml == "attr" {
		out, _ = rbEncode1(out, enc, '"')
	}
	cr := false
	for i := 0; i < len(src); {
		var r rune
		var n, st, again int
		if dec == "UTF-16" || dec == "UTF-32" { // a dummy source wants a BOM first, unit by unit, as MRI's decoder does
			w := map[string]int{"UTF-16": 2, "UTF-32": 4}[dec]
			if _, order, ok := rbEncBOM(dec, src[i:]); ok {
				dec = order
				i += w
				continue
			}
			n, st = min(w, len(src)-i), rbDecInvalid
			if n < w {
				st = rbDecIncomplete
			}
		} else {
			r, n, st, again = rbDecode1(dec, src[i:])
		}
		switch st {
		case rbDecInvalid, rbDecIncomplete:
			if !o.invalid {
				var ra []byte
				if again > 0 && i+n+again <= len(src) {
					ra = src[i+n : i+n+again]
				}
				rbEncInvalid(from, path[1], src[i:i+n], ra, st == rbDecIncomplete)
			}
			out = append(out, replBytes...)
			i += n
			cr = false
			continue
		case rbDecUndef:
			if !o.undef {
				rbEncUndef(path, from, "UTF-8", rbBinDump(string(src[i:i+n])), string(src[i:i+n]))
			}
			out = append(out, replBytes...)
			i += n
			cr = false
			continue
		}
		i += n
		if o.newline == "universal" {
			if r == '\n' && cr {
				cr = false
				continue
			}
			cr = r == '\r'
			if cr {
				r = '\n'
			}
		}
		out = rbEncodeDecorated(out, enc, r, o, path, replBytes)
	}
	if o.xml == "attr" {
		out, _ = rbEncode1(out, enc, '"')
	}
	if enc != to && len(out) > 0 {
		bom, _ := rbEncode1(nil, enc, 0xFEFF)
		out = append(bom, out...)
	}
	return string(out)
}

// rbEncodeDecorated appends one decoded character to out in enc after the xml and newline decorators.
func rbEncodeDecorated(out []byte, enc string, r rune, o rbEncOpts, path []string, replBytes []byte) []byte {
	var esc string
	switch {
	case o.xml != "" && r == '&':
		esc = "&amp;"
	case o.xml != "" && r == '<':
		esc = "&lt;"
	case o.xml != "" && r == '>':
		esc = "&gt;"
	case o.xml == "attr" && r == '"':
		esc = "&quot;"
	case o.xml == "attr" && r == '\'':
		esc = "&apos;"
	case r == '\n' && o.newline == "crlf":
		esc = "\r\n"
	case r == '\n' && o.newline == "cr":
		esc = "\r"
	}
	if esc != "" {
		for _, e := range esc {
			out, _ = rbEncode1(out, enc, e)
		}
		return out
	}
	next, ok := rbEncode1(out, enc, r)
	if ok {
		return next
	}
	switch {
	case o.xml != "":
		for _, e := range fmt.Sprintf("&#x%X;", r) {
			out, _ = rbEncode1(out, enc, e)
		}
		return out
	case o.undef:
		return append(out, replBytes...)
	}
	rbEncUndef(path, "UTF-8", path[len(path)-1], fmt.Sprintf("U+%04X", r), string(r))
	return nil
}

// rbEncDecorate applies the xml and newline decorators to bytes of an ASCII-compatible encoding without transcoding them.
func rbEncDecorate(s string, o rbEncOpts) string {
	var b strings.Builder
	if o.xml == "attr" {
		b.WriteByte('"')
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case o.newline == "universal" && c == '\r':
			b.WriteByte('\n')
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
		case o.xml != "" && c == '&':
			b.WriteString("&amp;")
		case o.xml != "" && c == '<':
			b.WriteString("&lt;")
		case o.xml != "" && c == '>':
			b.WriteString("&gt;")
		case o.xml == "attr" && c == '"':
			b.WriteString("&quot;")
		case o.xml == "attr" && c == '\'':
			b.WriteString("&apos;")
		case o.newline == "crlf" && c == '\n':
			b.WriteString("\r\n")
		case o.newline == "cr" && c == '\n':
			b.WriteByte('\r')
		default:
			b.WriteByte(c)
		}
	}
	if o.xml == "attr" {
		b.WriteByte('"')
	}
	return b.String()
}

// rbEncInvalid raises Encoding::InvalidByteSequenceError as MRI words it.
func rbEncInvalid(src, dst string, bad, again []byte, incomplete bool) {
	msg := rbBinDump(string(bad)) + " on " + src
	switch {
	case incomplete:
		msg = "incomplete " + msg
	case again != nil:
		msg = rbBinDump(string(bad)) + " followed by " + rbBinDump(string(again)) + " on " + src
	}
	var ra *String
	if again != nil {
		ra = Ref(String(again))
	}
	panic(NewEncoding_InvalidByteSequenceError(Ref(String(msg)), Ref(String(src)), Ref(String(dst)), Ref(String(bad)), ra, Ref(Boolean(incomplete))))
}

// rbEncUndef raises Encoding::UndefinedConversionError: "X from A to B" when the step that failed is the whole conversion, else "X to B in conversion from A to UTF-8 to C".
func rbEncUndef(path []string, src, dst, shown, char string) {
	msg := shown + " from " + src + " to " + dst
	if len(path) > 2 {
		msg = shown + " to " + dst + " in conversion from " + strings.Join(path, " to ")
	}
	panic(NewEncoding_UndefinedConversionError(Ref(String(msg)), Ref(String(src)), Ref(String(dst)), Ref(String(char))))
}

// rbEncNotFound raises Encoding::ConverterNotFoundError for names rb2go has no converter for.
func rbEncNotFound(from, to string) {
	panic(NewEncoding_ConverterNotFoundError(Ref(String("code converter not found (" + from + " to " + to + ")"))))
}

// rbStrEncode: a missing from is UTF-8, what every rb2go String holds; a missing to is default_internal (dflt), else only the options apply (decision 136).
func rbStrEncode(s String, to, from any, dflt string, o rbEncOpts) String {
	toName, fromName := dflt, "UTF-8"
	if to != nil {
		toName = rbEncNameArg(to)
	}
	if from != nil {
		fromName = rbEncNameArg(from)
	}
	if toName == "" {
		if !o.invalid && !o.decorated() {
			return rbStrClone(s)
		}
		toName = fromName
	}
	t, f := rbEncCanon(toName), rbEncCanon(fromName)
	if t == "" || f == "" {
		rbEncNotFound(fromName, toName)
	}
	return String(rbTranscode(string(s), f, t, o))
}

// rbEncOptions checks encode's keyword values as MRI does.
func rbEncOptions(invalid, undef *Symbol, replace *String, xml *Symbol, universal, crlf, cr bool) rbEncOpts {
	var o rbEncOpts
	if invalid != nil {
		if *invalid != "replace" {
			panic(NewArgumentError(Ref(String("unknown value for invalid character option"))))
		}
		o.invalid = true
	}
	if undef != nil {
		if *undef != "replace" {
			panic(NewArgumentError(Ref(String("unknown value for undefined character option"))))
		}
		o.undef = true
	}
	if replace != nil {
		r := string(*replace)
		o.replace = &r
	}
	if xml != nil {
		if *xml != "text" && *xml != "attr" {
			panic(NewArgumentError(Ref(String("unexpected value for xml option: " + string(*xml)))))
		}
		o.xml = string(*xml)
	}
	switch {
	case universal:
		o.newline = "universal"
	case crlf:
		o.newline = "crlf"
	case cr:
		o.newline = "cr"
	}
	return o
}

// rbScrubParts: valid runs at even indexes, invalid maximal subparts at odd ones, so scrub's block sees each.
func rbScrubParts(s string) []string {
	var parts []string
	start := 0
	b := []byte(s)
	for i := 0; i < len(b); {
		_, n, st, _ := rbDecodeUTF8(b[i:])
		if st != rbDecOK {
			parts = append(parts, s[start:i], s[i:i+n])
			start = i + n
		}
		i += n
	}
	return append(parts, s[start:])
}

// rbScrub is String#scrub with a replacement string.
func rbScrub(s, repl string) string {
	if utf8.ValidString(s) {
		return s
	}
	parts := rbScrubParts(s)
	var b strings.Builder
	for i, p := range parts {
		if i%2 == 1 {
			b.WriteString(repl)
		} else {
			b.WriteString(p)
		}
	}
	return b.String()
}

// rbIntChr is Integer#chr(encoding).
func rbIntChr(n int, enc any) String {
	name := rbEncNameArg(enc)
	e := rbEncCanon(name)
	if e == "" {
		panic(NewArgumentError(Ref(String("unknown encoding name - " + name))))
	}
	single := e == "ASCII-8BIT" || e == "ISO-8859-1" || e == "US-ASCII"
	switch {
	case n < 0 || single && n > 0xFF || e == "UTF-8" && n > unicode.MaxRune:
		panic(NewRangeError(Ref(String(strconv.Itoa(n) + " out of char range"))))
	case e == "US-ASCII" && n > 0x7F, !single && (n > unicode.MaxRune || n >= 0xD800 && n < 0xE000):
		panic(NewRangeError(Ref(String(fmt.Sprintf("invalid codepoint 0x%X in %s", n, e)))))
	case single:
		return String([]byte{byte(n)})
	}
	out, _ := rbEncode1(nil, e, rune(n))
	return String(out)
}

// rbTranscodeReader converts as it reads, so bad input raises at the read that meets it, as in MRI.
type rbTranscodeReader struct {
	src      io.Reader
	from, to string
	pend     []byte
	out      []byte
	eof      bool
}

func (t *rbTranscodeReader) Read(p []byte) (int, error) {
	for len(t.out) == 0 {
		if t.eof && len(t.pend) == 0 {
			return 0, io.EOF
		}
		if !t.eof {
			buf := make([]byte, 4096)
			n, err := t.src.Read(buf)
			t.pend = append(t.pend, buf[:n]...)
			if err != nil {
				t.eof = true
			}
		}
		if t.from == "UTF-16" || t.from == "UTF-32" {
			if len(t.pend) < 4 && !t.eof {
				continue
			}
			if rest, order, ok := rbEncBOM(t.from, t.pend); ok {
				t.pend, t.from = rest, order
			} else {
				rbTranscode(string(t.pend), t.from, t.to, rbEncOpts{}) // raises MRI's error for the missing BOM
			}
		}
		k := 0
		for k < len(t.pend) {
			_, n, st, _ := rbDecode1(t.from, t.pend[k:])
			if st == rbDecIncomplete && !t.eof {
				break
			}
			k += n
		}
		t.out = []byte(rbTranscode(string(t.pend[:k]), t.from, t.to, rbEncOpts{}))
		t.pend = t.pend[k:]
	}
	n := copy(p, t.out)
	t.out = t.out[n:]
	return n, nil
}

// rbIOEnc: "" is the default encoding; bomOut keeps a dummy UTF-16/UTF-32 file to one BOM.
type rbIOEnc struct {
	ext, intern string
	bin, bomOut bool
	raw         *bufio.Reader
}

// rbFileEncHook stays nil until the compiler sees a File.open, File.new or CSV.open whose mode may name encodings (rbFileEncModes), so other programs carry no transcoder (decision 136).
var rbFileEncHook func(access, spec string, bin bool) func(*File)

func rbFileEncModes() { rbFileEncHook = rbFileModeEnc }

// rbFileModeEnc checks a mode's ":ext[:int]" part before the file is opened, as MRI does, and returns what to set up once it is.
func rbFileModeEnc(access, spec string, bin bool) func(*File) {
	var e rbIOEnc
	if bin {
		e.bin, e.ext = true, "ASCII-8BIT"
	}
	parts := strings.SplitN(spec, ":", 2)
	name, bom := parts[0], false
	if len(name) > 4 && strings.EqualFold(name[:4], "BOM|") {
		name, bom = name[4:], true
	}
	if c := rbEncCanon(name); c != "" {
		e.ext = c
	} else {
		rbWarnLoc("Unsupported encoding " + name + " ignored")
	}
	if len(parts) > 1 {
		if c := rbEncCanon(parts[1]); c != "" {
			e.intern = c
		} else {
			rbWarnLoc("Unsupported encoding " + parts[1] + " ignored")
		}
	}
	if e.intern == e.ext {
		e.intern = ""
	}
	if (access == "r" || strings.HasSuffix(access, "+")) && !e.bin && e.intern == "" && !rbEncASCIICompat(e.ext) {
		panic(NewArgumentError(Ref(String("ASCII incompatible encoding needs binmode"))))
	}
	return func(f *File) {
		f.enc = e
		if f.r != nil {
			if bom {
				f.enc.ext = rbSkipBOM(f.r, f.enc.ext)
			}
			f.r = f.enc.readerFor(f.r)
		}
		f.conv = f.enc.writeConv
	}
}

// rbWarnLoc prints `file:line: warning: msg` to $stderr for the nearest user-code frame.
func rbWarnLoc(msg string) {
	rbFlushIfTTY()
	if loc := rbCallerLoc(); loc != "" {
		msg = loc + ": warning: " + msg
	} else {
		msg = "warning: " + msg
	}
	_, _ = os.Stderr.WriteString(msg + "\n")
}

// rbSkipBOM is `r:BOM|UTF-8`: the mark, when there is one, overrides ext.
func rbSkipBOM(r *bufio.Reader, ext string) string {
	head, _ := r.Peek(4)
	for _, b := range []struct {
		mark []byte
		enc  string
	}{
		{[]byte{0, 0, 0xFE, 0xFF}, "UTF-32BE"}, {[]byte{0xFF, 0xFE, 0, 0}, "UTF-32LE"},
		{[]byte{0xEF, 0xBB, 0xBF}, "UTF-8"}, {[]byte{0xFE, 0xFF}, "UTF-16BE"}, {[]byte{0xFF, 0xFE}, "UTF-16LE"},
	} {
		if bytes.HasPrefix(head, b.mark) {
			_, _ = r.Discard(len(b.mark))
			return b.enc
		}
	}
	return ext
}

// readerFor wraps the file's own reader, never an earlier transcoder, so a second set_encoding does not convert twice.
func (e *rbIOEnc) readerFor(r *bufio.Reader) *bufio.Reader {
	if e.raw == nil {
		e.raw = r
	}
	if e.intern == "" || e.ext == "" || e.ext == e.intern {
		return e.raw
	}
	return bufio.NewReader(&rbTranscodeReader{src: e.raw, from: e.ext, to: e.intern})
}

// writeConv: MRI converts on write whenever the external encoding differs from the String's.
func (e *rbIOEnc) writeConv(s string) string {
	if e.ext == "" || e.ext == "UTF-8" || e.ext == "ASCII-8BIT" {
		return s
	}
	to := e.ext
	if to == "UTF-16" || to == "UTF-32" {
		to += "BE"
		if !e.bomOut && s != "" {
			e.bomOut = true
			return rbTranscode(s, "UTF-8", e.ext, rbEncOpts{})
		}
	}
	return rbTranscode(s, "UTF-8", to, rbEncOpts{})
}

// rbSetEncoding: ext is an Encoding, a name, "ext:int", or nil for the defaults.
func rbSetEncoding(ext, intern any) rbIOEnc {
	var e rbIOEnc
	if ext == nil {
		return e
	}
	name := rbEncNameArg(ext)
	if intern == nil {
		if a, b, ok := strings.Cut(name, ":"); ok {
			name, intern = a, String(b)
		}
	}
	if c := rbEncCanon(strings.TrimPrefix(strings.TrimPrefix(name, "BOM|"), "bom|")); c != "" {
		e.ext = c
	} else {
		rbWarnLoc("Unsupported encoding " + name + " ignored")
	}
	if intern != nil {
		in := rbEncNameArg(intern)
		if c := rbEncCanon(in); c != "" {
			e.intern = c
		} else {
			rbWarnLoc("Unsupported encoding " + in + " ignored")
		}
	}
	if e.intern == e.ext {
		e.intern = ""
	}
	return e
}
