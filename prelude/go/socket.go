//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbSockConsts are Socket's constants, from this platform's syscall package as MRI's come from its headers (decision 135).
var rbSockConsts = map[string]int{
	"AF_INET": syscall.AF_INET, "AF_INET6": syscall.AF_INET6, "AF_UNIX": syscall.AF_UNIX, "AF_UNSPEC": syscall.AF_UNSPEC,
	"PF_INET": syscall.AF_INET, "PF_INET6": syscall.AF_INET6, "PF_UNIX": syscall.AF_UNIX, "PF_UNSPEC": syscall.AF_UNSPEC,
	"SOCK_STREAM": syscall.SOCK_STREAM, "SOCK_DGRAM": syscall.SOCK_DGRAM, "SOCK_RAW": syscall.SOCK_RAW,
	"SOL_SOCKET": syscall.SOL_SOCKET, "SO_REUSEADDR": syscall.SO_REUSEADDR, "SO_REUSEPORT": syscall.SO_REUSEPORT,
	"SO_KEEPALIVE": syscall.SO_KEEPALIVE, "SO_LINGER": syscall.SO_LINGER, "SO_BROADCAST": syscall.SO_BROADCAST,
	"SO_RCVBUF": syscall.SO_RCVBUF, "SO_SNDBUF": syscall.SO_SNDBUF, "SO_TYPE": syscall.SO_TYPE, "SO_ERROR": syscall.SO_ERROR,
	"IPPROTO_IP": syscall.IPPROTO_IP, "IPPROTO_IPV6": syscall.IPPROTO_IPV6, "IPPROTO_TCP": syscall.IPPROTO_TCP, "IPPROTO_UDP": syscall.IPPROTO_UDP,
	"TCP_NODELAY": syscall.TCP_NODELAY, "INADDR_ANY": 0, "SHUT_RD": syscall.SHUT_RD, "SHUT_WR": syscall.SHUT_WR, "SHUT_RDWR": syscall.SHUT_RDWR,
	"MSG_PEEK": syscall.MSG_PEEK, "MSG_OOB": syscall.MSG_OOB, "SOMAXCONN": syscall.SOMAXCONN,
}

// rbSockFamilyName is the family's name in MRI's addr arrays.
func rbSockFamilyName(fam int) string {
	switch fam {
	case syscall.AF_INET:
		return "AF_INET"
	case syscall.AF_INET6:
		return "AF_INET6"
	case syscall.AF_UNIX:
		return "AF_UNIX"
	}
	return "AF_UNSPEC"
}

// rbSockArg turns a family or socket-type argument (nil, an Integer, or a Symbol/String such as :INET or "SOCK_STREAM") into its number.
func rbSockArg(v any, prefix string) int {
	switch x := v.(type) {
	case nil:
		return 0
	case Integer:
		return int(x)
	case Symbol, String:
		name := strings.TrimPrefix(string(rbToS(x)), prefix)
		if n, ok := rbSockConsts[prefix+name]; ok {
			return n
		}
		panic(NewSocketError(Ref(String("unknown socket domain: " + name))))
	}
	panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(v) + " into Integer"))))
}

// rbSockErrnos are the Errno classes a socket call raises by number.
var rbSockErrnos = map[syscall.Errno]func(*String) any{
	syscall.ECONNREFUSED:  func(m *String) any { return NewErrno_ECONNREFUSED(m) },
	syscall.EADDRINUSE:    func(m *String) any { return NewErrno_EADDRINUSE(m) },
	syscall.EADDRNOTAVAIL: func(m *String) any { return NewErrno_EADDRNOTAVAIL(m) },
	syscall.EPIPE:         func(m *String) any { return NewErrno_EPIPE(m) },
	syscall.ECONNRESET:    func(m *String) any { return NewErrno_ECONNRESET(m) },
	syscall.ECONNABORTED:  func(m *String) any { return NewErrno_ECONNABORTED(m) },
	syscall.EAGAIN:        func(m *String) any { return NewErrno_EAGAIN(m) },
	syscall.ENOTCONN:      func(m *String) any { return NewErrno_ENOTCONN(m) },
	syscall.EISCONN:       func(m *String) any { return NewErrno_EISCONN(m) },
	syscall.EDESTADDRREQ:  func(m *String) any { return NewErrno_EDESTADDRREQ(m) },
	syscall.ETIMEDOUT:     func(m *String) any { return NewErrno_ETIMEDOUT(m) },
	syscall.EHOSTUNREACH:  func(m *String) any { return NewErrno_EHOSTUNREACH(m) },
	syscall.ENETUNREACH:   func(m *String) any { return NewErrno_ENETUNREACH(m) },
	syscall.EAFNOSUPPORT:  func(m *String) any { return NewErrno_EAFNOSUPPORT(m) },
	syscall.ENOENT:        func(m *String) any { return NewErrno_ENOENT(m) },
	syscall.EACCES:        func(m *String) any { return NewErrno_EACCES(m) },
	syscall.EINVAL:        func(m *String) any { return NewErrno_EINVAL(m) },
}

