# rbs_inline: enabled

# Sockets over Go's net (decision 135); BasicSocket is no IO subclass since IO is a @go_type (decision 62).

class SocketError < StandardError; end

class IO
  module WaitReadable; end
  module WaitWritable; end

  class TimeoutError < IOError; end

  # accept_nonblock's "would block", rescued as IO::WaitReadable or Errno::EAGAIN.
  class EAGAINWaitReadable < Errno::EAGAIN
    include IO::WaitReadable
  end
end

class BasicSocket < Object
  include IOWritable
  include IOReadable

  # @go_type struct { kind string; c net.Conn; ln net.Listener; r *bufio.Reader; path *string; fam int; rdone, wdone, closed atomic.Bool }
  class Handle__ < Object
    #: (String, Integer, String?, Integer?, Float?, bool) -> BasicSocket::Handle__
    def self.dial_tcp(host, port, local, local_port, timeout, for_socket) = %x{
      t := 0.0
      if timeout != nil {
        t = float64(*timeout)
      }
      return rbSockDialTCP(string(host), int(port), local, local_port, t, bool(for_socket))
    }

    #: (String?, Integer) -> BasicSocket::Handle__
    def self.listen_tcp(host, port) = %x{
      h := ""
      if host != nil {
        h = string(*host)
      }
      return rbSockListenTCP(h, int(port))
    }

    #: (String) -> BasicSocket::Handle__
    def self.listen_unix(path) = %x{ rbSockListenUnix(string(path)) }

    #: (String) -> BasicSocket::Handle__
    def self.dial_unix(path) = %x{ rbSockDialUnix(string(path)) }

    #: (Integer) -> BasicSocket::Handle__
    def self.udp(family) = %x{ rbSockUDP(int(family)) }

    #: (untyped) -> [BasicSocket::Handle__, BasicSocket::Handle__]
    def self.pair(socktype) = %x{
      a, b := rbSockPair(socktype)
      return Tuple2[*BasicSocket_Handle__, *BasicSocket_Handle__]{a, b}
    }

    #: () -> BasicSocket::Handle__
    def accept = %x{
      if self.ln == nil {
        panic(NewIOError(Ref(String("not a server socket"))))
      }
      self.raw()
      c, err := self.ln.Accept()
      if err != nil {
        panic(rbSockErr(err, "", "accept(2)"))
      }
      return rbSockNew(self.kind, c)
    }

    #: () -> BasicSocket::Handle__
    def accept_nonblock = %x{ self.acceptNonblock() }

    #: (untyped) -> Integer
    def write(x) = %x{
      self.writable()
      s := string(rbToS(x))
      if _, err := io.WriteString(self.c, s); err != nil {
        panic(rbSockErr(err, "", ""))
      }
      return Integer(len(s))
    }

    #: () -> String?
    def gets = %x{
      self.readable()
      line, err := self.r.ReadString('\\n')
      if line == "" {
        rbSockReadErr(err)
        return nil
      }
      return Ref(String(line))
    }

    #: () -> String
    def read_all = %x{
      self.readable()
      b, err := io.ReadAll(self.r)
      rbSockReadErr(err)
      return String(b)
    }

    #: (Integer) -> String?
    def read(n) = %x{
      self.readable()
      if n < 0 {
        panic(NewArgumentError(Ref(String(fmt.Sprintf("negative length %d given", n)))))
      }
      if n == 0 {
        return Ref(String(""))
      }
      buf := make([]byte, int(n))
      k, err := io.ReadFull(self.r, buf)
      if k == 0 {
        rbSockReadErr(err)
        return nil
      }
      if !errors.Is(err, io.ErrUnexpectedEOF) {
        rbSockReadErr(err)
      }
      return Ref(String(buf[:k]))
    }

    # nil at end of file, so the caller picks EOFError (readpartial) or "" (recv).
    #: (Integer) -> String?
    def readpartial(n) = %x{
      self.readable()
      if n < 0 {
        panic(NewArgumentError(Ref(String(fmt.Sprintf("negative length %d given", n)))))
      }
      if n == 0 {
        return Ref(String(""))
      }
      buf := make([]byte, int(n))
      k, err := self.r.Read(buf)
      if k == 0 {
        rbSockReadErr(err)
        return nil
      }
      return Ref(String(buf[:k]))
    }

    #: () -> bool
    def eof? = %x{
      self.readable()
      _, err := self.r.Peek(1)
      return Boolean(err != nil)
    }

    # recv(2)/recvfrom(2) with flags; bytes the reader already buffered come first, as a stream must stay in order.
    #: (Integer, Integer) -> [String, Addrinfo?]
    def recvfrom(n, flags) = %x{
      if self.closed.Load() {
        panic(NewIOError(Ref(String("closed stream"))))
      }
      if n < 0 {
        panic(NewArgumentError(Ref(String("negative string size (or size too big)"))))
      }
      if self.r != nil && self.r.Buffered() > 0 {
        k := min(int(n), self.r.Buffered())
        if int(flags)&syscall.MSG_PEEK != 0 { // a peek leaves the bytes for the next read
          b, _ := self.r.Peek(k)
          return Tuple2[String, **Addrinfo]{String(b), nil}
        }
        buf := make([]byte, k)
        k, _ = self.r.Read(buf)
        return Tuple2[String, **Addrinfo]{String(buf[:k]), nil}
      }
      buf := make([]byte, int(n))
      var k int
      var from syscall.Sockaddr
      var rerr error
      if err := self.raw().Read(func(fd uintptr) bool {
        k, from, rerr = syscall.Recvfrom(int(fd), buf, int(flags))
        return !errors.Is(rerr, syscall.EAGAIN)
      }); err != nil {
        panic(rbSockErr(err, "", ""))
      }
      if rerr != nil {
        panic(rbSockErr(rerr, "", "recvfrom(2)"))
      }
      var a **Addrinfo
      if from != nil {
        a = Ref(rbSockAddrinfo(from, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP))
      }
      return Tuple2[String, **Addrinfo]{String(buf[:max(k, 0)]), a}
    }

    #: (String, Integer, String?, Integer?) -> Integer
    def send(msg, flags, host, port) = %x{
      self.writable()
      var to syscall.Sockaddr
      call := "send(2)"
      if host != nil {
        p := 0
        if port != nil {
          p = int(*port)
        }
        to = self.sockaddr(string(*host), p)
        call = rbSockFor("sendto(2)", string(*host), p)
      }
      var n int
      var serr error
      if err := self.raw().Write(func(fd uintptr) bool {
        n, serr = syscall.SendmsgN(int(fd), []byte(msg), nil, to, int(flags)) // Sendto drops the count of a short stream write
        return !errors.Is(serr, syscall.EAGAIN)
      }); err != nil {
        panic(rbSockErr(err, "", ""))
      }
      if serr != nil {
        panic(rbSockErr(serr, "", call))
      }
      return Integer(n)
    }

    #: (String, Integer, bool) -> void
    def bind_or_connect(host, port, connect) = %x{
      sa := self.sockaddr(string(host), int(port))
      call := "bind(2)"
      if connect {
        call = "connect(2)"
      }
      err := self.control(func(fd int) error {
        if connect {
          return syscall.Connect(fd, sa)
        }
        return syscall.Bind(fd, sa)
      })
      if err != nil {
        panic(rbSockErr(err, "getaddrinfo(3)", rbSockFor(call, string(host), int(port))))
      }
    }

    #: () -> bool
    def stream? = %x{ Boolean(self.kind == "tcp" || self.kind == "unix" && !self.dgram()) }

    #: () -> void
    def close = %x{ self.close() }

    #: () -> bool
    def closed? = %x{ Boolean(self.closed.Load()) }

    #: () -> void
    def close_read = %x{ self.closeHalf(syscall.SHUT_RD) }

    #: () -> void
    def close_write = %x{ self.closeHalf(syscall.SHUT_WR) }

    #: (Integer) -> Integer
    def shutdown(how) = %x{
      err := self.control(func(fd int) error { return syscall.Shutdown(fd, int(how)) })
      if err != nil {
        panic(rbSockErr(err, "", "shutdown(2)"))
      }
      return 0
    }

    #: (bool) -> Addrinfo
    def info(peer) = %x{ self.info(bool(peer)) }

    #: (Integer, Integer, untyped) -> Integer
    def setsockopt(level, opt, val) = %x{
      self.setsockopt(int(level), int(opt), val)
      return 0
    }

    #: () -> Integer
    def fileno = %x{ Integer(self.fd()) }

    #: () -> String
    def path = %x{
      if self.path == nil { // kept once read, as MRI's pathv: inspect shows it from then on
        p := self.info(false).path
        self.path = &p
      }
      return String(*self.path)
    }

    #: (String) -> String
    def inspect_as(cls) = %x{ String(self.inspect(string(cls))) }
  end

  # @rbs @h: BasicSocket::Handle__

  # Only what is unrelated to buffering: sockets write straight through, so sync is always true.
  #: () -> bool
  def sync = true

  #: (bool) -> bool
  def sync=(on)
    on
  end

  #: (untyped) -> Integer
  def write(x) = @h.write(x)

  #: (untyped) -> self
  def <<(x)
    @h.write(x)
    self
  end

  #: () -> String?
  def gets = @h.gets

  #: () -> String
  def read = @h.read_all

  #: (Integer) -> String?
  def __read_1(n) = @h.read(n)

  #: () -> String
  def readline
    line = gets
    raise EOFError, "end of file reached" unless line
    line
  end

  #: (Integer) -> String
  def readpartial(n)
    s = @h.readpartial(n)
    raise EOFError, "end of file reached" unless s
    s
  end

  #: () -> bool
  def eof? = @h.eof?

  #: () -> bool
  def eof = @h.eof?

  # nil once the peer has closed a stream (MRI 3.3+); an empty datagram is "".
  #: (Integer, ?Integer) -> String?
  def recv(maxlen, flags = 0)
    msg = @h.recvfrom(maxlen, flags)[0]
    msg.empty? && maxlen > 0 && @h.stream? ? nil : msg
  end

  #: (String, ?Integer) -> Integer
  def send(msg, flags = 0) = @h.send(msg, flags, nil, nil)

  #: () -> nil
  def close
    @h.close
    nil
  end

  #: () -> bool
  def closed? = @h.closed?

  #: () -> nil
  def close_read
    @h.close_read
    nil
  end

  #: () -> nil
  def close_write
    @h.close_write
    nil
  end

  #: (?Integer) -> Integer
  def shutdown(how = Socket::SHUT_RDWR) = @h.shutdown(how)

  # Level and option are Integers (Socket::SOL_SOCKET, Socket::SO_REUSEADDR); the value true/false, an Integer or a packed String.
  #: (Integer, Integer, untyped) -> Integer
  def setsockopt(level, optname, optval) = @h.setsockopt(level, optname, optval)

  #: () -> Addrinfo
  def local_address = @h.info(false)

  #: () -> Addrinfo
  def remote_address = @h.info(true)

  #: () -> Integer
  def fileno = @h.fileno

  #: () -> String
  def inspect = @h.inspect_as(__class_name)
