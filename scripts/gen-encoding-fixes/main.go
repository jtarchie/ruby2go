// Command gen-encoding-fixes writes prelude/go/encoding_xtext_fixes.go: where MRI's single-byte transcoders differ
// from x/text's tables, MRI's mapping, so rugaroo's transcoding is MRI's with x/text as the data (decision 380).
//
//	go -C scripts/gen-encoding-fixes run .    # rerun after a Ruby or x/text upgrade (its own module, as scripts/census)
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"log"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

// root is the repository, from this module's directory.
const root = "../../"

// singleByte must list rbSingleByte's encodings with the same tables (prelude/go/encoding_xtext.go).
var singleByte = map[string]*charmap.Charmap{
	"ISO-8859-2": charmap.ISO8859_2, "ISO-8859-3": charmap.ISO8859_3, "ISO-8859-4": charmap.ISO8859_4,
	"ISO-8859-5": charmap.ISO8859_5, "ISO-8859-6": charmap.ISO8859_6, "ISO-8859-7": charmap.ISO8859_7,
	"ISO-8859-8": charmap.ISO8859_8, "ISO-8859-9": charmap.ISO8859_9, "ISO-8859-10": charmap.ISO8859_10,
	"ISO-8859-13": charmap.ISO8859_13, "ISO-8859-14": charmap.ISO8859_14, "ISO-8859-15": charmap.ISO8859_15,
	"ISO-8859-16":  charmap.ISO8859_16,
	"Windows-1250": charmap.Windows1250, "Windows-1251": charmap.Windows1251, "Windows-1252": charmap.Windows1252,
	"Windows-1253": charmap.Windows1253, "Windows-1254": charmap.Windows1254, "Windows-1255": charmap.Windows1255,
	"Windows-1256": charmap.Windows1256, "Windows-1257": charmap.Windows1257, "Windows-874": charmap.Windows874,
	"KOI8-R": charmap.KOI8R, "KOI8-U": charmap.KOI8U,
	"IBM437": charmap.CodePage437, "CP850": charmap.CodePage850, "IBM852": charmap.CodePage852,
	"IBM855": charmap.CodePage855, "CP852": charmap.CodePage852, "CP855": charmap.CodePage855,
	"ISO-8859-11": charmap.Windows874, "TIS-620": charmap.Windows874, "IBM860": charmap.CodePage860, "IBM862": charmap.CodePage862,
	"IBM863": charmap.CodePage863, "IBM865": charmap.CodePage865, "IBM866": charmap.CodePage866,
	"macRoman": charmap.Macintosh, "macCyrillic": charmap.MacintoshCyrillic,
}

// mriDump prints, for encoding ARGV[0]: "d BYTE CP" for each of the 256 bytes (CP -1: undefined), "e CP BYTE" for each
// BMP character it encodes, and "n NAME" with the converter name MRI's encode errors give.
const mriDump = `
e = ARGV[0]
256.times do |b|
  cp = begin
    [b].pack("C").encode("UTF-8", e).ord
  rescue EncodingError
    -1
  end
  puts "d #{b} #{cp}"
end
(0..0xFFFF).each do |cp|
  next if cp.between?(0xD800, 0xDFFF)
  begin
    puts "e #{cp} #{[cp].pack("U").encode(e).bytes.join(",")}"
  rescue EncodingError
  end
end
begin
  "\u{10FFFF}".encode(e)
rescue Encoding::UndefinedConversionError => ex
  puts "n #{ex.message[/ to (\S+)\z/, 1]}"
end
`

