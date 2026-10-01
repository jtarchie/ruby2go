# rbs_inline: enabled

require "minitest/autorun"
require "pp"
require "set"

# pp (decision 113): each layout is MRI's own PP.pp output at that width.
module PpTests
  Pair = Struct.new(:left, :right) #: [untyped, untyped]

  class PpTest < Minitest::Test
    def test_layout_0
      assert_equal "[1, 2, 3]\n", PP.pp([1, 2, 3], "".dup, 10)
      assert_equal "[1,\n 2,\n 3]\n", PP.pp([1, 2, 3], "".dup, 4)
    end

    def test_layout_1
      assert_equal "{a: 1,\n \"b\" =>\n  [1,\n   2,\n   3],\n c:\n  {d: :e}}\n", PP.pp({ a: 1, "b" => [1, 2, 3], c: { d: :e } }, "".dup, 10)
      assert_equal "{a: 1,\n \"b\" => [1, 2, 3],\n c: {d: :e}}\n", PP.pp({ a: 1, "b" => [1, 2, 3], c: { d: :e } }, "".dup, 20)
      assert_equal "{a: 1, \"b\" => [1, 2, 3], c: {d: :e}}\n", PP.pp({ a: 1, "b" => [1, 2, 3], c: { d: :e } }, "".dup, 40)
    end

    def test_layout_2
      assert_equal "[1,\n 2,\n 3,\n 4,\n 5,\n 6,\n 7,\n 8,\n 9,\n 10,\n 11,\n 12,\n 13,\n 14,\n 15,\n 16,\n 17,\n 18,\n 19,\n 20]\n", PP.pp((1..20).to_a, "".dup, 20)
      assert_equal "[1,\n 2,\n 3,\n 4,\n 5,\n 6,\n 7,\n 8,\n 9,\n 10,\n 11,\n 12,\n 13,\n 14,\n 15,\n 16,\n 17,\n 18,\n 19,\n 20]\n", PP.pp((1..20).to_a, "".dup, 30)
    end

    def test_layout_3
      assert_equal "{\"+\": 1, \"odd key\": 2, a?: 3, b!: 4, \"@iv\": 5, \"$g\": 6, \"==\": 7}\n", PP.pp({ "+": 1, "odd key": 2, :"a?" => 3, :b! => 4, :@iv => 5, :"$g" => 6, :== => 7 }, "".dup, 80)
      assert_equal "{\"+\": 1,\n \"odd key\": 2,\n a?: 3,\n b!: 4,\n \"@iv\": 5,\n \"$g\": 6,\n \"==\": 7}\n", PP.pp({ "+": 1, "odd key": 2, :"a?" => 3, :b! => 4, :@iv => 5, :"$g" => 6, :== => 7 }, "".dup, 15)
    end

    def test_layout_4
      assert_equal "[\"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\",\n [\"yyyyyyyyyyyyyyyyyyyyyyyyyyyyyy\",\n  [\"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzz\"]]]\n", PP.pp(["x" * 30, ["y" * 30, ["z" * 30]]], "".dup, 40)
      assert_equal "[\"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\",\n [\"yyyyyyyyyyyyyyyyyyyyyyyyyyyyyy\", [\"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzz\"]]]\n", PP.pp(["x" * 30, ["y" * 30, ["z" * 30]]], "".dup, 79)
    end

    def test_layout_5
      assert_equal "\"one\\n\" +\n\"two\\n\" +\n\"three\"\n", PP.pp("one\ntwo\nthree", "".dup, 10)
      assert_equal "\"one\\n\" + \"two\\n\" + \"three\"\n", PP.pp("one\ntwo\nthree", "".dup, 79)
    end

    def test_layout_6
      assert_equal "\"no newline at all\"\n", PP.pp("no newline at all", "".dup, 5)
    end

    def test_layout_7
      assert_equal "Set[1,\n 2,\n 3]\n", PP.pp(Set[1, 2, 3], "".dup, 5)
      assert_equal "Set[1, 2, 3]\n", PP.pp(Set[1, 2, 3], "".dup, 79)
    end

    def test_layout_8
      assert_equal "1\n..\n5\n", PP.pp((1..5), "".dup, 3)
    end

    def test_layout_9
      assert_equal "1...\n", PP.pp((1...), "".dup, 79)
    end

    def test_layout_10
      assert_equal "[nil,\n true,\n false,\n 1.5,\n :sym,\n -3]\n", PP.pp([nil, true, false, 1.5, :sym, -3], "".dup, 10)
    end

    def test_layout_11
      assert_equal "{list:\n  [1,\n   2,\n   3,\n   4,\n   5,\n   6,\n   7,\n   8,\n   9,\n   10,\n   11,\n   12,\n   13,\n   14,\n   15,\n   16,\n   17,\n   18,\n   19,\n   20,\n   21,\n   22,\n   23,\n   24,\n   25,\n   26,\n   27,\n   28,\n   29,\n   30],\n words:\n  [\"alpha\",\n   \"beta\",\n   \"gamma\",\n   \"delta\",\n   \"epsilon\",\n   \"zeta\",\n   \"eta\",\n   \"theta\",\n   \"iota\",\n   \"kappa\"]}\n", PP.pp({ list: (1..30).to_a, words: %w[alpha beta gamma delta epsilon zeta eta theta iota kappa] }, "".dup, 40)
      assert_equal "{list:\n  [1,\n   2,\n   3,\n   4,\n   5,\n   6,\n   7,\n   8,\n   9,\n   10,\n   11,\n   12,\n   13,\n   14,\n   15,\n   16,\n   17,\n   18,\n   19,\n   20,\n   21,\n   22,\n   23,\n   24,\n   25,\n   26,\n   27,\n   28,\n   29,\n   30],\n words:\n  [\"alpha\",\n   \"beta\",\n   \"gamma\",\n   \"delta\",\n   \"epsilon\",\n   \"zeta\",\n   \"eta\",\n   \"theta\",\n   \"iota\",\n   \"kappa\"]}\n", PP.pp({ list: (1..30).to_a, words: %w[alpha beta gamma delta epsilon zeta eta theta iota kappa] }, "".dup, 79)
    end

    def test_layout_12
      assert_equal "[[],\n [],\n []]\n", PP.pp([[], [], []], "".dup, 4)
    end

    def test_layout_13
      assert_equal "{}\n", PP.pp({}, "".dup, 1)
    end

    def test_struct_and_cycles
      assert_equal "#<struct PpTests::Pair left=1, right=2>\n", PP.pp(Pair.new(1, 2), "".dup, 79)
      assert_equal "#<struct PpTests::Pair\n left=\"aaaaaaaaaa\",\n right=\"bbbbbbbbbb\">\n", PP.pp(Pair.new("a" * 10, "b" * 10), "".dup, 30)
      a = [1] #: Array[untyped]
      a << a
      assert_equal "[1, [...]]\n", PP.pp(a, "".dup, 79)
      h = { k: 1 } #: Hash[Symbol, untyped]
      h[:me] = h
      assert_equal "{k: 1, me: {...}}\n", PP.pp(h, "".dup, 79)
      list = [1, [2, [3]]] #: Array[untyped]
      assert_equal "[1, [2, [3]]]\n", list.pretty_inspect
    end
  end
end