// rbSockErrno is MRI's Errno exception for e: C's strerror text (Go's table, capitalized as libc's), then " - detail".
func rbSockErrno(e syscall.Errno, detail string) any {
	s := e.Error()
	if s != "" {
		s = strings.ToUpper(s[:1]) + s[1:]
	}
	if detail != "" {
		s += " - " + detail
	}
	m := Ref(String(s))
	if f, ok := rbSockErrnos[e]; ok {
		return f(m)
	}
	return NewSystemCallError(m)
}

// rbGaiNoName is getaddrinfo(3)'s EAI_NONAME text on this platform (macOS and the BSDs, else glibc's).
func rbGaiNoName() string {
	if runtime.GOOS == "linux" {
		return "Name or service not known"
	}
	return "nodename nor servname provided, or not known"
}

// rbSockErr maps a Go net error to MRI's exception; label is "getaddrinfo" or "getaddrinfo(3)" as MRI words each caller.
func rbSockErr(err error, label, detail string) any {
	var dns *net.DNSError
	var errno syscall.Errno
	switch {
	case errors.As(err, &dns):
		msg := rbGaiNoName()
		if !dns.IsNotFound {
			msg = dns.Err
		}
		return NewSocket_ResolutionError(Ref(String(label + ": " + msg)))
	case errors.Is(err, net.ErrClosed):
		return NewIOError(Ref(String("closed stream")))
	case errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, context.DeadlineExceeded):
		return NewIO_TimeoutError(Ref(String("user specified timeout")))
	case errors.As(err, &errno):
		return rbSockErrno(errno, detail)
	}
	return NewIOError(Ref(String(err.Error())))
}

// rbSockFor is the " for ..." MRI appends to a failed connect or bind: a quoted host and port, or a Unix path.
func rbSockFor(call, host string, port int) string {
	return call + " for " + strconv.Quote(host) + " port " + strconv.Itoa(port)
}

// rbSockFile wraps a raw descriptor (from socketpair or accept, already close-on-exec) as a Go net.Conn; the descriptor itself is closed, net keeps a dup.
func rbSockFile(fd int) net.Conn {
	f := os.NewFile(uintptr(fd), "socket")
	defer func() { _ = f.Close() }()
	c, err := net.FileConn(f)
	if err != nil {
		panic(rbSockErr(err, "", ""))
	}
	return c
}

// rbSockNew is a fresh handle over a stream connection.
func rbSockNew(kind string, c net.Conn) *BasicSocket_Handle__ {
	return &BasicSocket_Handle__{kind: kind, c: c, r: bufio.NewReader(c)}
}

// rbSockDialTCP is TCPSocket.new and Socket.tcp: connect to host:port, from local when given, within timeout seconds when positive.
func rbSockDialTCP(host string, port int, local *String, localPort *Integer, timeout float64, forSocket bool) *BasicSocket_Handle__ {
	d := net.Dialer{Timeout: time.Duration(timeout * float64(time.Second))}
	if local != nil || localPort != nil {
		la := &net.TCPAddr{}
		if local != nil {
			ips, err := net.DefaultResolver.LookupNetIP(context.Background(), "ip", string(*local))
			if err != nil {
				panic(rbSockErr(err, "getaddrinfo(3)", ""))
			}
			la.IP = net.IP(ips[0].Unmap().AsSlice())
		}
		if localPort != nil {
			la.Port = int(*localPort)
		}
		d.LocalAddr = la
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	c, err := d.Dial("tcp", addr)
	if err != nil {
		if timeout > 0 && (errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, context.DeadlineExceeded)) {
			panic(NewIO_TimeoutError(Ref(String("user specified timeout for " + addr))))
		}
		detail := rbSockFor("connect(2)", host, port)
		if forSocket {
			detail = "connect(2) for " + addr
		}
		panic(rbSockErr(err, "getaddrinfo(3)", detail))
	}
	return rbSockNew("tcp", c)
}

