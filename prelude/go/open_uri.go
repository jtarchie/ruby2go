//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbOpenURIMeta is what OpenURI::Meta.init puts on an extended StringIO; base is the final URI's string, set only on success as MRI's open_loop does.
type rbOpenURIMeta struct {
	status *Array[String]
	base   *String
	metas  *Hash[String, *Array[String]]
}

// rbOUM is s's open-uri metadata, or MRI's NoMethodError for a StringIO open-uri did not extend.
func (s *StringIO) rbOUM(name string) *rbOpenURIMeta {
	if s.oum == nil {
		panic(NewNoMethodError(Ref(String("undefined method '" + name + "' for an instance of StringIO"))))
	}
	return s.oum
}

// rbOpenURIField is one field's values joined with ", ", as Meta#meta holds them.
func rbOpenURIField(metas *Hash[String, *Array[String]], name String) (string, bool) {
	vs, ok := metas.vals[name]
	if !ok {
		return "", false
	}
	parts := make([]string, 0, len(*vs))
	for _, v := range *vs {
		parts = append(parts, string(v))
	}
	return strings.Join(parts, ", "), true
}

// rbOpenURIJoined is OpenURI::Meta#meta.
func rbOpenURIJoined(metas *Hash[String, *Array[String]]) *Hash[String, String] {
	out := NewHash[String, String]()
	for _, k := range metas.keys {
		v, _ := rbOpenURIField(metas, k)
		Hash_Op_idxSet(out, k, String(v))
	}
	return out
}

// rbOpenURIMediaType is Meta#content_type_parse's type and charset parameter; ok is false where MRI's regexp does not match.
func rbOpenURIMediaType(v string) (typ, charset string, hasCharset, ok bool) {
	// ponytail: mime rejects a repeated parameter MRI's regexp accepts (first wins there); a hand parser of RE_PARAMETERS fixes it.
	mt, params, err := mime.ParseMediaType(v)
	if err != nil || !strings.Contains(mt, "/") { // mime takes a bare "text" (Content-Disposition form); MRI needs type/subtype
		return "", "", false, false
	}
	charset, hasCharset = params["charset"]
	return mt, strings.ToLower(charset), hasCharset, true
}

// rbOpenURITokenByte is RE_TOKEN's byte class: no CTLs, space or separators.
func rbOpenURITokenByte(b byte) bool {
	return b > ' ' && b != 0x7f && !strings.ContainsRune("()<>@,;:\\\"/[]?={}", rune(b))
}

// rbOpenURICodings is Meta#content_encoding: when v starts with a token, every token in it, downcased.
func rbOpenURICodings(v string) *Array[String] {
	out := NewArray[String]()
	t := strings.TrimLeft(v, "\r\n\t ")
	if t == "" || !rbOpenURITokenByte(t[0]) {
		return out
	}
	start := -1
	for i := 0; i <= len(v); i++ {
		if i < len(v) && rbOpenURITokenByte(v[i]) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			*out = append(*out, String(strings.ToLower(v[start:i])))
			start = -1
		}
	}
	return out
}
