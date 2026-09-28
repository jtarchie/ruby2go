//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

type I_ToJson interface{ ToJson(...any) String }

// rbToJson is `a.to_json(*args)` on any value. The generator passes its
// state as the one argument, as the gem does, so a user to_json with a
// signature other than (*untyped) is reached through its Dyn wrapper.
func rbToJson(a any, args ...any) String {
	a = rbUnbox(a)
	if a == nil {
		return "null"
	}
	if j, ok := a.(I_ToJson); ok {
		return j.ToJson(args...)
	}
	if j, ok := a.(interface{ DynToJson(...any) any }); ok {
		r := j.DynToJson(args...)
		if s, ok := r.(String); ok {
			return s
		}
		panic(NewTypeError(Ref(String("wrong argument type " + rbClassName(r) + " (expected String)"))))
	}
	return rbToS(a).ToJson(args...)
}

// rbJSONSymbolizeNames reads JSON.parse/.load's symbolize_names: option.
func rbJSONSymbolizeNames(opts *Hash[Symbol, any]) bool {
	if opts == nil {
		return false
	}
	v, ok := opts.vals[Symbol("symbolize_names")]
	return ok && rbTruthy(v)
}

// rbJSONParse walks the token stream itself, not json.Unmarshal into a map, so object keys keep insertion order (rb2go's Hash).
func rbJSONParse(s string, symbolizeNames bool) any {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()

	var (
		rv  rbJSONVal
		err error
	)
	if symbolizeNames {
		rv, err = rbJSONDecodeValue(dec, func(k string) Symbol { return Symbol(k) })
	} else {
		rv, err = rbJSONDecodeValue(dec, func(k string) String { return String(k) })
	}
	if err == nil {
		if dec.More() {
			tok, terr := dec.Token()
			if terr != nil {
				err = terr
			} else {
				err = fmt.Errorf("unexpected token at end of stream %v", tok)
			}
		} else if _, terr := dec.Token(); terr != nil && !errors.Is(terr, io.EOF) {
			err = terr
		}
	}
	if err != nil {
		panic(NewJSON_ParserError(Ref(String(rbJSONDecodeErr(err)))))
	}
	return rv.v
}

// rbJSONVal wraps a decoded value: a JSON null is a real `any(nil)` v, not a decode failure, so rbJSONDecodeValue returns this struct rather than (any, error), which would read as nilnil's ambiguous "nil value, nil error".
type rbJSONVal struct{ v any }

// rbJSONDecodeValue decodes one JSON value; keyOf (String or Symbol per symbolize_names) fixes every nested Hash to the same key type.
func rbJSONDecodeValue[K comparable](dec *json.Decoder, keyOf func(string) K) (rbJSONVal, error) {
	tok, err := dec.Token()
	if err != nil {
		return rbJSONVal{}, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			h := NewHash[K, any]()
			for dec.More() {
				kt, kerr := dec.Token()
				if kerr != nil {
					return rbJSONVal{}, kerr
				}
				ks, ok := kt.(string)
				if !ok {
					return rbJSONVal{}, fmt.Errorf("expected object key, got %v", kt)
				}
				v, verr := rbJSONDecodeValue(dec, keyOf)
				if verr != nil {
					return rbJSONVal{}, verr
				}
				h.Op_idxSet(keyOf(ks), v.v)
			}
			if _, cerr := dec.Token(); cerr != nil { // consume '}'
				return rbJSONVal{}, cerr
			}
			return rbJSONVal{h}, nil
		case '[':
			arr := &Array[any]{}
			for dec.More() {
				v, verr := rbJSONDecodeValue(dec, keyOf)
				if verr != nil {
					return rbJSONVal{}, verr
				}
				*arr = append(*arr, v.v)
			}
			if _, cerr := dec.Token(); cerr != nil { // consume ']'
				return rbJSONVal{}, cerr
			}
			return rbJSONVal{arr}, nil
		}
	case string:
		return rbJSONVal{String(t)}, nil
	case json.Number:
		return rbJSONVal{rbJSONParseNumber(string(t))}, nil
	case bool:
		return rbJSONVal{Boolean(t)}, nil
	case nil:
		return rbJSONVal{}, nil // v's zero value is untyped nil: JSON null
	}
	return rbJSONVal{}, fmt.Errorf("unexpected token %v", tok)
}

// rbJSONParseNumber: a decimal point or exponent is a Float (MRI's json gem); past 64 bits raises RangeError, as String#to_i (decision 35: no Bignum).
func rbJSONParseNumber(s string) any {
	if strings.ContainsAny(s, ".eE") {
		f, _ := strconv.ParseFloat(s, 64) // ErrRange (too large) saturates to +-Inf, as MRI
		return Float(f)
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		panic(NewRangeError(Ref(String(s + " overflows Integer (64-bit; no Bignum)"))))
	}
	return Integer(n)
}