// rbSockListenTCP is TCPServer.new: host "" listens on every address, as MRI's nil host.
func rbSockListenTCP(host string, port int) *BasicSocket_Handle__ {
	ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		panic(rbSockErr(err, "getaddrinfo(3)", rbSockFor("bind(2)", host, port)))
	}
	return &BasicSocket_Handle__{kind: "tcp", ln: ln}
}

// rbSockListenUnix is UNIXServer.new. The socket file stays after close, as in MRI (Go would unlink it).
func rbSockListenUnix(path string) *BasicSocket_Handle__ {
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		panic(rbSockErr(err, "", "connect(2) for "+path)) // MRI's message names connect(2) here too
	}
	ln.SetUnlinkOnClose(false)
	return &BasicSocket_Handle__{kind: "unix", ln: ln, path: Ref(path)}
}

// rbSockDialUnix is UNIXSocket.new.
func rbSockDialUnix(path string) *BasicSocket_Handle__ {
	c, err := net.Dial("unix", path)
	if err != nil {
		panic(rbSockErr(err, "", "connect(2) for "+path))
	}
	return rbSockNew("unix", c)
}

// rbSockPair is UNIXSocket.pair: two connected sockets of socktype (:STREAM or :DGRAM).
func rbSockPair(socktype any) (*BasicSocket_Handle__, *BasicSocket_Handle__) {
	st := rbSockArg(socktype, "SOCK_")
	if st == 0 {
		st = syscall.SOCK_STREAM
	}
	syscall.ForkLock.RLock() // as net's own sockets: no fork may inherit the descriptors before close-on-exec is set
	fds, err := syscall.Socketpair(syscall.AF_UNIX, st, 0)
	if err == nil {
		syscall.CloseOnExec(fds[0])
		syscall.CloseOnExec(fds[1])
	}
	syscall.ForkLock.RUnlock()
	if err != nil {
		panic(rbSockErr(err, "", "socketpair(2)"))
	}
	return rbSockNew("unix", rbSockFile(fds[0])), rbSockNew("unix", rbSockFile(fds[1]))
}

// rbSockUDP is UDPSocket.new: an unbound datagram socket of family, so bind, connect and sendto are the kernel's own.
func rbSockUDP(fam int) *BasicSocket_Handle__ {
	syscall.ForkLock.RLock()
	fd, err := syscall.Socket(fam, syscall.SOCK_DGRAM, 0)
	if err == nil {
		syscall.CloseOnExec(fd)
	}
	syscall.ForkLock.RUnlock()
	if err != nil {
		panic(rbSockErr(err, "", "socket(2) - udp"))
	}
	f := os.NewFile(uintptr(fd), "udp")
	defer func() { _ = f.Close() }()
	pc, err := net.FilePacketConn(f)
	if err != nil {
		panic(rbSockErr(err, "", ""))
	}
	return &BasicSocket_Handle__{kind: "udp", c: pc.(*net.UDPConn), fam: fam}
}

// rbSockSockaddr resolves host:port to a syscall sockaddr of the socket's family.
func (self *BasicSocket_Handle__) sockaddr(host string, port int) syscall.Sockaddr {
	ip := rbSockLookup(host, self.fam, "getaddrinfo")[0] // MRI's UDPSocket#bind/connect/send word it so, not getaddrinfo(3)
	if ip.Is4() && self.fam != syscall.AF_INET6 {
		return &syscall.SockaddrInet4{Port: port, Addr: ip.As4()}
	}
	return &syscall.SockaddrInet6{Port: port, Addr: ip.As16()}
}

// raw is the socket's descriptor access; a closed socket raises IOError.
func (self *BasicSocket_Handle__) raw() syscall.RawConn {
	if self.closed.Load() {
		panic(NewIOError(Ref(String("closed stream"))))
	}
	var sc syscall.Conn
	if self.ln != nil {
		sc = self.ln.(syscall.Conn)
	} else {
		sc = self.c.(syscall.Conn)
	}
	rc, err := sc.SyscallConn()
	if err != nil {
		panic(rbSockErr(err, "", ""))
	}
	return rc
}

