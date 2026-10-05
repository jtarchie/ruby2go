# rbs_inline: enabled

require "minitest/autorun"
require "socket"
require "tmpdir"

# socket (decision 135): ports, fds and temp paths differ per run, so they are masked or only compared as booleans.
module SocketTests
  class SocketTest < Minitest::Test
    LINGER_RESET = "\x01\x00\x00\x00\x00\x00\x00\x00" #: String

    #: (String, Integer) -> String
    def mask(s, port) = s.gsub(port.to_s, "P")

    #: () -> [TCPServer, TCPSocket, TCPSocket]
    def tcp_trio
      server = TCPServer.new("127.0.0.1", 0)
      client = TCPSocket.new("127.0.0.1", server.addr[1])
      [server, client, server.accept]
    end

    # The class and message raised, for messages holding a port or path that assert_raises would log.
    #: () { () -> void } -> [String, String]
    def socket_raised
      yield
      ["none", ""]
    rescue => e
      [e.class.name.to_s, e.message]
    end

    #: (TCPServer) -> TCPSocket
    def socket_accept_retry(server)
      server.accept_nonblock
    rescue IO::WaitReadable
      Thread.pass
      retry
    end

    def test_tcp_round_trip
      server, client, peer = tcp_trio
      client.write("ab")
      client.print("c", "d")
      client << "e" << "f"
      client.puts "g"
      client.puts ["h", "i"]
      client.close_write
      assert_equal "abc", peer.read(3)
      assert_equal "defg\n", peer.gets
      assert_equal "h\ni\n", peer.read
      assert_equal "", peer.read
      assert_nil peer.read(1)
      assert_nil peer.gets
      assert peer.eof?
      assert_raises(EOFError) { peer.readpartial(1) }
      assert_raises(EOFError) { peer.readline }
      peer.close
      client.close
      server.close
    end

    def test_lines_and_partial_reads
      server, client, peer = tcp_trio
      client.write("one\ntwo\nthree")
      client.close_write
      assert_equal "one\n", peer.readline
      assert_equal "tw", peer.readpartial(2)
      lines = [] #: Array[String]
      peer.each_line { |l| lines << l }
      assert_equal ["o\n", "three"], lines
      assert_nil peer.recv(10)
      [server, client, peer].each(&:close)
    end

    def test_readlines_and_send
      server, client, peer = tcp_trio
      assert_equal 3, client.send("a\nb", 0)
      client.close_write
      assert_equal ["a\n", "b"], peer.readlines
      [server, client, peer].each(&:close)
    end

    def test_recv_flags_and_stream_recvfrom
      server, client, peer = tcp_trio
      client.write("hello")
      assert_equal "he", peer.recv(2, Socket::MSG_PEEK)
      assert_equal ["hel", nil], peer.recvfrom(3)
      client.close
      assert_equal "lo", peer.recv(10)
      assert_nil peer.recvfrom(3)
      peer.close
      server.close
    end

    def test_addresses
      server, client, peer = tcp_trio
      port = server.addr[1]
      assert port > 0
      assert_equal ["AF_INET", "127.0.0.1", "127.0.0.1"], [server.addr[0], server.addr[2], server.addr[3]]
      assert client.peeraddr[1] == port
      assert peer.addr[1] == port
      assert peer.peeraddr[1] == client.addr[1]
      assert_equal "#<Addrinfo: 127.0.0.1:P TCP>", mask(client.remote_address.inspect, port)
      assert_equal "127.0.0.1", server.local_address.ip_address
      assert server.local_address.ip_port == port
      assert_equal [true, false, false], [peer.local_address.ipv4?, peer.local_address.ipv6?, peer.local_address.unix?]
      assert /\A#<TCPSocket:fd \d+, AF_INET, 127\.0\.0\.1, \d+>\z/.match?(client.inspect)
      assert_equal "#<TCPServer:fd F, AF_INET, 127.0.0.1, P>", mask(server.inspect, port).sub(/fd \d+/, "fd F")
      assert client.fileno >= 0
      [server, client, peer].each(&:close)
      assert_equal "#<TCPSocket:(closed)>", client.inspect
    end

    def test_class_tree
      assert_equal [TCPServer, TCPSocket, IPSocket, BasicSocket], TCPServer.ancestors.take(4)
      assert_equal IPSocket, UDPSocket.superclass
      assert_equal BasicSocket, UNIXSocket.superclass
      assert_equal UNIXSocket, UNIXServer.superclass
      assert_equal BasicSocket, Socket.superclass
      assert_equal SocketError, Socket::ResolutionError.superclass
      assert_equal Errno::EAGAIN, IO::EAGAINWaitReadable.superclass
      assert_equal IOError, IO::TimeoutError.superclass
    end

    def test_close
      server, client, peer = tcp_trio
      refute client.closed?
      assert_nil client.close
      assert_nil client.close
      assert client.closed?
      e = assert_raises(IOError) { client.write("x") }
      assert_equal "closed stream", e.message
      e = assert_raises(IOError) { client.gets }
      assert_equal "closed stream", e.message
      peer.close_read
      refute peer.closed?
      e = assert_raises(IOError) { peer.gets }
      assert_equal "not opened for reading", e.message
      peer.close_write
      assert peer.closed?
      server.close
      e = assert_raises(IOError) { server.accept }
      assert_equal "closed stream", e.message
    end

    def test_accept_nonblock
      server = TCPServer.new("127.0.0.1", 0)
      e = assert_raises(IO::WaitReadable) { server.accept_nonblock }
      assert_equal IO::EAGAINWaitReadable, e.class
      assert_kind_of Errno::EAGAIN, e
      assert_equal "Resource temporarily unavailable - accept(2) would block", e.message
      client = TCPSocket.new("127.0.0.1", server.addr[1])
      peer = socket_accept_retry(server)
      client.puts "hi"
      assert_equal "hi\n", peer.gets
      [server, client, peer].each(&:close)
    end

    def test_connect_errors
      e = assert_raises(Errno::ECONNREFUSED) { TCPSocket.new("127.0.0.1", 1) }
      assert_equal "Connection refused - connect(2) for \"127.0.0.1\" port 1", e.message
      errno = begin
        TCPSocket.new("127.0.0.1", 1)
        nil
      rescue Errno::ECONNREFUSED => refused
        refused.errno
      end
      assert_kind_of Integer, errno
      e = assert_raises(Errno::ECONNREFUSED) { Socket.tcp("127.0.0.1", 1) }
      assert_equal "Connection refused - connect(2) for 127.0.0.1:1", e.message
      server = TCPServer.new("127.0.0.1", 0)
      port = server.addr[1]
      cls, msg = socket_raised { TCPServer.new("127.0.0.1", port) }
      assert_equal ["Errno::EADDRINUSE", "Address already in use - bind(2) for \"127.0.0.1\" port P"], [cls, mask(msg, port)]
      server.close
      e = assert_raises(SocketError) { TCPSocket.new("nonexistent.invalid", 80) }
      assert_equal Socket::ResolutionError, e.class
      assert e.message.start_with?("getaddrinfo(3): ")
      e = assert_raises(Socket::ResolutionError) { Addrinfo.tcp("nonexistent.invalid", 80) }
      assert e.message.start_with?("getaddrinfo: ")
    end

    def test_peer_errors
      server, client, peer = tcp_trio
      client.write("abc")
      peer.setsockopt(Socket::SOL_SOCKET, Socket::SO_LINGER, LINGER_RESET)
      peer.close
      e = assert_raises(Errno::ECONNRESET) do
        loop { client.readpartial(10) }
      end
      assert_equal "Connection reset by peer", e.message
      client.close
      client = TCPSocket.new("127.0.0.1", server.addr[1])
      server.accept.close
      raised = socket_raised { loop { client.write("x" * 65536) } }
      assert ["Errno::EPIPE Broken pipe", "Errno::ECONNRESET Connection reset by peer"].include?("#{raised[0]} #{raised[1]}") # which one is a race
      client.close
      server.close
    end

    def test_options_and_timeouts
      server = TCPServer.new("127.0.0.1", 0)
      assert_equal 0, server.setsockopt(Socket::SOL_SOCKET, Socket::SO_REUSEADDR, true)
      assert_equal 0, server.listen(5)
      client = TCPSocket.new("127.0.0.1", server.addr[1], connect_timeout: 2.0)
      assert_equal 0, client.setsockopt(Socket::IPPROTO_TCP, Socket::TCP_NODELAY, 1)
      assert client.sync
      client.sync = true
      peer = server.accept
      client.puts "ok"
      assert_equal "ok\n", peer.gets
      [server, client, peer].each(&:close)
      assert_equal TCPServer, TCPServer.open("127.0.0.1", 0).class
      assert_equal "AF_INET6", TCPServer.new(0).addr[0]
    end

    def test_socket_tcp
      server = TCPServer.new("127.0.0.1", 0)
      got = Socket.tcp("127.0.0.1", server.addr[1]) do |s|
        assert_equal Socket, s.class
        s.puts "via Socket.tcp"
        assert_equal "127.0.0.1", s.remote_address.ip_address
        7
      end
      assert_equal 7, got
      assert_equal "via Socket.tcp\n", server.accept.gets
      s = Socket.tcp("127.0.0.1", server.addr[1])
      refute s.closed?
      s.close
      server.close
    end

    def test_udp
      u = UDPSocket.new
      assert_equal ["AF_INET", 0, "0.0.0.0", "0.0.0.0"], u.addr
      assert_equal 0, u.bind("127.0.0.1", 0)
      port = u.addr[1]
      assert_equal 4, u.send("ping", 0, "127.0.0.1", port)
      msg, from = u.recvfrom(16)
      assert_equal "ping", msg
      assert_equal ["AF_INET", "127.0.0.1", "127.0.0.1"], [from[0], from[2], from[3]]
      assert from[1] == port
      v = UDPSocket.new
      e = assert_raises(Errno::EDESTADDRREQ) { v.send("x", 0) }
      assert_equal "Destination address required - send(2)", e.message
      assert_equal 0, v.connect("127.0.0.1", port)
      assert_equal 5, v.send("hello", 0)
      assert_equal "hel", u.recv(3)
      cls, msg = socket_raised { v.send("x", 0, "127.0.0.1", port) }
      assert_equal ["Errno::EISCONN", "Socket is already connected - sendto(2) for \"127.0.0.1\" port P"], [cls, mask(msg, port)]
      assert v.peeraddr[1] == port
      assert /\A#<UDPSocket:fd \d+, AF_INET, 127\.0\.0\.1, \d+>\z/.match?(u.inspect)
      u.close
      v.close
      assert u.closed?
      assert_equal "#<UDPSocket:(closed)>", u.inspect
      assert_equal "AF_INET6", UDPSocket.new(Socket::AF_INET6).addr[0]
      w = UDPSocket.new
      assert_equal 0, w.bind("", 0)
      assert_equal "0.0.0.0", w.addr[2]
      e = assert_raises(Socket::ResolutionError) { w.connect("nonexistent.invalid", 1) }
      assert e.message.start_with?("getaddrinfo: ")
      w.close
    end

    def test_unix
      Dir.mktmpdir do |dir|
        path = File.join(dir, "s.sock")
        server = UNIXServer.new(path)
        assert server.path == path
        assert server.addr == ["AF_UNIX", path]
        assert server.inspect == "#<UNIXServer:#{path}>"
        client = UNIXSocket.new(path)
        peer = server.accept
        assert_equal UNIXSocket, peer.class
        assert /\A#<UNIXSocket:fd \d+>\z/.match?(client.inspect)
        assert peer.path == path
        assert_equal "", client.path
        assert_equal ["AF_UNIX", ""], client.addr
        assert client.peeraddr == ["AF_UNIX", path]
        assert peer.addr == ["AF_UNIX", path]
        assert_equal ["AF_UNIX", ""], peer.peeraddr
        assert_equal "#<UNIXSocket:>", client.inspect
        assert_equal "#<Addrinfo: empty-path-AF_UNIX-sockaddr SOCK_STREAM>", client.local_address.inspect
        assert client.remote_address.unix_path == path
        client.puts "hey"
        assert_equal "hey\n", peer.gets
        [peer, client, server].each(&:close)
        assert File.exist?(path)
        cls, msg = socket_raised { UNIXServer.new(path) }
        assert_equal ["Errno::EADDRINUSE", "Address already in use - connect(2) for D/s.sock"], [cls, msg.sub(dir, "D")]
        File.delete(path)
        cls, msg = socket_raised { UNIXSocket.new(path) }
        assert_equal ["Errno::ENOENT", "No such file or directory - connect(2) for D/s.sock"], [cls, msg.sub(dir, "D")]
      end
    end

    def test_unix_pair
      a, b = UNIXSocket.pair
      assert_equal [UNIXSocket, UNIXSocket], [a.class, b.class]
      assert_equal ["AF_UNIX", ""], a.addr
      assert_equal ["AF_UNIX", ""], a.peeraddr
      assert /\A#<UNIXSocket:fd \d+>\z/.match?(a.inspect)
      a.write("pair")
      a.close_write
      assert_equal "pair", b.read
      a.close
      b.close
      c, d = UNIXSocket.pair(:DGRAM)
      c.send("one", 0)
      c.send("two", 0)
      assert_equal "one", d.recv(10)
      assert_equal "two", d.recv(10)
      assert_equal "#<Addrinfo: empty-path-AF_UNIX-sockaddr SOCK_DGRAM>", d.local_address.inspect
      c.close
      d.close
      e, f = UNIXSocket.socketpair
      e.puts "sp"
      assert_equal "sp\n", f.gets
      e.close
      f.close
    end

    def test_short_send_and_negative_lengths
      a, b = UNIXSocket.pair
      n = a.send("x" * 4_000_000, 0)
      a.close
      assert n > 0
      assert_equal n, b.read.bytesize
      e = assert_raises(ArgumentError) { b.readpartial(-1) }
      assert_equal "negative length -1 given", e.message
      e = assert_raises(ArgumentError) { b.recv(-1) }
      assert_equal "negative string size (or size too big)", e.message
      b.close
    end

    def test_addrinfo
      ai = Addrinfo.tcp("127.0.0.1", 80)
      assert_equal "#<Addrinfo: 127.0.0.1:80 TCP>", ai.inspect
      assert_equal ["127.0.0.1", 80, true, false, false, true], [ai.ip_address, ai.ip_port, ai.ipv4?, ai.ipv6?, ai.unix?, ai.ip?]
      assert_equal [Socket::AF_INET, Socket::PF_INET, Socket::SOCK_STREAM, Socket::IPPROTO_TCP], [ai.afamily, ai.pfamily, ai.socktype, ai.protocol]
      assert_equal ["127.0.0.1", 80], ai.ip_unpack
      assert_equal "127.0.0.1:80", ai.inspect_sockaddr
      assert ai.to_sockaddr == ai.to_s # binary Strings inspect differently: rb2go has no encodings
      assert_equal [16, 0, 80, 127, 0, 0, 1], [ai.to_s.bytesize] + ai.to_s.bytes.drop(2).take(6)
      assert ai.ipv4_loopback?
      u = Addrinfo.udp("::1", 53)
      assert_equal "#<Addrinfo: [::1]:53 UDP>", u.inspect
      assert_equal [true, true, 28], [u.ipv6?, u.ipv6_loopback?, u.to_s.bytesize]
      assert_equal "#<Addrinfo: 127.0.0.1 TCP>", Addrinfo.tcp("127.0.0.1", 0).inspect
      assert_equal "#<Addrinfo: 127.0.0.1:80 TCP (:http)>", Addrinfo.tcp("127.0.0.1", "http").inspect
      assert_equal "#<Addrinfo: 127.0.0.1:8080 TCP>", Addrinfo.tcp("127.0.0.1", "8080").inspect
      ip = Addrinfo.ip("127.0.0.1")
      assert_equal ["#<Addrinfo: 127.0.0.1>", 0, 0], [ip.inspect, ip.socktype, ip.ip_port]
      assert_equal "#<Addrinfo: ::1>", Addrinfo.ip("::1").inspect
      assert_equal "#<Addrinfo: [::ffff:127.0.0.1]:80 TCP>", Addrinfo.tcp("::ffff:127.0.0.1", 80).inspect
      assert_equal "#<Addrinfo: 255.255.255.255:80 TCP (<broadcast>)>", Addrinfo.tcp("<broadcast>", 80).inspect
      assert_equal "#<Addrinfo: 0.0.0.0:80 TCP (<any>)>", Addrinfo.tcp("<any>", 80).inspect
      un = Addrinfo.unix("/tmp/x")
      assert_equal ["#<Addrinfo: /tmp/x SOCK_STREAM>", "/tmp/x", true, false], [un.inspect, un.unix_path, un.unix?, un.ip?]
      assert_equal Socket::AF_UNIX, un.afamily
      assert_equal "#<Addrinfo: /tmp/x SOCK_DGRAM>", Addrinfo.unix("/tmp/x", :DGRAM).inspect
      assert_equal [47, 116, 109, 112, 47, 120, 0], un.to_s.bytes.drop(2).take(7)
      e = assert_raises(SocketError) { un.ip_address }
      assert_equal "need IPv4 or IPv6 address", e.message
      e = assert_raises(SocketError) { un.ip_port }
      assert_equal "need IPv4 or IPv6 address", e.message
      e = assert_raises(SocketError) { ai.unix_path }
      assert_equal "need AF_UNIX address", e.message
      assert_equal ["#<Addrinfo: 127.0.0.1:80 TCP>", "#<Addrinfo: 127.0.0.1:80 UDP>", "#<Addrinfo: 127.0.0.1:80 SOCK_RAW>"], Addrinfo.getaddrinfo("127.0.0.1", 80).map(&:inspect)
      assert_equal ["#<Addrinfo: 127.0.0.1:80 UDP>"], Addrinfo.getaddrinfo("127.0.0.1", 80, :INET, :DGRAM).map(&:inspect)
      assert_equal [], Addrinfo.getaddrinfo("localhost", 80, nil, :STREAM).map(&:ip_address).reject { |a| ["127.0.0.1", "::1"].include?(a) }
      assert Addrinfo.getaddrinfo("localhost", 80, nil, :STREAM).all? { |a| a.inspect.end_with?(" TCP (localhost)>") }
    end

    def test_socket_class_methods
      assert_equal [["AF_INET", 80, "127.0.0.1", "127.0.0.1", Socket::AF_INET, Socket::SOCK_STREAM, Socket::IPPROTO_TCP]],
        Socket.getaddrinfo("127.0.0.1", 80, Socket::AF_INET, Socket::SOCK_STREAM)
      assert_equal 3, Socket.getaddrinfo("127.0.0.1", nil).size
      assert_equal [["AF_INET6", 22, "::1", "::1", Socket::AF_INET6, Socket::SOCK_DGRAM, Socket::IPPROTO_UDP]],
        Socket.getaddrinfo("::1", 22, :INET6, :DGRAM)
      assert_equal [80], Socket.getaddrinfo("127.0.0.1", "http", nil, :STREAM).map { |r| r[1] }
      e = assert_raises(Socket::ResolutionError) { Socket.getaddrinfo("nonexistent.invalid", 80) }
      assert e.message.start_with?("getaddrinfo: ")
      name = Socket.gethostname
      assert_kind_of String, name
      refute name.empty?
      list = Socket.ip_address_list
      assert list.all? { |a| a.is_a?(Addrinfo) }
      assert list.map(&:ip_address).include?("127.0.0.1")
      assert_equal "127.0.0.1", IPSocket.getaddress("127.0.0.1")
    end

    def test_constants
      assert_equal [2, 1, 1, 2, 0], [Socket::AF_INET, Socket::AF_UNIX, Socket::SOCK_STREAM, Socket::SOCK_DGRAM, Socket::AF_UNSPEC]
      assert_equal [6, 17, 1, 0], [Socket::IPPROTO_TCP, Socket::IPPROTO_UDP, Socket::TCP_NODELAY, Socket::INADDR_ANY]
      assert_equal [Socket::AF_INET6, Socket::SOL_SOCKET, Socket::SO_REUSEADDR], [Socket::PF_INET6, Socket::SOL_SOCKET, Socket::SO_REUSEADDR]
      assert_equal [0, 1, 2], [Socket::SHUT_RD, Socket::SHUT_WR, Socket::SHUT_RDWR]
    end
  end
end