// rbJSONDecodeErr turns a decode error into JSON::ParserError's message.
func rbJSONDecodeErr(err error) string {
	if errors.Is(err, io.EOF) {
		return "unexpected end of input"
	}
	return err.Error()
}

// rbJSONState is the gem's generator State. It is handed to every
// nested to_json, which is how depth reaches the indentation.
type rbJSONState struct {
	indent, space, spaceBefore, objectNl, arrayNl string
	scriptSafe, asciiOnly, allowNaN               bool
	depth                                         int
}

// rbJSONStateOf is State.from_state on a to_json's arguments: the state
// passed down, a new one configured by an options Hash, or a default.
func rbJSONStateOf(args []any) *rbJSONState {
	st := &rbJSONState{}
	if len(args) == 0 {
		return st
	}
	switch o := rbUnbox(args[0]).(type) {
	case *rbJSONState:
		return o
	case Hash_Any:
		opts := o._ToAny()
		for _, k := range opts.keys {
			v := opts.vals[k]
			switch k {
			case Symbol("indent"):
				st.indent = rbJSONOpt(v)
			case Symbol("space"):
				st.space = rbJSONOpt(v)
			case Symbol("space_before"):
				st.spaceBefore = rbJSONOpt(v)
			case Symbol("object_nl"):
				st.objectNl = rbJSONOpt(v)
			case Symbol("array_nl"):
				st.arrayNl = rbJSONOpt(v)
			case Symbol("script_safe"), Symbol("escape_slash"):
				st.scriptSafe = rbTruthy(v)
			case Symbol("ascii_only"):
				st.asciiOnly = rbTruthy(v)
			case Symbol("allow_nan"):
				st.allowNaN = rbTruthy(v)
			case Symbol("depth"):
				if d, ok := v.(Integer); ok {
					st.depth = int(d)
				}
			case Symbol("sort_keys"), Symbol("strict"), Symbol("as_json"):
				if rbTruthy(v) {
					panic(NewNotImplementedError(Ref(String("JSON generator option " + string(k.(Symbol)) + " is not supported"))))
				}
			}
		}
	}
	return st
}

// rbJSONOpt reads a String option; nil means "".
func rbJSONOpt(v any) string {
	switch s := v.(type) {
	case nil:
		return ""
	case String:
		return string(s)
	}
	panic(NewTypeError(Ref(String("wrong argument type " + rbClassName(v) + " (expected String)"))))
}

// rbJSONArray and rbJSONHash port the gem's generate_json_array and
// generate_json_object: indent per depth before each element, the
// newline string after the opener and each ",", and before the closer
// (then indented) only when set.
func rbJSONArray[E any](xs []E, args []any) String {
	st := rbJSONStateOf(args)
	if len(xs) == 0 {
		return "[]"
	}
	st.depth++
	depth := st.depth
	var b strings.Builder
	b.WriteByte('[')
	b.WriteString(st.arrayNl)
	for i, x := range xs {
		if i > 0 {
			b.WriteByte(',')
			b.WriteString(st.arrayNl)
		}
		b.WriteString(strings.Repeat(st.indent, depth))
		b.WriteString(string(rbToJson(x, st)))
		st.depth = depth
	}
	st.depth--
	if st.arrayNl != "" {
		b.WriteString(st.arrayNl)
		b.WriteString(strings.Repeat(st.indent, st.depth))
	}
	b.WriteByte(']')
	return String(b.String())
}

func rbJSONHash[K, V comparable](h *Hash[K, V], args []any) String {
	st := rbJSONStateOf(args)
	if len(h.keys) == 0 {
		return "{}"
	}
	st.depth++
	depth := st.depth
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range h.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(st.objectNl)
		b.WriteString(strings.Repeat(st.indent, depth))
		b.WriteString(string(rbJSONString(string(rbToS(k)), st)))
		b.WriteString(st.spaceBefore)
		b.WriteByte(':')
		b.WriteString(st.space)
		b.WriteString(string(rbToJson(h.vals[k], st)))
		st.depth = depth
	}
	st.depth--
	if st.objectNl != "" {
		b.WriteString(st.objectNl)
		b.WriteString(strings.Repeat(st.indent, st.depth))
	}
	b.WriteByte('}')
	return String(b.String())
}