// control runs f on the descriptor and returns f's error.
func (self *BasicSocket_Handle__) control(f func(fd int) error) error {
	var ferr error
	if err := self.raw().Control(func(fd uintptr) { ferr = f(int(fd)) }); err != nil {
		return err
	}
	return ferr
}

func (self *BasicSocket_Handle__) fd() int {
	n := -1
	_ = self.control(func(fd int) error {
		n = fd
		return nil
	})
	return n
}

// readable checks the read side is open, as MRI's IOError messages say.
func (self *BasicSocket_Handle__) readable() {
	if self.closed.Load() {
		panic(NewIOError(Ref(String("closed stream"))))
	}
	if self.rdone.Load() || self.r == nil {
		panic(NewIOError(Ref(String("not opened for reading"))))
	}
}

func (self *BasicSocket_Handle__) writable() {
	if self.closed.Load() {
		panic(NewIOError(Ref(String("closed stream"))))
	}
	if self.wdone.Load() || self.c == nil {
		panic(NewIOError(Ref(String("not opened for writing"))))
	}
}

// readErr raises for a read that failed other than at end of file.
func rbSockReadErr(err error) {
	if err != nil && !errors.Is(err, io.EOF) {
		panic(rbSockErr(err, "", ""))
	}
}

func (self *BasicSocket_Handle__) close() {
	if self.closed.Swap(true) {
		return
	}
	if self.ln != nil {
		_ = self.ln.Close()
	}
	if self.c != nil {
		_ = self.c.Close()
	}
}

// closeHalf shuts one direction (syscall.SHUT_RD or SHUT_WR); once both are shut the socket is closed, as MRI's.
func (self *BasicSocket_Handle__) closeHalf(how int) {
	if self.closed.Load() {
		return
	}
	other := &self.wdone
	mine := &self.rdone
	if how == syscall.SHUT_WR {
		other, mine = &self.rdone, &self.wdone
	}
	if other.Load() || self.ln != nil {
		mine.Store(true)
		self.close()
		return
	}
	mine.Store(true)
	_ = self.control(func(fd int) error { return syscall.Shutdown(fd, how) })
}

// rbSockAddrinfo is the Addrinfo for a socket address: socktype and protocol are the socket's.
func rbSockAddrinfo(sa syscall.Sockaddr, socktype, proto int) *Addrinfo {
	a := &Addrinfo{socktype: socktype, proto: proto}
	switch s := sa.(type) {
	case *syscall.SockaddrInet4:
		a.fam, a.ip, a.port = syscall.AF_INET, netip.AddrFrom4(s.Addr), s.Port
	case *syscall.SockaddrInet6:
		a.fam, a.ip, a.port = syscall.AF_INET6, netip.AddrFrom16(s.Addr), s.Port
		if s.ZoneId != 0 {
			if ifc, err := net.InterfaceByIndex(int(s.ZoneId)); err == nil {
				a.ip = a.ip.WithZone(ifc.Name)
			}
		}
	case *syscall.SockaddrUnix:
		a.fam, a.path = syscall.AF_UNIX, s.Name
	default:
		a.fam = syscall.AF_UNIX // an unbound Unix socket
	}
	return a
}

// info is the socket's own (peer false) or its peer's address as an Addrinfo.
func (self *BasicSocket_Handle__) info(peer bool) *Addrinfo {
	var sa syscall.Sockaddr
	err := self.control(func(fd int) error {
		var err error
		if peer {
			sa, err = syscall.Getpeername(fd)
		} else {
			sa, err = syscall.Getsockname(fd)
		}
		return err
	})
	if err != nil {
		panic(rbSockErr(err, "", map[bool]string{true: "getpeername(2)", false: "getsockname(2)"}[peer]))
	}
	st, proto := syscall.SOCK_STREAM, syscall.IPPROTO_TCP
	if self.kind == "udp" {
		st, proto = syscall.SOCK_DGRAM, syscall.IPPROTO_UDP
	}
	a := rbSockAddrinfo(sa, st, proto)
	if a.fam == syscall.AF_UNIX {
		a.proto = 0
	}
	if a.fam == syscall.AF_UNIX && self.kind == "unix" && self.dgram() {
		a.socktype = syscall.SOCK_DGRAM
	}
	return a
}