end

class IPSocket < BasicSocket
  #: (String) -> String
  def self.getaddress(host) = Addrinfo.ip(host).ip_address

  #: () -> [String, Integer, String, String]
  def addr = local_address.__addr

  #: () -> [String, Integer, String, String]
  def peeraddr = remote_address.__addr

end

class TCPSocket < IPSocket
  # recvfrom is per subclass, not IPSocket's: a stream's answer is nilable (no sender; nil at EOF), a datagram's not.
  #: (Integer, ?Integer) -> [String, [String, Integer, String, String]?]?
  def recvfrom(maxlen, flags = 0)
    msg, from = @h.recvfrom(maxlen, flags)
    return nil if msg.empty? && maxlen > 0

    [msg, from&.__addr]
  end

  #: (String, Integer, ?String?, ?Integer?, ?connect_timeout: Float?) -> void
  def initialize(host, port, local_host = nil, local_port = nil, connect_timeout: nil)
    @h = BasicSocket::Handle__.dial_tcp(host, port, local_host, local_port, connect_timeout, false)
  end

  #: (String, Integer, ?String?, ?Integer?, ?connect_timeout: Float?) -> TCPSocket
  def self.open(host, port, local_host = nil, local_port = nil, connect_timeout: nil) = new(host, port, local_host, local_port, connect_timeout: connect_timeout)

  #: (BasicSocket::Handle__) -> TCPSocket
  def self.__wrap(h) = %x{
    o := &TCPSocket{}
    o._BasicSocket().h = h
    return o
  }