func main() {
	if os.Getenv("GEN_MB_PROBE") != "" {
		mbProbe(strings.Fields(os.Getenv("GEN_MB_PROBE")))
		return
	}
	names := make([]string, 0, len(singleByte))
	for n := range singleByte {
		names = append(names, n)
	}
	slices.Sort(names)
	mbNames := make([]string, 0, len(multiByte))
	for n := range multiByte {
		mbNames = append(mbNames, n)
	}
	slices.Sort(mbNames)
	family := map[string]string{}
	for _, n := range names {
		family[n] = "SingleByte"
	}
	for _, n := range mbNames {
		family[n] = multiByte[n].family
	}
	aliases := writeMeta(append(slices.Clone(names), mbNames...), family)
	var dec, enc, conv strings.Builder
	for _, name := range names {
		cm := singleByte[name]
		out, err := exec.Command("ruby", "-e", mriDump, name).Output()
		if err != nil {
			log.Fatalf("ruby %s: %v", name, err)
		}
		mriDec := map[int]int{}
		mriEnc := map[int]string{}
		sc := bufio.NewScanner(bytes.NewReader(out))
		for sc.Scan() {
			f := strings.Fields(sc.Text())
			switch f[0] {
			case "d":
				a, _ := strconv.Atoi(f[1])
				b, _ := strconv.Atoi(f[2])
				mriDec[a] = b
			case "e":
				a, _ := strconv.Atoi(f[1])
				var bs []byte
				for _, x := range strings.Split(f[2], ",") {
					n, _ := strconv.Atoi(x)
					bs = append(bs, byte(n))
				}
				mriEnc[a] = string(bs)
			case "n":
				if f[1] != name {
					fmt.Fprintf(&conv, "%q: %q,\n", name, f[1])
				}
			}
		}
		var d, e []string
		for b := range 256 {
			x := int(cm.DecodeByte(byte(b)))
			if x == 0xFFFD {
				x = -1
			}
			if m := mriDec[b]; m != x {
				d = append(d, fmt.Sprintf("0x%02X: %d", b, m))
			}
		}
		for cp := range 0x10000 {
			if cp >= 0xD800 && cp <= 0xDFFF {
				continue
			}
			x := ""
			if c, ok := cm.EncodeRune(rune(cp)); ok {
				x = string([]byte{c})
			}
			if m := mriEnc[cp]; m != x {
				e = append(e, fmt.Sprintf("0x%04X: %q", cp, m))
			}
		}
		if len(d) > 0 {
			fmt.Fprintf(&dec, "%q: {%s},\n", name, strings.Join(d, ", "))
		}
		if len(e) > 0 {
			fmt.Fprintf(&enc, "%q: {%s},\n", name, strings.Join(e, ", "))
		}
	}
	var isSB string
	for _, n := range names {
		isSB += fmt.Sprintf("%q: true,\n", n)
	}
	var outs []mbOut
	for _, n := range mbNames {
		outs = append(outs, genMB(n))
	}
	for _, c := range emitMB(outs) {
		conv.WriteString(c + "\n")
	}
	src := `//go:build rugaroo_prelude

// Code generated by scripts/gen-encoding-fixes from MRI ` + rubyVersion() + ` and x/text; DO NOT EDIT.

package prelude

// rbSBDecodeFix is MRI's character for a byte where x/text's table differs (-1: MRI leaves it undefined).
var rbSBDecodeFix = map[string]map[byte]rune{
` + dec.String() + `}

// rbSBEncodeFix is MRI's bytes for a character where x/text's table differs ("": MRI cannot encode it; Windows-1255
// writes some presentation forms as a letter and its point).
var rbSBEncodeFix = map[string]map[rune]string{
` + enc.String() + `}

// rbXConvName is the converter name MRI's errors give where it is not the encoding's own (WINDOWS-1252).
var rbXConvName = map[string]string{
` + conv.String() + `}

// rbIsSingleByte are the single-byte encodings rbUseEncSingleByte links.
var rbIsSingleByte = map[string]bool{
` + isSB + `}

// rbXEncAliases are the x/text encodings' names, upper-cased as rbEncCanon looks them up, to MRI's canonical one.
var rbXEncAliases = map[string]string{
` + aliases + `}
`
	b, err := format.Source([]byte(src))
	if err != nil {
		log.Fatalf("gofmt: %v\n%s", err, src)
	}
	if err := os.WriteFile(root+"prelude/go/encoding_xtext_fixes.go", b, 0o600); err != nil {
		log.Fatal(err)
	}
}

// mriMeta prints MRI's facts about the encodings rugaroo has (the ten of decision 136 and ARGV's): each one's name,
// names, ascii_compatible?, dummy? and constants, in Encoding.list order, and Encoding.name_list restricted to them.
const mriMeta = `
require "json"
have = %w[ASCII-8BIT UTF-8 US-ASCII UTF-16BE UTF-16LE UTF-32BE UTF-32LE UTF-16 UTF-32 ISO-8859-1].map { |n| Encoding.find(n) } + ARGV.map { |n| Encoding.find(n) }
consts = Encoding.constants.group_by { |c| Encoding.const_get(c) }
list = Encoding.list.select { |e| have.include?(e) }.map do |e|
  cs = consts[e].map(&:to_s)
  main = cs.include?(e.name.upcase.tr("-", "_")) ? e.name.upcase.tr("-", "_") : cs.first
  { name: e.name, names: e.names, ascii: e.ascii_compatible?, dummy: e.dummy?, const: main, aliases: cs - [main] }
end
names = Encoding.name_list.select { |n| (have.include?(Encoding.find(n)) rescue false) }
puts JSON.generate({ list: list, name_list: names })
`