func (self *BasicSocket_Handle__) dgram() bool {
	n := 0
	_ = self.control(func(fd int) error {
		var err error
		n, err = syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_TYPE)
		return err
	})
	return n == syscall.SOCK_DGRAM
}

// rbSockIPAddr is IPSocket#addr's array for an address: family, port, host, ip (no reverse lookup, MRI's default).
func rbSockIPAddr(a *Addrinfo) Tuple4[String, Integer, String, String] {
	ip := String(a.ip.String())
	return Tuple4[String, Integer, String, String]{String(rbSockFamilyName(a.fam)), Integer(a.port), ip, ip}
}

// rbSockLookup resolves host to addresses of fam (0 for any), as getaddrinfo(3) would; label words the error.
func rbSockLookup(host string, fam int, label string) []netip.Addr {
	switch host { // MRI's host_str: "" and "<any>" are INADDR_ANY, "<broadcast>" INADDR_BROADCAST
	case "", "<any>":
		host = "0.0.0.0"
	case "<broadcast>":
		host = "255.255.255.255"
	}
	var ips []netip.Addr
	if ip, err := netip.ParseAddr(host); err == nil {
		ips = []netip.Addr{ip} // a numeric ::ffff:a.b.c.d stays IPv6, as getaddrinfo(3) answers it
	} else {
		network := map[int]string{syscall.AF_INET: "ip4", syscall.AF_INET6: "ip6"}[fam]
		if network == "" {
			network = "ip"
		}
		ips, err = net.DefaultResolver.LookupNetIP(context.Background(), network, host)
		if err != nil {
			panic(rbSockErr(err, label, ""))
		}
		for i, ip := range ips {
			ips[i] = ip.Unmap() // the resolver's 4-in-6 form of an IPv4 answer
		}
	}
	out := ips[:0:0]
	for _, ip := range ips {
		if fam == 0 || fam == syscall.AF_INET && ip.Is4() || fam == syscall.AF_INET6 && ip.Is6() {
			out = append(out, ip)
		}
	}
	if len(out) == 0 {
		panic(NewSocket_ResolutionError(Ref(String(label + ": " + rbGaiNoName()))))
	}
	return out
}

// rbSockService is a service argument's port: nil is 0, an Integer itself, a String a number or a service name.
func rbSockService(service any, label string) (int, bool) {
	switch s := service.(type) {
	case nil:
		return 0, false
	case Integer:
		return int(s), false
	case String:
		if n, err := strconv.Atoi(string(s)); err == nil {
			return n, false
		}
		n, err := net.LookupPort("tcp", string(s))
		if err != nil {
			panic(NewSocket_ResolutionError(Ref(String(label + ": " + rbGaiNoName()))))
		}
		return n, true
	}
	panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(service) + " into String"))))
}

// rbSockGetaddrinfo repeats each address per socket type (stream, datagram, raw) as getaddrinfo(3) does with no hints.
func rbSockGetaddrinfo(host string, service, family, socktype any, label string) []*Addrinfo {
	fam := rbSockArg(family, "AF_")
	st := rbSockArg(socktype, "SOCK_")
	port, named := rbSockService(service, label)
	name := ""
	if _, err := netip.ParseAddr(host); err != nil {
		name = host
	}
	if named {
		name += ":" + string(service.(String))
	}
	var out []*Addrinfo
	for _, ip := range rbSockLookup(host, fam, label) {
		f := syscall.AF_INET6
		if ip.Is4() {
			f = syscall.AF_INET
		}
		for _, t := range [][2]int{{syscall.SOCK_STREAM, syscall.IPPROTO_TCP}, {syscall.SOCK_DGRAM, syscall.IPPROTO_UDP}, {syscall.SOCK_RAW, 0}} {
			if st == 0 || st == t[0] {
				out = append(out, &Addrinfo{fam: f, ip: ip, port: port, socktype: t[0], proto: t[1], name: name})
			}
		}
	}
	return out
}

