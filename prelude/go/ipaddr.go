//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbIPAddrParse parses "addr"/"addr/N"/"addr/netmask" like MRI's IPAddr.new; the third return is nil, or a ready-to-panic error.
func rbIPAddrParse(s string) (netip.Addr, int, any) {
	addrPart, maskPart, hasMask := strings.Cut(s, "/")
	addr, err := netip.ParseAddr(addrPart)
	if err != nil {
		return netip.Addr{}, 0, NewIPAddr_InvalidAddressError(Ref(String("invalid address: " + addrPart)))
	}
	bits := addr.BitLen()
	if hasMask {
		spec := any(String(maskPart))
		if n, err2 := strconv.Atoi(maskPart); err2 == nil {
			spec = Integer(n)
		}
		var exc any
		bits, exc = rbIPAddrMaskSpec(addr, spec)
		if exc != nil {
			return netip.Addr{}, 0, exc
		}
	}
	return netip.PrefixFrom(addr, bits).Masked().Addr(), bits, nil
}

// rbIPAddrMaskSpec turns mask's Integer (prefix length) or String (netmask) argument into a prefix length.
func rbIPAddrMaskSpec(addr netip.Addr, spec any) (int, any) {
	switch v := spec.(type) {
	case Integer:
		n := int(v)
		if n < 0 || n > addr.BitLen() {
			return 0, NewIPAddr_InvalidPrefixError(Ref(String("invalid length")))
		}

		return n, nil
	case String:
		maskAddr, err := netip.ParseAddr(string(v))
		if err != nil || maskAddr.BitLen() != addr.BitLen() {
			return 0, NewIPAddr_InvalidAddressError(Ref(String("invalid address: " + string(v))))
		}

		n, ok := rbIPAddrMaskBits(maskAddr)
		if !ok {
			return 0, NewIPAddr_InvalidPrefixError(Ref(String("invalid mask " + string(v))))
		}

		return n, nil
	default:
		return 0, NewIPAddr_InvalidPrefixError(Ref(String("invalid length")))
	}
}

// rbIPAddrMaskBits counts leading 1 bits, rejecting a netmask whose 1s aren't contiguous.
func rbIPAddrMaskBits(a netip.Addr) (int, bool) {
	ones, seenZero := 0, false

	for _, byteVal := range a.AsSlice() {
		for bit := 7; bit >= 0; bit-- {
			if byteVal&(1<<uint(bit)) != 0 {
				if seenZero {
					return 0, false
				}

				ones++
			} else {
				seenZero = true
			}
		}
	}

	return ones, true
}

// rbIPAddrCoerce turns <=>/==/include?'s argument into an address+prefix; ok is false for anything but an IPAddr or String.
func rbIPAddrCoerce(other any) (addr netip.Addr, bits int, ok bool, exc any) {
	switch v := other.(type) {
	case *IPAddr:
		return v.addr, v.bits, true, nil
	case String:
		a, b, e := rbIPAddrParse(string(v))
		if e != nil {
			return netip.Addr{}, 0, false, e
		}

		return a, b, true, nil
	default:
		return netip.Addr{}, 0, false, nil
	}
}

// rbIPAddrBroadcast sets every host bit past the prefix length to 1, the last address of addr/bits.
func rbIPAddrBroadcast(addr netip.Addr, bits int) netip.Addr {
	n := new(big.Int).SetBytes(addr.AsSlice())

	host := addr.BitLen() - bits
	if host > 0 {
		ones := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(host)), big.NewInt(1))
		n.Or(n, ones)
	}

	return rbIPAddrFromBig(addr, n)
}

// rbIPAddrSucc returns addr+1, or ok=false with the overflowed value's decimal, like MRI's succ.
func rbIPAddrSucc(addr netip.Addr) (netip.Addr, string, bool) {
	n := new(big.Int).SetBytes(addr.AsSlice())
	n.Add(n, big.NewInt(1))

	limit := new(big.Int).Lsh(big.NewInt(1), uint(addr.BitLen()))
	if n.Cmp(limit) >= 0 {
		return netip.Addr{}, n.String(), false
	}

	return rbIPAddrFromBig(addr, n), "", true
}

// rbIPAddrFromBig renders n (which must fit) into a 4- or 16-byte netip.Addr, matching like's width.
func rbIPAddrFromBig(like netip.Addr, n *big.Int) netip.Addr {
	if like.Is4() {
		var arr [4]byte

		n.FillBytes(arr[:])

		return netip.AddrFrom4(arr)
	}

	var arr [16]byte

	n.FillBytes(arr[:])

	return netip.AddrFrom16(arr)
}

// rbIPAddrInspectAddr formats addr fully expanded, with no "::" compression, like MRI's IPAddr#inspect.
func rbIPAddrInspectAddr(a netip.Addr) string {
	if a.Is4() {
		return a.String()
	}

	b := a.As16()
	parts := make([]string, 8)

	for i := range 8 {
		parts[i] = fmt.Sprintf("%04x", uint16(b[2*i])<<8|uint16(b[2*i+1]))
	}

	return strings.Join(parts, ":")
}

// rbIPAddrNetmask formats the netmask for bits over like's family, fully expanded like rbIPAddrInspectAddr.
func rbIPAddrNetmask(like netip.Addr, bits int) string {
	width := 4
	if like.Is6() {
		width = 16
	}

	b := make([]byte, width)
	for i := range bits {
		b[i/8] |= 1 << uint(7-i%8)
	}

	if width == 4 {
		return netip.AddrFrom4([4]byte(b)).String()
	}

	return rbIPAddrInspectAddr(netip.AddrFrom16([16]byte(b)))
}