end

class TCPServer < TCPSocket
  # A nil host listens on every address, as MRI's; TCPServer.new(port) is the same.
  #: (String?, Integer) -> void
  def initialize(host, port)
    @h = BasicSocket::Handle__.listen_tcp(host, port)
  end

  #: (Integer) -> TCPServer
  def self.__new_1(port) = new(nil, port)

  #: (String?, Integer) -> TCPServer
  def self.open(host, port) = new(host, port)

  #: (Integer) -> TCPServer
  def self.__open_1(port) = new(nil, port)

  #: () -> TCPSocket
  def accept = TCPSocket.__wrap(@h.accept)

  # Raises IO::EAGAINWaitReadable (an IO::WaitReadable and Errno::EAGAIN) when no connection is pending.
  #: () -> TCPSocket
  def accept_nonblock = TCPSocket.__wrap(@h.accept_nonblock)

  # Go's listener already listens with the system's backlog; a later listen(2) only changes the backlog.
  #: (Integer) -> Integer
  def listen(_backlog) = 0
end

class UDPSocket < IPSocket
  #: (?Integer) -> void
  def initialize(family = Socket::AF_INET)
    @h = BasicSocket::Handle__.udp(family)
  end

  #: (String, Integer) -> Integer
  def bind(host, port)
    @h.bind_or_connect(host, port, false)
    0
  end

  #: (String, Integer) -> Integer
  def connect(host, port)
    @h.bind_or_connect(host, port, true)
    0
  end

  # sendto(2) when host and port are given, else send(2) to the connected peer.
  #: (String, ?Integer, ?String?, ?Integer?) -> Integer
  def send(msg, flags = 0, host = nil, port = nil) = @h.send(msg, flags, host, port)

  # A datagram always has a sender, so UDP's answer needs no nil checks.
  #: (Integer, ?Integer) -> [String, [String, Integer, String, String]]
  def recvfrom(maxlen, flags = 0)
    msg, from = @h.recvfrom(maxlen, flags)
    [msg, (from || remote_address).__addr]
  end