// rbJSONString escapes like the json gem's generator: quotes,
// backslashes and control characters; "/" and non-ASCII stay as-is
// unless st asks for script_safe (also U+2028/9) or ascii_only.
// Invalid UTF-8 is the gem's GeneratorError.
func rbJSONString(s string, st *rbJSONState) String {
	if !utf8.ValidString(s) {
		panic(NewJSON_GeneratorError(Ref(String("source sequence is illegal/malformed utf-8"))))
	}
	var b strings.Builder
	b.WriteByte('"')
	for i, r := range s {
		if st.scriptSafe && (r == '/' || r == 0x2028 || r == 0x2029) || st.asciiOnly && r >= 0x80 {
			switch {
			case r == '/':
				b.WriteString(`\/`)
			case r > 0xffff:
				r1, r2 := utf16.EncodeRune(r)
				fmt.Fprintf(&b, `\u%04x\u%04x`, r1, r2)
			default:
				fmt.Fprintf(&b, `\u%04x`, r)
			}
			continue
		}
		c := s[i]
		if r >= 0x80 {
			b.WriteRune(r)
			continue
		}
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if c < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, c)
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
	return String(b.String())
}

// rbJSONFloat ports the json gem's fpconv_dtoa: Grisu2's digits (not
// always the shortest: 1e23 is 9.999999999999999e+22), then
// emit_digits' plain decimal for moderate exponents, else d.ddde[+-]N.
func rbJSONFloat(f float64, st *rbJSONState) String {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		if st.allowNaN {
			return rbFloatToS(f)
		}
		panic(NewJSON_GeneratorError(Ref(String(string(rbFloatToS(f)) + " not allowed in JSON"))))
	}
	sign := ""
	if math.Signbit(f) {
		sign = "-"
		f = -f
	}
	if f == 0 {
		return String(sign + "0.0")
	}
	digits, k := rbGrisu2(f)
	nd := len(digits)
	exp := k + nd - 1
	if exp < 0 {
		exp = -exp
	}
	switch {
	case k >= 0 && exp < 15:
		return String(sign + digits + strings.Repeat("0", k) + ".0")
	case k < 0 && (k > -7 || exp < 10):
		offset := nd + k
		if offset <= 0 {
			return String(sign + "0." + strings.Repeat("0", -offset) + digits)
		}
		return String(sign + digits[:offset] + "." + digits[offset:])
	}
	nd = min(nd, 18-len(sign))
	out := sign + digits[:1]
	if nd > 1 {
		out += "." + digits[1:nd]
	}
	esign := "+"
	if k+nd-1 < 0 {
		esign = "-"
	}
	return String(out + "e" + esign + strconv.Itoa(exp))
}

// rbFp is fpconv's Fp: frac * 2^exp.
type rbFp struct {
	frac uint64
	exp  int
}

// rbPowersTen is fpconv's cached powers of ten, 10^-348 to 10^340 in
// steps of 8.
var rbPowersTen = [...]rbFp{
	{18054884314459144840, -1220}, {13451937075301367670, -1193},
	{10022474136428063862, -1166}, {14934650266808366570, -1140},
	{11127181549972568877, -1113}, {16580792590934885855, -1087},
	{12353653155963782858, -1060}, {18408377700990114895, -1034},
	{13715310171984221708, -1007}, {10218702384817765436, -980},
	{15227053142812498563, -954}, {11345038669416679861, -927},
	{16905424996341287883, -901}, {12595523146049147757, -874},
	{9384396036005875287, -847}, {13983839803942852151, -821},
	{10418772551374772303, -794}, {15525180923007089351, -768},
	{11567161174868858868, -741}, {17236413322193710309, -715},
	{12842128665889583758, -688}, {9568131466127621947, -661},
	{14257626930069360058, -635}, {10622759856335341974, -608},
	{15829145694278690180, -582}, {11793632577567316726, -555},
	{17573882009934360870, -529}, {13093562431584567480, -502},
	{9755464219737475723, -475}, {14536774485912137811, -449},
	{10830740992659433045, -422}, {16139061738043178685, -396},
	{12024538023802026127, -369}, {17917957937422433684, -343},
	{13349918974505688015, -316}, {9946464728195732843, -289},
	{14821387422376473014, -263}, {11042794154864902060, -236},
	{16455045573212060422, -210}, {12259964326927110867, -183},
	{18268770466636286478, -157}, {13611294676837538539, -130},
	{10141204801825835212, -103}, {15111572745182864684, -77},
	{11258999068426240000, -50}, {16777216000000000000, -24},
	{12500000000000000000, 3}, {9313225746154785156, 30},
	{13877787807814456755, 56}, {10339757656912845936, 83},
	{15407439555097886824, 109}, {11479437019748901445, 136},
	{17105694144590052135, 162}, {12744735289059618216, 189},
	{9495567745759798747, 216}, {14149498560666738074, 242},
	{10542197943230523224, 269}, {15709099088952724970, 295},
	{11704190886730495818, 322}, {17440603504673385349, 348},
	{12994262207056124023, 375}, {9681479787123295682, 402},
	{14426529090290212157, 428}, {10748601772107342003, 455},
	{16016664761464807395, 481}, {11933345169920330789, 508},
	{17782069995880619868, 534}, {13248674568444952270, 561},
	{9871031767461413346, 588}, {14708983551653345445, 614},
	{10959046745042015199, 641}, {16330252207878254650, 667},
	{12166986024289022870, 694}, {18130221999122236476, 720},
	{13508068024458167312, 747}, {10064294952495520794, 774},
	{14996968138956309548, 800}, {11173611982879273257, 827},
	{16649979327439178909, 853}, {12405201291620119593, 880},
	{9242595204427927429, 907}, {13772540099066387757, 933},
	{10261342003245940623, 960}, {15290591125556738113, 986},
	{11392378155556871081, 1013}, {16975966327722178521, 1039},
	{12648080533535911531, 1066},
}