type mriEnc struct {
	Name    string   `json:"name"`
	Names   []string `json:"names"`
	ASCII   bool     `json:"ascii"`
	Dummy   bool     `json:"dummy"`
	Const   string   `json:"const"`
	Aliases []string `json:"aliases"`
}

// writeMeta rewrites prelude/encoding.rb's generated region (the constants, Encoding.list, Encoding.name_list) and
// internal/compiler/encoding_names.go (the names a literal may give), and returns the alias table for the Go side.
func writeMeta(xnames []string, family map[string]string) string {
	out, err := exec.Command("ruby", append([]string{"-e", mriMeta}, xnames...)...).Output()
	if err != nil {
		log.Fatalf("ruby meta: %v", err)
	}
	var meta struct {
		List     []mriEnc `json:"list"`
		NameList []string `json:"name_list"`
	}
	if err := json.Unmarshal(out, &meta); err != nil {
		log.Fatal(err)
	}
	x := map[string]bool{}
	for _, n := range xnames {
		x[n] = true
	}
	var rb, aliases, names, consts strings.Builder
	rb.WriteString("  # BEGIN scripts/gen-encoding-fixes: the x/text encodings (decision 380), MRI's list and names\n")
	var list []string
	for _, e := range meta.List {
		list = append(list, e.Const)
		if !x[e.Name] {
			continue
		}
		fmt.Fprintf(&rb, "  %s = __new(%q, %q, %v, %v) #: Encoding\n", e.Const, e.Name, strings.Join(e.Names, ","), e.ASCII, e.Dummy)
		for _, a := range e.Aliases {
			fmt.Fprintf(&rb, "  %s = %s #: Encoding\n", a, e.Const)
		}
		if f := family[e.Name]; f != "" {
			for _, c := range append([]string{e.Const}, e.Aliases...) {
				fmt.Fprintf(&consts, "%q: %q,\n", c, f)
			}
		}
		for _, n := range e.Names {
			fmt.Fprintf(&aliases, "%q: %q,\n", strings.ToUpper(n), e.Name)
			fmt.Fprintf(&names, "%q: %q,\n", strings.ToUpper(n), family[e.Name])
		}
	}
	fmt.Fprintf(&rb, "  LIST__ = [%s] #: Array[Encoding]\n", strings.Join(list, ", "))
	fmt.Fprintf(&rb, "  NAME_LIST__ = %%w[%s] #: Array[String]\n", strings.Join(meta.NameList, " "))
	rb.WriteString("  # END scripts/gen-encoding-fixes\n")
	const path = root + "prelude/encoding.rb"
	src, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	start := bytes.Index(src, []byte("  # BEGIN scripts/gen-encoding-fixes"))
	end := bytes.Index(src, []byte("  # END scripts/gen-encoding-fixes\n"))
	if start < 0 || end < 0 {
		log.Fatalf("%s: no generated region", path)
	}
	src = slices.Concat(src[:start], []byte(rb.String()), src[end+len("  # END scripts/gen-encoding-fixes\n"):])
	if err := os.WriteFile(path, src, 0o600); err != nil {
		log.Fatal(err)
	}
	goNames := "// Code generated by scripts/gen-encoding-fixes; DO NOT EDIT.\n\npackage compiler\n\n// xtextEncodings are the names of the encodings x/text transcodes (decision 380), upper-cased, to the family whose\n// rbUseEnc func links their tables (\"\": a single-byte one, always linked).\nvar xtextEncodings = map[string]string{\n" + names.String() + "}\n\n// xtextEncodingConsts are the multibyte encodings' Encoding constants, to their family.\nvar xtextEncodingConsts = map[string]string{\n" + consts.String() + "}\n"
	b, err := format.Source([]byte(goNames))
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(root+"internal/compiler/encoding_names.go", b, 0o600); err != nil {
		log.Fatal(err)
	}
	return aliases.String()
}

func rubyVersion() string {
	out, err := exec.Command("ruby", "-e", "print RUBY_VERSION").Output()
	if err != nil {
		log.Fatal(err)
	}
	return string(out)
}