end

class UNIXSocket < BasicSocket
  #: (String) -> void
  def initialize(path)
    @h = BasicSocket::Handle__.dial_unix(path)
  end

  #: (BasicSocket::Handle__) -> UNIXSocket
  def self.__wrap(h) = %x{
    o := &UNIXSocket{}
    o._BasicSocket().h = h
    return o
  }

  # Two connected sockets (socketpair(2)); socktype :STREAM (the default) or :DGRAM.
  #: (?untyped) -> [UNIXSocket, UNIXSocket]
  def self.pair(socktype = :STREAM)
    a, b = BasicSocket::Handle__.pair(socktype)
    [UNIXSocket.__wrap(a), UNIXSocket.__wrap(b)]
  end

  #: (?untyped) -> [UNIXSocket, UNIXSocket]
  def self.socketpair(socktype = :STREAM) = pair(socktype)

  #: () -> String
  def path = @h.path

  #: () -> [String, String]
  def addr = ["AF_UNIX", local_address.__path]

  #: () -> [String, String]
  def peeraddr = ["AF_UNIX", remote_address.__path]
end

class UNIXServer < UNIXSocket
  #: (String) -> void
  def initialize(path)
    @h = BasicSocket::Handle__.listen_unix(path)
  end

  #: () -> UNIXSocket
  def accept = UNIXSocket.__wrap(@h.accept)

  #: () -> UNIXSocket
  def accept_nonblock = UNIXSocket.__wrap(@h.accept_nonblock)

  #: (Integer) -> Integer
  def listen(_backlog) = 0
end

