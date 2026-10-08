//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// rbFormEscape is URI.encode_www_form_component: url.QueryEscape, except that * stays and ~ is encoded.
func rbFormEscape(s string) string {
	return strings.NewReplacer("%2A", "*", "~", "%7E").Replace(url.QueryEscape(s))
}

// rbURIEscape is URI::RFC2396_Parser#escape: every byte outside RFC 2396's unreserved and reserved sets is %XX.
func rbURIEscape(s string) string {
	var b strings.Builder
	for i := range len(s) {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("-_.!~*'();/?:@&=+$,[]", c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// rbURIUnescape is URI::RFC2396_Parser#unescape: each %XX becomes its byte; anything else, `+` included, stays.
func rbURIUnescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				b.WriteByte(byte(v))
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// rbURIRaw: Has* tells an absent (nil in Ruby) component from an explicit empty one; Path has none since MRI's path is never nil.
type rbURIRaw struct {
	Scheme      string
	HasScheme   bool
	Userinfo    string
	HasUserinfo bool
	Host        string
	HasHost     bool
	Port        int
	HasPort     bool
	Path        string
	Query       string
	HasQuery    bool
	Fragment    string
	HasFragment bool
}

// rbURIDefaultPort is URI::HTTP/URI::HTTPS's default_port; only these two schemes are implemented.
func rbURIDefaultPort(scheme string) (int, bool) {
	switch scheme {
	case "http":
		return 80, true
	case "https":
		return 443, true
	default:
		return 0, false
	}
}

// rbURIDisallowed: raw bytes MRI's parser rejects unencoded but net/url is permissive about; ponytail, not full RFC validation.
func rbURIDisallowed(s string) bool {
	for i := range len(s) {
		c := s[i]
		if c <= 0x20 || c == 0x7f {
			return true
		}
		switch c {
		case '"', '<', '>', '\\', '^', '`', '{', '|', '}':
			return true
		}
	}
	return false
}

// rbURIParse is URI.parse's Go half; the returned error's message is already MRI's wording.
func rbURIParse(s string) (rbURIRaw, error) {
	var raw rbURIRaw
	if rbURIDisallowed(s) {
		return raw, fmt.Errorf("bad URI (is not URI?): %q", s)
	}
	u, err := url.Parse(s)
	if err != nil {
		return raw, fmt.Errorf("bad URI (is not URI?): %q", s)
	}
	if u.Scheme != "" {
		raw.Scheme = u.Scheme
		raw.HasScheme = true
	}
	if u.User != nil {
		raw.Userinfo = u.User.String()
		raw.HasUserinfo = true
	}
	// an empty authority ("file:///x") is host "" in MRI, not nil
	if u.Host != "" || u.Opaque == "" && strings.HasPrefix(strings.TrimPrefix(s[len(u.Scheme):], ":"), "//") {
		raw.HasHost = true
		raw.Host = u.Hostname()
		if p := u.Port(); p != "" {
			if n, err := strconv.Atoi(p); err == nil {
				raw.Port = n
				raw.HasPort = true
			}
		} else if n, ok := rbURIDefaultPort(u.Scheme); ok {
			raw.Port = n
			raw.HasPort = true
		}
	}
	raw.Path = u.EscapedPath()
	if raw.Path == "" && u.Opaque != "" {
		raw.Path = u.Opaque
	}
	if u.ForceQuery || u.RawQuery != "" {
		raw.Query = u.RawQuery
		raw.HasQuery = true
	}
	if strings.Contains(s, "#") {
		raw.Fragment = u.EscapedFragment()
		raw.HasFragment = true
	}
	return raw, nil
}

// rbURIMerge is URI#merge/URI.join's Go half, checked against MRI's URI.join on trailing-slash, "..", "/" and absolute-override cases.
func rbURIMerge(base string, rel string) (rbURIRaw, error) {
	b, err := url.Parse(base)
	if err != nil {
		return rbURIRaw{}, fmt.Errorf("bad URI (is not URI?): %q", base)
	}
	r, err := url.Parse(rel)
	if err != nil {
		return rbURIRaw{}, fmt.Errorf("bad URI (is not URI?): %q", rel)
	}
	return rbURIParse(b.ResolveReference(r).String())
}

// rbURIFromRaw picks URI::HTTP/URI::HTTPS/URI::Generic by scheme so `is_a?(URI::HTTP)` works like MRI's.
func rbURIFromRaw(raw rbURIRaw) URI_GenericI {
	var scheme, userinfo, host, query, fragment *String
	var port *Integer
	if raw.HasScheme {
		scheme = Ref(String(raw.Scheme))
	}
	if raw.HasUserinfo {
		userinfo = Ref(String(raw.Userinfo))
	}
	if raw.HasHost {
		host = Ref(String(raw.Host))
	}
	if raw.HasPort {
		port = Ref(Integer(raw.Port))
	}
	if raw.HasQuery {
		query = Ref(String(raw.Query))
	}
	if raw.HasFragment {
		fragment = Ref(String(raw.Fragment))
	}
	path := String(raw.Path)
	switch raw.Scheme {
	case "https":
		return NewURI_HTTPS(scheme, userinfo, host, port, path, query, fragment)
	case "http":
		return NewURI_HTTP(scheme, userinfo, host, port, path, query, fragment)
	default:
		return NewURI_Generic(scheme, userinfo, host, port, path, query, fragment)
	}
}

// rbOptStr and rbOptInt unwrap a `T?` ivar into a Go primitive plus a presence flag, for rbURIString's plain-Go signature.
func rbOptStr(p *String) (string, bool) {
	if p == nil {
		return "", false
	}
	return string(*p), true
}

func rbOptInt(p *Integer) (int, bool) {
	if p == nil {
		return 0, false
	}
	return int(*p), true
}

// rbURIString is URI::Generic#to_s; it drops an explicit default port, unlike net/url.URL.String.
func rbURIString(hasScheme bool, scheme string, hasUserinfo bool, userinfo string, hasHost bool, host string, hasPort bool, port int, path string, hasQuery bool, query string, hasFragment bool, fragment string) string {
	var b strings.Builder
	if hasScheme {
		b.WriteString(scheme)
		b.WriteByte(':')
	}
	if hasHost {
		b.WriteString("//")
		if hasUserinfo {
			b.WriteString(userinfo)
			b.WriteByte('@')
		}
		h := host
		if strings.Contains(h, ":") && !strings.HasPrefix(h, "[") {
			h = "[" + h + "]"
		}
		b.WriteString(h)
		if hasPort {
			if def, ok := rbURIDefaultPort(scheme); !ok || port != def {
				fmt.Fprintf(&b, ":%d", port)
			}
		}
	}
	b.WriteString(path)
	if hasQuery {
		b.WriteByte('?')
		b.WriteString(query)
	}
	if hasFragment {
		b.WriteByte('#')
		b.WriteString(fragment)
	}
	return b.String()
}

// rbURIAbsRefSrc is MRI's URI::RFC2396_Parser#make_regexp source (/x), whose (?!//) RE2 cannot match: built lenient.
const rbURIAbsRefSrc = `
        ([a-zA-Z][\-+.a-zA-Z\d]*):                           (?# 1: scheme)
        (?:
           ((?:[\-_.!~*'()a-zA-Z\d;?:@&=+$,]|%[a-fA-F\d]{2})(?:[\-_.!~*'()a-zA-Z\d;/?:@&=+$,\[\]]|%[a-fA-F\d]{2})*)                    (?# 2: opaque)
        |
           (?:(?:
             //(?:
                 (?:(?:((?:[\-_.!~*'()a-zA-Z\d;:&=+$,]|%[a-fA-F\d]{2})*)@)?        (?# 3: userinfo)
                   (?:((?:(?:[a-zA-Z0-9\-.]|%\h\h)+|\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}|\[(?:(?:[a-fA-F\d]{1,4}:)*(?:[a-fA-F\d]{1,4}|\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})|(?:(?:[a-fA-F\d]{1,4}:)*[a-fA-F\d]{1,4})?::(?:(?:[a-fA-F\d]{1,4}:)*(?:[a-fA-F\d]{1,4}|\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}))?)\]))(?::(\d*))?))? (?# 4: host, 5: port)
               |
                 ((?:[\-_.!~*'()a-zA-Z\d$,;:@&=+]|%[a-fA-F\d]{2})+)                 (?# 6: registry)
               )
             |
             (?!//))                           (?# XXX: '//' is the mark for hostport)
             (/(?:[\-_.!~*'()a-zA-Z\d:@&=+$,]|%[a-fA-F\d]{2})*(?:;(?:[\-_.!~*'()a-zA-Z\d:@&=+$,]|%[a-fA-F\d]{2})*)*(?:/(?:[\-_.!~*'()a-zA-Z\d:@&=+$,]|%[a-fA-F\d]{2})*(?:;(?:[\-_.!~*'()a-zA-Z\d:@&=+$,]|%[a-fA-F\d]{2})*)*)*)?                    (?# 7: path)
           )(?:\?((?:[\-_.!~*'()a-zA-Z\d;/?:@&=+$,\[\]]|%[a-fA-F\d]{2})*))?                 (?# 8: query)
        )
        (?:\#((?:[\-_.!~*'()a-zA-Z\d;/?:@&=+$,\[\]]|%[a-fA-F\d]{2})*))?                  (?# 9: fragment)
      `