// rbFpMul is fpconv's multiply: the high 64 bits of a*b, rounded.
func rbFpMul(a, b rbFp) rbFp {
	hi, lo := bits.Mul64(a.frac, b.frac)
	_, c := bits.Add64(lo, 1<<63, 0)
	return rbFp{hi + c, a.exp + b.exp + 64}
}

// rbGrisu2 is fpconv's grisu2 and generate_digits on a positive finite
// f: its digits d and exponent k, f ~= d * 10^k.
func rbGrisu2(f float64) (string, int) {
	b := math.Float64bits(f)
	w := rbFp{b & (1<<52 - 1), int(b >> 52)}
	if w.exp != 0 {
		w.frac += 1 << 52
		w.exp -= 1075
	} else {
		w.exp = -1074
	}
	upper := rbFp{w.frac<<1 + 1, w.exp - 1}
	for upper.frac&(1<<53) == 0 {
		upper.frac <<= 1
		upper.exp--
	}
	upper.frac <<= 10
	upper.exp -= 10
	ls := 1
	if w.frac == 1<<52 {
		ls = 2
	}
	lower := rbFp{(w.frac<<ls - 1) << (w.exp - ls - upper.exp), upper.exp}
	for w.frac&(1<<52) == 0 {
		w.frac <<= 1
		w.exp--
	}
	w.frac <<= 11
	w.exp -= 11

	idx := (int(float64(-(upper.exp+87))*0.30102999566398114) + 348) / 8
	for {
		if c := upper.exp + rbPowersTen[idx].exp + 64; c < -60 {
			idx++
		} else if c > -32 {
			idx--
		} else {
			break
		}
	}
	k := 348 - idx*8
	cp := rbPowersTen[idx]
	w, upper, lower = rbFpMul(w, cp), rbFpMul(upper, cp), rbFpMul(lower, cp)
	lower.frac++
	upper.frac--

	wfrac := upper.frac - w.frac
	delta := upper.frac - lower.frac
	sh := uint(-upper.exp)
	one := uint64(1) << sh
	part1, part2 := upper.frac>>sh, upper.frac&(one-1)
	var d []byte
	kappa := 10
	for div := uint64(1e9); kappa > 0; div /= 10 {
		digit := part1 / div
		if digit != 0 || len(d) > 0 {
			d = append(d, byte('0'+digit))
		}
		part1 -= digit * div
		kappa--
		if rem := part1<<sh + part2; rem <= delta {
			rbRoundDigit(d, delta, rem, div<<sh, wfrac)
			return string(d), k + kappa
		}
	}
	for unit := uint64(10); ; unit *= 10 {
		part2 *= 10
		delta *= 10
		kappa--
		if digit := part2 >> sh; digit != 0 || len(d) > 0 {
			d = append(d, byte('0'+digit))
		}
		part2 &= one - 1
		if part2 < delta {
			rbRoundDigit(d, delta, part2, one, wfrac*unit)
			return string(d), k + kappa
		}
	}
}

// rbRoundDigit is fpconv's round_digit: step the last digit down while
// that moves closer to w and stays inside the boundaries.
func rbRoundDigit(d []byte, delta, rem, kappa, frac uint64) {
	for rem < frac && delta-rem >= kappa && (rem+kappa < frac || frac-rem > rem+kappa-frac) {
		d[len(d)-1]--
		rem += kappa
	}
}