class Socket < BasicSocket
  class ResolutionError < SocketError; end

  AF_INET = __const("AF_INET") #: Integer
  AF_INET6 = __const("AF_INET6") #: Integer
  AF_UNIX = __const("AF_UNIX") #: Integer
  AF_UNSPEC = __const("AF_UNSPEC") #: Integer
  PF_INET = __const("PF_INET") #: Integer
  PF_INET6 = __const("PF_INET6") #: Integer
  PF_UNIX = __const("PF_UNIX") #: Integer
  PF_UNSPEC = __const("PF_UNSPEC") #: Integer
  SOCK_STREAM = __const("SOCK_STREAM") #: Integer
  SOCK_DGRAM = __const("SOCK_DGRAM") #: Integer
  SOCK_RAW = __const("SOCK_RAW") #: Integer
  SOL_SOCKET = __const("SOL_SOCKET") #: Integer
  SO_REUSEADDR = __const("SO_REUSEADDR") #: Integer
  SO_REUSEPORT = __const("SO_REUSEPORT") #: Integer
  SO_KEEPALIVE = __const("SO_KEEPALIVE") #: Integer
  SO_LINGER = __const("SO_LINGER") #: Integer
  SO_BROADCAST = __const("SO_BROADCAST") #: Integer
  SO_RCVBUF = __const("SO_RCVBUF") #: Integer
  SO_SNDBUF = __const("SO_SNDBUF") #: Integer
  SO_TYPE = __const("SO_TYPE") #: Integer
  SO_ERROR = __const("SO_ERROR") #: Integer
  IPPROTO_IP = __const("IPPROTO_IP") #: Integer
  IPPROTO_IPV6 = __const("IPPROTO_IPV6") #: Integer
  IPPROTO_TCP = __const("IPPROTO_TCP") #: Integer
  IPPROTO_UDP = __const("IPPROTO_UDP") #: Integer
  TCP_NODELAY = __const("TCP_NODELAY") #: Integer
  INADDR_ANY = __const("INADDR_ANY") #: Integer
  SHUT_RD = __const("SHUT_RD") #: Integer
  SHUT_WR = __const("SHUT_WR") #: Integer
  SHUT_RDWR = __const("SHUT_RDWR") #: Integer
  MSG_PEEK = __const("MSG_PEEK") #: Integer
  MSG_OOB = __const("MSG_OOB") #: Integer
  SOMAXCONN = __const("SOMAXCONN") #: Integer

  #: (String) -> Integer
  def self.__const(name) = %x{ Integer(rbSockConsts[string(name)]) }

  #: (BasicSocket::Handle__) -> Socket
  def self.__wrap(h) = %x{
    o := &Socket{}
    o._BasicSocket().h = h
    return o
  }

  # The connected Socket for the block, closed when it returns; the block's value is the result.
  #: [T] (String, Integer, ?String?, ?Integer?, ?connect_timeout: Float?) { (Socket) -> T } -> T
  def self.tcp(host, port, local_host = nil, local_port = nil, connect_timeout: nil)
    s = Socket.__wrap(BasicSocket::Handle__.dial_tcp(host, port, local_host, local_port, connect_timeout, true))
    begin
      yield s
    ensure
      s.close
    end
  end

  #: (String, Integer, ?String?, ?Integer?, ?connect_timeout: Float?) -> Socket
  def self.__tcp_enum(host, port, local_host = nil, local_port = nil, connect_timeout: nil) = Socket.__wrap(BasicSocket::Handle__.dial_tcp(host, port, local_host, local_port, connect_timeout, true))

  #: () -> String
  def self.gethostname = %x{
    h, err := os.Hostname()
    if err != nil {
      panic(rbSockErr(err, "", "gethostname(3)"))
    }
    return String(h)
  }

  # Each address once per socket type, as getaddrinfo(3) answers with no hints: [family, port, host, ip, afamily, socktype, protocol].
  #: (String, untyped, ?untyped, ?untyped) -> Array[[String, Integer, String, String, Integer, Integer, Integer]]
  def self.getaddrinfo(host, service, family = nil, socktype = nil)
    Addrinfo.getaddrinfo(host, service, family, socktype).map do |ai|
      a = ai.__addr
      row = [a[0], a[1], a[2], a[3], ai.afamily, ai.socktype, ai.protocol] #: [String, Integer, String, String, Integer, Integer, Integer]
      row
    end
  end

  #: () -> Array[Addrinfo]
  def self.ip_address_list = %x{ rbSockIPAddressList() }
end

