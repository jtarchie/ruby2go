# rbs_inline: enabled

require "minitest/autorun"
require "yaml"
require "stringio"
require "tmpdir"

# YAML over go-yaml (decision 376): loads follow Psych's rules; a dump is checked by loading it back, since its text is rugaroo's emitter, not libyaml's.
module YamlTests
  #: () { () -> untyped } -> Class?
  def self.raised
    yield
    nil
  rescue Exception => e # rubocop:disable Lint/RescueException
    e.class
  end

  class ScalarTest < Minitest::Test
    def test_yaml_1_1_scalars
      h = YAML.load("a: yes\nb: Off\nc: ~\nd: Null\ne:\nf: 'yes'\ng: \"1\"\nh: y") #: Hash[String, untyped]
      assert_equal({ "a" => true, "b" => false, "c" => nil, "d" => nil, "e" => nil, "f" => "yes", "g" => "1", "h" => "y" }, h)
    end

    def test_numbers
      h = YAML.load("a: 017\nb: 0x_ff\nc: 0b11\nd: 1_000\ne: 1,000\nf: 08\ng: 0o17\nh: +12\ni: -0x1A") #: Hash[String, untyped]
      assert_equal({ "a" => 15, "b" => 255, "c" => 3, "d" => 1000, "e" => 1000, "f" => "08", "g" => "0o17", "h" => 12, "i" => -26 }, h)
      f = YAML.load("a: .5\nb: 1.\nc: 1_0.5\nd: 12e3\ne: 1.2e+3\nf: -.inf\ng: +.5") #: Hash[String, untyped]
      assert_equal({ "a" => 0.5, "b" => 1.0, "c" => 10.5, "d" => "12e3", "e" => 1200.0, "f" => -Float::INFINITY, "g" => 0.5 }, f)
    end

    def test_sexagesimal
      assert_equal [5400, 45045, "1.5:30"], YAML.load("- 1:30\n- 12:30:45\n- 1.5:30")
    end

    def test_tags
      h = YAML.load("a: !!str 12\nb: !!int \"12\"\nc: !!float 1\nd: !!binary aGVsbG8=\ne: !local bar\nf: !!null ''\ng: !!int abc\nh: !binary aGk=\ni: !ruby/string 12\nj: !local \"1\"") #: Hash[String, untyped]
      assert_equal({ "a" => "12", "b" => 12, "c" => 1.0, "d" => "hello", "e" => "bar", "f" => nil, "g" => "abc", "h" => "hi", "i" => "12", "j" => 1 }, h)
    end

    def test_block_scalars
      h = YAML.load("lit: |\n  one\n  two\nfold: >\n  three\n  four\nkeep: |-\n  five\nmulti: plain\n  text\nesc: \"tab\\there\"") #: Hash[String, untyped]
      assert_equal({ "lit" => "one\ntwo\n", "fold" => "three four\n", "keep" => "five", "multi" => "plain text", "esc" => "tab\there" }, h)
    end

    def test_symbols
      assert_equal({ a: :b, "c" => :"d e" }, YAML.load(":a: :b\nc: :\"d e\""))
      assert_equal Psych::DisallowedClass, YamlTests.raised { YAML.safe_load(":a: 1") }
      assert_equal({ a: 1 }, YAML.safe_load(":a: 1", permitted_classes: [Symbol], permitted_symbols: [:a]))
      assert_equal Psych::DisallowedClass, YamlTests.raised { YAML.load(":b: 1", permitted_symbols: [:a]) }
    end

    def test_dates_and_times
      assert_equal Psych::DisallowedClass, YamlTests.raised { YAML.load("d: 2001-12-14") }
      d = YAML.load("d: 2001-12-14", permitted_classes: [Date]) #: Hash[String, untyped]
      assert_equal "2001-12-14", d["d"].to_s
      assert_equal({ "x" => "2001-02-30" }, YAML.unsafe_load("x: 2001-02-30"))
      t = YAML.unsafe_load("a: 2001-12-14t21:59:43.10Z\nb: 2001-12-14 21:59:43.10 -5") #: Hash[String, untyped]
      assert_equal [true, 1008367183, 100000], [t["a"].utc?, t["a"].to_i, t["a"].usec]
      assert_equal [-18000, 1008385183], [t["b"].utc_offset, t["b"].to_i]
    end
  end

  class StructureTest < Minitest::Test
    def test_collections_keep_order
      h = YAML.load("z: 1\na:\n  - b: 1\n    c: [x, 'y', \"z\"]\n  - d\nm: {k: v}") #: Hash[String, untyped]
      assert_equal ["z", "a", "m"], h.keys
      assert_equal({ "z" => 1, "a" => [{ "b" => 1, "c" => ["x", "y", "z"] }, "d"], "m" => { "k" => "v" } }, h)
    end

    def test_mixed_keys
      assert_equal({ 1 => "a", "b" => "c", nil => 1, true => 2 }, YAML.load("1: a\nb: c\n~: 1\nyes: 2"))
      assert_equal({ 1 => "a", b: "c" }, YAML.load("1: a\nb: c", symbolize_names: true))
    end

    def test_duplicate_keys_last_wins
      assert_equal({ "a" => 2 }, YAML.load("a: 1\na: 2"))
    end

    def test_aliases_and_merge
      assert_equal Psych::AliasesNotEnabled, YamlTests.raised { YAML.load("a: &x 1\nb: *x") }
      assert_equal Psych::AliasesNotEnabled, YamlTests.raised { YAML.load("base: &b {x: 1}\nc:\n  <<: *b") }
      h = YAML.load("base: &b {x: 1, y: 2}\nc:\n  <<: *b\n  y: 3", aliases: true) #: Hash[String, untyped]
      assert_equal({ "x" => 1, "y" => 3 }, h["c"])
      a = YAML.unsafe_load("- &a [1, 2]\n- *a") #: Array[untyped]
      assert_same a[0], a[1]
      assert_equal({ "<<" => 5, "a" => 1 }, YAML.load("<<: 5\na: 1"))
      assert_equal({ "b" => { "c" => 1 }, "d" => { "c" => 1 } }, YAML.load("b: &x {c: 1}\nd:\n  '<<': *x", aliases: true))
      assert_equal({ "<<" => [1] }, YAML.load("<<: [1]"))
    end

    def test_empty_documents
      assert_nil YAML.load("")
      assert_nil YAML.load("# only a comment")
      assert_nil YAML.load("--- ~")
      assert_equal false, YAML.unsafe_load("")
      assert_nil YAML.unsafe_load("---\n")
      assert_equal 3, YAML.load("", fallback: 3)
      assert_equal({}, YAML.load("{}"))
      assert_equal [], YAML.load("[]")
    end

    def test_streams
      assert_equal 1, YAML.load("--- 1\n--- 2")
      assert_equal [1, { "a" => 2 }], YAML.load_stream("--- 1\n--- {a: 2}")
    end

    def test_freeze
      h = YAML.load("a: [1]", freeze: true) #: Hash[String, untyped]
      assert_equal [true, true], [h.frozen?, h["a"].frozen?]
    end
  end

  class ErrorTest < Minitest::Test
    def test_syntax_error
      err = nil #: Psych::SyntaxError?
      begin
        YAML.load("a: [1\nb: 2")
      rescue Psych::SyntaxError => e
        err = e
      end
      assert_equal [Psych::SyntaxError, true, true], [err.class, err.is_a?(Psych::Exception), err.is_a?(RuntimeError)]
      assert_equal true, err&.message&.start_with?("(<unknown>): ")
    end

    def test_filename_in_message
      msg = ""
      begin
        YAML.load("a: [1", filename: "conf.yml")
      rescue Psych::SyntaxError => e
        msg = e.message
      end
      assert_equal true, msg.start_with?("(conf.yml): ")
    end

    def test_object_tag
      assert_equal Psych::DisallowedClass, YamlTests.raised { YAML.load("x: !ruby/object:Foo {a: 1}") }
    end
  end

  class DumpTest < Minitest::Test
    def test_round_trip
      strs = ["", " x", "yes", "No", "~", "null", "1", "1.5", "017", "1:30", ":sym", "- a", "a: b", "a #b", "#c", "&a", "*a", "!t", "'q'", "[a]", "a,b",
              "line\nnext", "line\nnext\n", " lead\nx", "trail\n\n", "é ü", "2001-12-14", ".inf", "1_000", "it's", "---", "<<", "\n", "\n\n",
              "\xff".b]
      data = { "strs" => strs, "syms" => [:a, :"b c", :yes], "nums" => [0, -5, 1.0, -2.5, 1e20, Float::INFINITY],
               "nest" => [[1, [2]], { "k" => { "k2" => [nil, true] } }, [], {}], 1 => "int key", :s => "sym key", nil => "nil key" }
      assert_equal data, YAML.unsafe_load(YAML.dump(data))
      assert_equal data, YAML.unsafe_load(data.to_yaml)
      ["a\n---\nb", " a\nb", "x\ny\n", { "<<" => 1 }].each { |o| assert_equal o, YAML.unsafe_load(YAML.dump(o)) }
      assert_nil YAML.unsafe_load(YAML.dump(nil))
    end

    def test_scalars_and_header
      assert_equal [true, true, true], [YAML.dump(1).start_with?("---"), YAML.dump([1]).start_with?("---\n"), "x".to_yaml.end_with?("\n")]
      assert_equal "yes", YAML.load("yes".to_yaml)
      assert_equal [1, "2"], YAML.load([1, "2"].to_yaml)
    end

    def test_dump_to_io
      io = StringIO.new
      YAML.dump({ "a" => 1 }, io)
      assert_equal({ "a" => 1 }, YAML.load(io.string))
    end

    def test_load_file
      path = File.join(Dir.tmpdir, "rugaroo_yaml_test_#{Process.pid}.yml")
      File.write(path, { "name" => "x", "list" => [1, 2] }.to_yaml)
      assert_equal({ "name" => "x", "list" => [1, 2] }, YAML.load_file(path))
      assert_equal({ "name" => "x", "list" => [1, 2] }, YAML.safe_load_file(path))
    ensure
      File.delete(path) if path && File.exist?(path)
    end
  end
end