// rbSockInspect is IO#inspect for a socket: "fd N, family, address, port" for IP, the path for Unix sockets made from one.
func (self *BasicSocket_Handle__) inspect(cls string) string {
	if self.closed.Load() {
		return "#<" + cls + ":(closed)>"
	}
	if self.path != nil {
		return "#<" + cls + ":" + *self.path + ">"
	}
	s := "#<" + cls + ":fd " + strconv.Itoa(self.fd())
	if self.kind != "unix" {
		a := self.info(false)
		s += ", " + rbSockFamilyName(a.fam) + ", " + a.ip.String() + ", " + strconv.Itoa(a.port)
	}
	return s + ">"
}

// acceptNonblock is one accept(2) without waiting: IO::EAGAINWaitReadable when nothing is pending, as MRI's.
func (self *BasicSocket_Handle__) acceptNonblock() *BasicSocket_Handle__ {
	nfd := -1
	var aerr error
	if err := self.raw().Control(func(fd uintptr) { // a listener's RawConn.Read is EINVAL; its descriptor is non-blocking
		syscall.ForkLock.RLock()
		nfd, _, aerr = syscall.Accept(int(fd))
		if aerr == nil {
			syscall.CloseOnExec(nfd)
		}
		syscall.ForkLock.RUnlock()
	}); err != nil {
		panic(rbSockErr(err, "", ""))
	}
	if errors.Is(aerr, syscall.EAGAIN) {
		panic(NewIO_EAGAINWaitReadable(Ref(String("Resource temporarily unavailable - accept(2) would block"))))
	}
	if aerr != nil {
		panic(rbSockErr(aerr, "", "accept(2)"))
	}
	return rbSockNew(self.kind, rbSockFile(nfd))
}

// setsockopt sets an option on the descriptor: true/false are 1/0, an Integer itself, a String the raw bytes (a packed struct).
func (self *BasicSocket_Handle__) setsockopt(level, opt int, val any) {
	err := self.control(func(fd int) error {
		switch v := val.(type) {
		case Boolean:
			n := 0
			if v {
				n = 1
			}
			return syscall.SetsockoptInt(fd, level, opt, n)
		case Integer:
			return syscall.SetsockoptInt(fd, level, opt, int(v))
		case String:
			return syscall.SetsockoptString(fd, level, opt, string(v))
		}
		panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(val) + " into Integer"))))
	})
	if err != nil {
		panic(rbSockErr(err, "", "setsockopt(2)"))
	}
}

// rbSockaddrBytes is Addrinfo#to_sockaddr: the C struct sockaddr this platform packs (BSD's leading length byte on macOS).
func rbSockaddrBytes(a *Addrinfo) string {
	bsd := runtime.GOOS != "linux"
	var b []byte
	head := func(size, fam int) {
		b = make([]byte, size)
		if bsd {
			b[0], b[1] = byte(size), byte(fam)
		} else {
			binary.LittleEndian.PutUint16(b, uint16(fam))
		}
	}
	switch a.fam {
	case syscall.AF_INET:
		head(16, a.fam)
		binary.BigEndian.PutUint16(b[2:], uint16(a.port))
		ip := a.ip.As4()
		copy(b[4:], ip[:])
	case syscall.AF_INET6:
		head(28, a.fam)
		binary.BigEndian.PutUint16(b[2:], uint16(a.port))
		ip := a.ip.As16()
		copy(b[8:], ip[:])
	default:
		size := 110
		if bsd {
			size = 106
		}
		head(size, a.fam)
		copy(b[2:], a.path)
	}
	return string(b)
}

// rbSockIPAddressList is Socket.ip_address_list: every interface address, link-local IPv6 ones with their zone.
func rbSockIPAddressList() *Array[*Addrinfo] {
	out := NewArray[*Addrinfo]()
	ifs, err := net.Interfaces()
	if err != nil {
		panic(rbSockErr(err, "", "getifaddrs"))
	}
	for _, ifc := range ifs {
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			n, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip, _ := netip.AddrFromSlice(n.IP)
			ip = ip.Unmap()
			fam := syscall.AF_INET
			if !ip.Is4() {
				fam = syscall.AF_INET6
				if ip.IsLinkLocalUnicast() {
					ip = ip.WithZone(ifc.Name)
				}
			}
			*out = append(*out, &Addrinfo{fam: fam, ip: ip})
		}
	}
	return out
}
