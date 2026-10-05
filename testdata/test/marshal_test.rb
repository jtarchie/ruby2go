# rbs_inline: enabled

require "minitest/autorun"
require "set"
require "date"
require "bigdecimal"
require "stringio"
require "tmpdir"
require "ostruct"
require "json"
require "socket"

# Marshal (decision 137): the bytes are rb2go's own, so no assertion ever sees them; only the object graphs are MRI's.
module MarshalTests
  Point = Struct.new(:x, :y) #: [Integer, Integer]
  Pair = Data.define(:left, :right) #: [Integer, String]

  class Node
    attr_reader :name #: String
    attr_reader :children #: Array[Node]
    attr_accessor :parent #: Node?

    #: (String) -> void
    def initialize(name)
      @name = name
      @children = [] #: Array[Node]
      @parent = nil
    end

    #: (Node) -> Node
    def add(child)
      child.parent = self
      @children << child
      child
    end
  end

  class Account
    attr_reader :owner #: String
    attr_reader :balance #: Float
    attr_reader :tags #: Set[Symbol]
    attr_reader :note #: String?

    #: (String, Float, Set[Symbol], String?) -> void
    def initialize(owner, balance, tags, note)
      @owner = owner
      @balance = balance
      @tags = tags
      @note = note
    end
  end

  # marshal_dump/marshal_load replace the ivars (private, as MRI calls them regardless): the area is recomputed
  class Circle
    attr_reader :radius #: Integer
    attr_reader :area #: Integer

    #: (Integer) -> void
    def initialize(radius)
      @radius = radius
      @area = radius * radius * 3
    end

    private

    #: () -> Array[Integer]
    def marshal_dump = [@radius]

    #: (Array[Integer]) -> void
    def marshal_load(data)
      @radius = data.sum
      @area = @radius * @radius * 3
    end
  end

  class DumpOnly
    #: () -> Integer
    def marshal_dump = 1
  end

  class MarshalTest < Minitest::Test
    #: (untyped) -> untyped
    def round(x) = Marshal.load(Marshal.dump(x))

    def test_issue_example
      data = {name: "a", pts: [Point.new(1, 2), Point.new(3, 4)], when: Time.at(0).utc}
      copy = round(data)
      assert_equal true, copy == data
      copy[:pts][0].x = 99
      assert_equal 1, data[:pts][0].x
      assert_equal 99, copy[:pts][0].x
      assert_equal false, copy == data
      assert_equal "1970-01-01 00:00:00 UTC", copy[:when].to_s
    end

    def test_core_values
      assert_nil round(nil)
      assert_equal [true, false], round([true, false])
      assert_equal [0, -1, 4611686018427387904, -9223372036854775808], round([0, -1, 4611686018427387904, -9223372036854775808])
      assert_equal [1.5, -0.25, Float::INFINITY], round([1.5, -0.25, Float::INFINITY])
      assert_equal true, round(Float::NAN).nan?
      assert_equal "-0.0", round(-0.0).to_s
      assert_equal ["", "héllo", "a\nb"], round(["", "héllo", "a\nb"])
      assert_equal :sym, round(:sym)
      assert_equal [1, [2, ["three", nil]], :four], round([1, [2, ["three", nil]], :four])
      assert_equal({"a" => 1, b: [2], 3 => nil}, round({"a" => 1, b: [2], 3 => nil}))
      assert_equal [1, nil, 3], round([1, nil, 3])
      assert_equal Marshal::MAJOR_VERSION, 4
      assert_equal Marshal::MINOR_VERSION, 8
      assert_equal true, Marshal.dump(1).is_a?(String)
    end

    def test_ranges_sets_and_classes
      assert_equal 1..3, round(1..3)
      assert_equal "a"..."c", round("a"..."c")
      assert_equal Set[1, 2, 3], round(Set[1, 2, 3])
      assert_equal String, round(String)
      assert_equal Node, round(Node)
      assert_equal [Integer, Point], round([Integer, Point])
      assert_equal(/a+b/i, round(/a+b/i))
    end

    def test_numbers
      assert_equal Rational(1, 3), round(Rational(1, 3))
      assert_equal Complex(1, 2), round(Complex(1, 2))
      assert_equal Complex(Rational(1, 2), 1.5), round(Complex(Rational(1, 2), 1.5))
      assert_equal BigDecimal("123.456"), round(BigDecimal("123.456"))
      assert_equal "-0.1e-9", round(BigDecimal("-1e-10")).to_s
      assert_equal true, round(BigDecimal("NaN")).nan?
    end

    def test_times_and_dates
      t = Time.at(1_700_000_000).utc
      assert_equal t, round(t)
      assert_equal true, round(t).utc?
      local = Time.at(1_700_000_000)
      assert_equal local.to_s, round(local).to_s
      assert_equal false, round(local).utc?
      assert_equal Date.new(2024, 2, 29), round(Date.new(2024, 2, 29))
      dt = DateTime.new(2024, 2, 29, 13, 45, 30, "+09:00")
      assert_equal dt.to_s, round(dt).to_s
      assert_equal true, round(dt).instance_of?(DateTime)
    end

    def test_structs_data_and_objects
      p2 = round(Point.new(5, 6))
      assert_equal Point.new(5, 6), p2
      assert_equal Pair.new(left: 1, right: "r"), round(Pair.new(left: 1, right: "r"))
      acct = Account.new("ann", 10.5, Set[:a, :b], nil)
      copy = round(acct)
      assert_equal "ann", copy.owner
      assert_equal 10.5, copy.balance
      assert_equal Set[:a, :b], copy.tags
      assert_nil copy.note
      assert_equal true, copy.is_a?(Account)
      assert_equal false, copy.equal?(acct)
      assert_equal %i[@owner @balance @tags @note], copy.instance_variables
    end

    def test_copy_is_independent
      src = {list: [1, 2], inner: {k: "v"}}
      copy = round(src)
      copy[:list] << 3
      copy[:inner][:k] = "w"
      assert_equal({list: [1, 2], inner: {k: "v"}}, src)
      assert_equal({list: [1, 2, 3], inner: {k: "w"}}, copy)
      acct = Account.new("bo", 1.0, Set[:x], "n")
      c2 = round(acct)
      c2.tags << :y
      assert_equal Set[:x], acct.tags
    end

    def test_shared_references_and_cycles
      a = [] #: Array[untyped]
      a << a
      b = round(a)
      assert_equal true, b[0].equal?(b)
      shared = [1, 2]
      pair = round([shared, shared])
      assert_equal true, pair[0].equal?(pair[1])
      pair[0] << 3
      assert_equal [1, 2, 3], pair[1]
      h = {} #: Hash[Symbol, untyped]
      h[:self] = h
      h2 = round(h)
      assert_equal true, h2[:self].equal?(h2)
      root = Node.new("root")
      kid = root.add(Node.new("kid"))
      kid.add(Node.new("leaf"))
      r2 = round(root)
      k2 = r2.children[0]
      assert_equal "kid", k2.name
      assert_equal true, k2.parent.equal?(r2)
      assert_equal "leaf", k2.children[0].name
      assert_equal true, k2.children[0].parent.equal?(k2)
      pt = Point.new(1, 1)
      both = round({a: pt, b: pt})
      assert_equal true, both[:a].equal?(both[:b])
    end

    def test_hooks
      c = round(Circle.new(4))
      assert_equal 4, c.radius
      assert_equal 48, c.area
      many = round([Circle.new(1), Circle.new(2)])
      assert_equal [3, 12], [many[0].area, many[1].area]
      bytes = Marshal.dump(DumpOnly.new)
      e = assert_raises(TypeError) { Marshal.load(bytes) }
      assert_equal "instance of MarshalTests::DumpOnly needs to have method 'marshal_load'", e.message
    end

    def test_io_forms
      io = StringIO.new
      assert_equal true, Marshal.dump([1, 2], io).equal?(io)
      Marshal.dump({a: 1}, io)
      io.rewind
      assert_equal [1, 2], Marshal.load(io)
      assert_equal({a: 1}, Marshal.load(io))
      e = assert_raises(EOFError) { Marshal.load(io) }
      assert_equal "end of file reached", e.message
      Dir.mktmpdir do |dir|
        path = File.join(dir, "cache.bin")
        File.binwrite(path, Marshal.dump({name: "a", pts: [Point.new(1, 2)]}))
        back = Marshal.load(File.binread(path))
        assert_equal "a", back[:name]
        assert_equal [Point.new(1, 2)], back[:pts]
        File.open(path, "wb") do |f|
          Marshal.dump("first", f)
          Marshal.dump([:second], f)
        end
        got = [] #: Array[untyped]
        File.open(path, "rb") do |f|
          begin
            loop { got << Marshal.load(f) }
          rescue EOFError
            got << :eof
          end
          f.close
        end
        assert_equal ["first", [:second], :eof], got
      end
      a, b = UNIXSocket.pair
      Marshal.dump({x: [1, 2]}, a)
      Marshal.dump("second", a)
      assert_equal [{x: [1, 2]}, "second"], [Marshal.load(b), Marshal.load(b)]
      a.close
      assert_raises(EOFError) { Marshal.load(b) }
      b.close
      assert_equal [1], Marshal.restore(Marshal.dump([1]))
      [5, nil].each do |bad|
        e = assert_raises(TypeError) { Marshal.load(bad) }
        assert_equal "instance of IO needed", e.message
      end
      e = assert_raises(TypeError) { Marshal.dump(1, "x") }
      assert_equal "instance of IO needed", e.message
    end

    def test_library_values
      e = round(ArgumentError.new("boom"))
      assert_equal "boom", e.message
      assert_equal true, e.instance_of?(ArgumentError)
      o = round(OpenStruct.new(a: 1, b: "x"))
      assert_equal [1, "x"], [o.a, o.b]
      r = Random.new(3)
      r.rand(10)
      r2 = round(r)
      assert_equal r.rand(1000), r2.rand(1000)
      assert_equal({"a" => [1, {"b" => nil}]}, round(JSON.parse('{"a": [1, {"b": null}]}')))
      assert_equal Comparable, round(Comparable)
      assert_equal ["1..", 1], [round(1..).inspect, round(1..).begin]
      words = %w[a b a]
      assert_equal({"a" => 2, "b" => 1}, round(words.tally))
      assert_equal [%w[a b], ["a"]], round(words.each_slice(2).to_a)
    end

    def test_depth_limit
      e = assert_raises(ArgumentError) { Marshal.dump([[1]], 1) }
      assert_equal "exceed depth limit", e.message
      assert_equal [[1]], Marshal.load(Marshal.dump([[1]], 3))
    end

    def test_undumpable
      e = assert_raises(TypeError) { Marshal.dump(proc { 1 }) }
      assert_equal "no _dump_data is defined for class Proc", e.message
      e = assert_raises(TypeError) { Marshal.dump([1, $stdout]) }
      assert_equal "can't dump IO", e.message
      e = assert_raises(TypeError) { Marshal.dump(Mutex.new) }
      assert_equal "no _dump_data is defined for class Thread::Mutex", e.message
      e = assert_raises(TypeError) { Marshal.dump(Thread.current) }
      assert_equal "no _dump_data is defined for class Thread", e.message
      q = Queue.new #: Queue[Integer]
      e = assert_raises(TypeError) { Marshal.dump(q) }
      assert_equal "can't dump Thread::Queue", e.message
      e = assert_raises(TypeError) { Marshal.dump(StringIO.new) }
      assert_equal "no _dump_data is defined for class StringIO", e.message
      Dir.mktmpdir do |dir|
        File.open(File.join(dir, "f"), "w") do |f|
          e = assert_raises(TypeError) { Marshal.dump(f) }
          assert_equal "can't dump File", e.message
        end
      end
    end

    def test_bad_input
      e = assert_raises(ArgumentError) { Marshal.load("") }
      assert_equal "marshal data too short", e.message
      e = assert_raises(ArgumentError) { Marshal.load(Marshal.dump("abcdef")[0, 12]) }
      assert_equal "marshal data too short", e.message
      bad = begin
        Marshal.load("not marshal data")
        nil
      rescue TypeError => err
        err.class
      end
      assert_equal TypeError, bad
    end
  end
end