# An address with its socket type and protocol: what getaddrinfo(3) answers.
# @go_type struct { fam int; ip netip.Addr; port int; path string; socktype int; proto int; name string }
class Addrinfo < Object
  #: (String, untyped, untyped, untyped, String) -> Array[Addrinfo]
  def self.__lookup(host, service, family, socktype, label) = %x{
    return &Array[*Addrinfo]{s: rbSockGetaddrinfo(string(host), service, family, socktype, string(label))}
  }

  #: (String, ?untyped, ?untyped, ?untyped) -> Array[Addrinfo]
  def self.getaddrinfo(host, service = nil, family = nil, socktype = nil) = __lookup(host, service, family, socktype, "getaddrinfo")

  #: (String, untyped) -> Addrinfo
  def self.tcp(host, service) = __lookup(host, service, nil, Socket::SOCK_STREAM, "getaddrinfo").first || raise(SocketError, "no address")

  #: (String, untyped) -> Addrinfo
  def self.udp(host, service) = __lookup(host, service, nil, Socket::SOCK_DGRAM, "getaddrinfo").first || raise(SocketError, "no address")

  # Just the address: no port, socket type or protocol.
  #: (String) -> Addrinfo
  def self.ip(host) = %x{
    a := rbSockGetaddrinfo(string(host), nil, nil, nil, "getaddrinfo")[0]
    return &Addrinfo{fam: a.fam, ip: a.ip, name: a.name}
  }

  #: (String, ?untyped) -> Addrinfo
  def self.unix(path, socktype = :STREAM) = %x{
    st := rbSockArg(socktype, "SOCK_")
    if st == 0 {
      st = syscall.SOCK_STREAM
    }
    return &Addrinfo{fam: syscall.AF_UNIX, path: string(path), socktype: st}
  }

  #: () -> Integer
  def afamily = %x{ Integer(self.fam) }

  #: () -> Integer
  def pfamily = %x{ Integer(self.fam) }

  #: () -> Integer
  def socktype = %x{ Integer(self.socktype) }

  #: () -> Integer
  def protocol = %x{ Integer(self.proto) }

  #: () -> bool
  def ip? = %x{ Boolean(self.fam != syscall.AF_UNIX) }

  #: () -> bool
  def ipv4? = %x{ Boolean(self.fam == syscall.AF_INET) }

  #: () -> bool
  def ipv6? = %x{ Boolean(self.fam == syscall.AF_INET6) }

  #: () -> bool
  def unix? = %x{ Boolean(self.fam == syscall.AF_UNIX) }

  #: () -> bool
  def ipv4_loopback? = %x{ Boolean(self.fam == syscall.AF_INET && self.ip.IsLoopback()) }

  #: () -> bool
  def ipv6_loopback? = %x{ Boolean(self.fam == syscall.AF_INET6 && self.ip.IsLoopback()) }

  #: () -> String
  def ip_address = %x{
    if self.fam == syscall.AF_UNIX {
      panic(NewSocketError(Ref(String("need IPv4 or IPv6 address"))))
    }
    return String(self.ip.String())
  }

  #: () -> Integer
  def ip_port = %x{
    if self.fam == syscall.AF_UNIX {
      panic(NewSocketError(Ref(String("need IPv4 or IPv6 address"))))
    }
    return Integer(self.port)
  }

  #: () -> [String, Integer]
  def ip_unpack = [ip_address, ip_port]

  #: () -> String
  def unix_path = %x{
    if self.fam != syscall.AF_UNIX {
      panic(NewSocketError(Ref(String("need AF_UNIX address"))))
    }
    return String(self.path)
  }

  # The address as MRI prints it: "ip", "ip:port", "[ip6]:port", or the Unix path.
  #: () -> String
  def inspect_sockaddr = %x{
    switch {
    case self.fam == syscall.AF_UNIX && self.path == "":
      return "empty-path-AF_UNIX-sockaddr"
    case self.fam == syscall.AF_UNIX:
      return String(self.path)
    case self.port == 0:
      return String(self.ip.String())
    case self.fam == syscall.AF_INET6:
      return String("[" + self.ip.String() + "]:" + strconv.Itoa(self.port))
    }
    return String(self.ip.String() + ":" + strconv.Itoa(self.port))
  }

  #: () -> String
  def inspect = %x{
    s := "#<Addrinfo: " + string(self.InspectSockaddr())
    switch {
    case self.fam == syscall.AF_UNIX && self.socktype == syscall.SOCK_DGRAM:
      s += " SOCK_DGRAM"
    case self.fam == syscall.AF_UNIX:
      s += " SOCK_STREAM"
    case self.socktype == syscall.SOCK_STREAM:
      s += " TCP"
    case self.socktype == syscall.SOCK_DGRAM:
      s += " UDP"
    case self.socktype == syscall.SOCK_RAW:
      s += " SOCK_RAW"
    }
    if self.name != "" {
      s += " (" + self.name + ")"
    }
    return String(s + ">")
  }

  # The packed struct sockaddr, as MRI's to_s/to_sockaddr.
  #: () -> String
  def to_sockaddr = %x{ String(rbSockaddrBytes(self)) }

  #: () -> String
  def to_s = to_sockaddr

  #: () -> [String, Integer, String, String]
  def __addr = %x{ rbSockIPAddr(self) }

  #: () -> String
  def __path = %x{ String(self.path) }
end
