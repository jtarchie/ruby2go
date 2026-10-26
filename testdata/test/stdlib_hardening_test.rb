# rbs_inline: enabled

require "minitest/autorun"
require "base64"
require "csv"
require "json"
require "securerandom"
require "stringio"
require "strscan"
require "tempfile"
require "tmpdir"

# #101 stdlib hardening: hostile or degenerate arguments that hung, panicked or were silently wrong. Every expectation is MRI's.
module StdlibHardeningTests
  # The exception class an argument error check raises, "none" if it returns.
  #: () { () -> untyped } -> String
  def self.raised
    yield
    "none"
  rescue Exception => e # rubocop:disable Lint/RescueException
    e.class.to_s
  end

  class CsvTest < Minitest::Test
    # an empty col_sep made the parser append nil fields forever; an empty quote_char looped in the quoted-field scan
    def test_parse_rejects_an_empty_col_sep
      assert_equal "ArgumentError", StdlibHardeningTests.raised { CSV.parse("a,b", col_sep: "") }
      assert_equal "ArgumentError", StdlibHardeningTests.raised { CSV.parse_line("a,b", col_sep: "") }
    end

    def test_parse_rejects_a_quote_char_that_is_not_one_character
      assert_equal "ArgumentError", StdlibHardeningTests.raised { CSV.parse("a,b", quote_char: "") }
      assert_equal "ArgumentError", StdlibHardeningTests.raised { CSV.parse("a,b", quote_char: "ab") }
    end

    def test_parse_still_takes_a_multi_character_col_sep
      assert_equal [["a", "b"]], CSV.parse("a::b", col_sep: "::")
    end

    # the generator never raises: an empty col_sep joins the fields, an empty quote_char quotes nothing
    def test_generate_with_empty_separators
      assert_equal "a b1\n", CSV.generate_line(["a b", 1], col_sep: "")
      assert_equal "\"a\"\"b\"\"\"\n", CSV.generate_line(["a\"b", nil, ""], col_sep: "")
      assert_equal "a,b,1\n", CSV.generate_line(["a,b", 1], quote_char: "")
      assert_equal "a\"b,,\n", CSV.generate_line(["a\"b", nil, ""], quote_char: "")
      assert_equal "12\n", CSV.generate(col_sep: "") { |csv| csv << [1, 2] }
    end
  end

  class SecureRandomTest < Minitest::Test
    def test_negative_sizes_raise_argument_error
      assert_equal "ArgumentError", StdlibHardeningTests.raised { SecureRandom.hex(-1) }
      assert_equal "ArgumentError", StdlibHardeningTests.raised { SecureRandom.random_bytes(-1) }
      assert_equal "ArgumentError", StdlibHardeningTests.raised { SecureRandom.base64(-1) }
      assert_equal "ArgumentError", StdlibHardeningTests.raised { SecureRandom.urlsafe_base64(-1) }
    end

    def test_zero_and_negative_alphanumeric_are_empty
      assert_equal "", SecureRandom.hex(0)
      assert_equal "", SecureRandom.random_bytes(0)
      assert_equal "", SecureRandom.alphanumeric(0)
      assert_equal "", SecureRandom.alphanumeric(-1)
    end
  end

  class Base64Test < Minitest::Test
    # Go's Strict() decoder skips \r and \n; MRI's m0 does not
    def test_strict_decode_rejects_whitespace_and_garbage
      ["YQ==\n", "YQ==\r", "YQ=\n=", "Y Q==", "YQ-_", "YQ", "Y\nQ=="].each do |s|
        assert_equal "ArgumentError", StdlibHardeningTests.raised { Base64.strict_decode64(s) }, s.inspect
      end
      assert_equal "a", Base64.strict_decode64("YQ==")
      assert_equal "", Base64.strict_decode64("")
    end

    def test_urlsafe_decode_rejects_newlines_but_takes_missing_padding
      assert_equal "ArgumentError", StdlibHardeningTests.raised { Base64.urlsafe_decode64("YQ==\n") }
      assert_equal "ArgumentError", StdlibHardeningTests.raised { Base64.urlsafe_decode64("YQ\n") }
      assert_equal "a", Base64.urlsafe_decode64("YQ")
      assert_equal "a", Base64.urlsafe_decode64("YQ==")
    end

    def test_lenient_decode_skips_newlines
      assert_equal "a", Base64.decode64("YQ==\n")
      assert_equal "a", Base64.decode64("YQ")
      assert_equal "ab", Base64.decode64("YWI=\n")
    end
  end

  class TempfileTest < Minitest::Test
    def test_tempfile_is_private_named_by_basename_and_round_trips
      f = Tempfile.new("hard")
      path = f.path.to_s
      assert_equal true, File.basename(path).start_with?("hard")
      assert_equal 0o600, File.stat(path).mode & 0o777
      f.write("hello")
      f.close
      assert_equal "hello", File.read(path)
      assert_equal true, File.exist?(path)
      f.unlink
      assert_equal false, File.exist?(path)
      assert_nil f.path
    end

    def test_tempfile_basename_and_suffix_pair
      f = Tempfile.new(["pre", ".txt"])
      b = File.basename(f.path.to_s)
      assert_equal true, b.start_with?("pre") && b.end_with?(".txt")
      f.close
      f.unlink
    end

    def test_tempfile_create_removes_the_file_after_the_block
      seen = ""
      mode = 0
      Tempfile.create("hardc") do |g|
        seen = g.path
        mode = File.stat(seen).mode & 0o777
        g.write("x")
      end
      assert_equal 0o600, mode
      assert_equal false, File.exist?(seen)
    end

    # the standard idiom: write, rewind, read
    def test_tempfile_write_rewind_read
      f = Tempfile.new("hardrw")
      f.write("hello")
      f.rewind
      assert_equal "hello", f.read
      f.puts "!"
      f.print "a", "b"
      f.printf("%d", 7)
      f.rewind
      assert_equal "hello!\nab7", f.read
      f.rewind
      assert_equal "hello!\n", f.gets
      f.close!
    end

    def test_tempfile_seek_pos_and_lines
      f = Tempfile.new("hardseek")
      f.write("one\ntwo\n")
      f.seek(4)
      assert_equal 4, f.pos
      assert_equal "two\n", f.gets
      f.rewind
      assert_equal ["one\n", "two\n"], f.readlines
      assert_equal true, f.eof?
      f.close!
    end

    def test_tempfile_size_and_length
      f = Tempfile.new("hardsize")
      assert_equal 0, f.size
      f.write("abc")
      assert_equal 3, f.size
      assert_equal 3, f.length
      f.close
      assert_equal 3, f.size
      f.unlink
      f.close!
    end

    def test_tempfile_close_unlink_now_and_close_bang
      f = Tempfile.new("hardclose")
      path = f.path.to_s
      assert_nil f.close
      assert_equal true, File.exist?(path)
      assert_equal true, f.closed?
      assert_equal [true, true], [f.inspect.start_with?("#<Tempfile:"), f.inspect.end_with?(" (closed)>")]
      assert_equal true, f.inspect.include?(path)
      f.close(true)
      assert_equal false, File.exist?(path)
      assert_nil f.path

      g = Tempfile.new("hardbang")
      gp = g.path.to_s
      g.write("x")
      g.close!
      assert_equal false, File.exist?(gp)
      assert_nil g.path
      assert_equal true, g.closed?
      assert_nil g.unlink
    end

    def test_tempfile_open_reopens_a_closed_file
      f = Tempfile.new("hardopen")
      f.write("zz")
      f.close
      f.open
      assert_equal false, f.closed?
      assert_equal "zz", f.read
      f.close!
    end

    def test_tempfile_class_open_closes_after_the_block_but_keeps_the_file
      seen = ""
      t = Tempfile.open("hardblock") do |g|
        g.write("q")
        seen = g.path.to_s
        g
      end
      assert_equal true, t.closed?
      assert_equal "q", File.read(seen)
      t.close!
    end

    # the failing path names the tmpdir argument, not the system temp dir
    def test_mktmpdir_error_names_the_requested_directory
      msg = ""
      begin
        Dir.mktmpdir("x", "/nonexistent/hardening") {}
      rescue SystemCallError => e
        msg = e.message
      end
      assert_equal true, msg.start_with?("No such file or directory @ dir_s_mkdir - /nonexistent/hardening/x"), msg
    end
  end

  class StringScannerTest < Minitest::Test
    # the anchored twin lives on the Regexp: a tokenizer using one built at run time, and a literal reused across scanners, both work
    def test_dynamic_regexps_scan_anchored
      out = [] #: Array[String]
      %w[foo bar baz].each do |w|
        s = StringScanner.new("#{w} 12 #{w}")
        re = Regexp.new(w)
        out << s.scan(re).to_s
        out << s.scan(re).to_s
        out << s.skip(/\s+/).to_s
        out << s.scan(/\d+/).to_s
        out << s.skip(/\s+/).to_s
        out << s.scan(re).to_s
      end
      assert_equal %w[foo 1 12 1 foo bar 1 12 1 bar baz 1 12 1 baz], out.reject(&:empty?)
    end

    def test_shared_regexp_across_scanners
      re = /\d+/
      a = StringScanner.new("12ab")
      b = StringScanner.new("ab34")
      assert_equal "12", a.scan(re)
      assert_nil b.scan(re)
      assert_equal "ab", b.scan(/[a-z]+/)
      assert_equal "34", b.scan(re)
      assert_nil a.scan(re)
    end
  end

  class JsonTest < Minitest::Test
    def test_dump_writes_to_the_io_and_returns_it
      io = StringIO.new
      ret = JSON.dump({ "a" => [1, nil] }, io)
      assert_same io, ret
      assert_equal "{\"a\":[1,null]}", io.string
    end

    def test_dump_with_nil_or_options_is_the_string
      assert_equal "[1]", JSON.dump([1], nil)
      assert_equal "[1]", JSON.dump([1])
    end

    def test_dump_allows_nan_by_default
      assert_equal "[NaN,Infinity]", JSON.dump([Float::NAN, Float::INFINITY])
      assert_equal "GeneratorError", StdlibHardeningTests.raised { JSON.dump([Float::NAN], { allow_nan: false }) }.sub("JSON::", "")
    end

    def test_parse_freeze
      doc = JSON.parse("{\"a\":[1,\"x\",{\"b\":null}]}", freeze: true)
      assert_equal true, doc.frozen?
      inner = doc["a"] #: untyped
      assert_equal true, inner.frozen?
      assert_equal true, inner[2].frozen?
      assert_equal false, JSON.parse("[1]").frozen?
      assert_equal false, JSON.parse("[1]", freeze: false).frozen?
    end

    def test_parse_option_values_that_change_nothing
      assert_equal [1], JSON.parse("[1]", allow_nan: false)
      assert_equal [1], JSON.parse("[1]", create_additions: false)
      assert_equal [1], JSON.parse("[1]", unknown_option: 1)
    end
  end

  # #102: ruby/spec core/io, file, dir, env, process argument checks
  class IOArgumentTest < Minitest::Test
    def test_closed_streams_raise_ioerror
      Dir.mktmpdir do |d|
        path = File.join(d, "x.txt")
        File.write(path, "a\n")
        f = File.open(path)
        f.close
        r, w = IO.pipe
        r.close
        w.close
        got = [
          StdlibHardeningTests.raised { f.binmode }, StdlibHardeningTests.raised { f.binmode? }, StdlibHardeningTests.raised { f.lineno },
          StdlibHardeningTests.raised { f.lineno = 1 }, StdlibHardeningTests.raised { f.write "a" }, StdlibHardeningTests.raised { f.dup },
          StdlibHardeningTests.raised { f.sync }, StdlibHardeningTests.raised { f.sync = true }, StdlibHardeningTests.raised { f.flush },
          StdlibHardeningTests.raised { r.binmode }, StdlibHardeningTests.raised { r.binmode? }, StdlibHardeningTests.raised { r.lineno },
          StdlibHardeningTests.raised { w.lineno = 1 }, StdlibHardeningTests.raised { w.dup }
        ]
        assert_equal ["IOError"] * 14, got
      end
    end

    def test_write_only_lineno_and_negative_lengths
      Dir.mktmpdir do |d|
        path = File.join(d, "x.txt")
        w = File.open(path, "w")
        assert_equal ["IOError", "IOError", 0], [StdlibHardeningTests.raised { w.lineno }, StdlibHardeningTests.raised { w.lineno = 2 }, w.write("")]
        w.close
        r, pw = IO.pipe
        assert_equal ["ArgumentError", "ArgumentError"], [StdlibHardeningTests.raised { r.readpartial(-1) }, StdlibHardeningTests.raised { r.read_nonblock(-1) }]
        r.close
        pw.close
        e = begin; IO.read(path, -1, -1); rescue ArgumentError => x; x.message; end
        assert_equal "negative offset -1 given", e
        assert_equal "IOError", StdlibHardeningTests.raised { IO.read(path, mode: "w") }
      end
    end

    def test_io_new_options_said_twice
      Dir.mktmpdir do |d|
        fd = IO.sysopen(File.join(d, "x.txt"), "w")
        got = [
          StdlibHardeningTests.raised { IO.new(fd, "r") }, StdlibHardeningTests.raised { IO.new(fd, "w", mode: "w") },
          StdlibHardeningTests.raised { IO.new(fd, "w:UTF-8", encoding: "UTF-8") }, StdlibHardeningTests.raised { IO.new(fd, "wb", binmode: true) },
          StdlibHardeningTests.raised { IO.new(fd, "wt", textmode: false) }, StdlibHardeningTests.raised { IO.new(fd, "wb", textmode: true) },
          StdlibHardeningTests.raised { IO.new(fd, "w", nil) }
        ]
        assert_equal ["Errno::EINVAL"] + ["ArgumentError"] * 6, got
        io = IO.new(fd, "wb")
        assert_equal true, io.binmode?
        io.close
      end
    end

    def test_file_open_modes
      Dir.mktmpdir do |d|
        x = File.join(d, "x.txt")
        File.open(x, "wx") { |f| f.write("c") }
        assert_equal "c", File.read(x)
        assert_equal ["Errno::EEXIST", "ArgumentError", "ArgumentError"],
          [StdlibHardeningTests.raised { File.open(x, "wx") { 1 } }, StdlibHardeningTests.raised { File.open(x, "rx") { 1 } },
           StdlibHardeningTests.raised { File.open(x, "rb", newline: :universal) { 1 } }]
        assert_equal ["Errno::ENOTDIR", "Errno::ENOENT", false, "Errno::EINVAL", "Errno::EINVAL"],
          [StdlibHardeningTests.raised { Dir.rmdir(x) }, StdlibHardeningTests.raised { Dir.empty?(File.join(d, "nope")) }, Dir.empty?(x),
           StdlibHardeningTests.raised { File.truncate(x, -1) }, StdlibHardeningTests.raised { File.readlink(x) }]
      end
    end

    def test_paths
      assert_equal ["usr//bin", "usr/bin", "/", "a/", "/a", ""], [File.join("usr/", "//bin"), File.join("usr//", "/bin"), File.join("", ""), File.join("a", ""), File.join("", "a"), File.join("")]
      assert_equal "ArgumentError", StdlibHardeningTests.raised { File.join("a\0", "b") }
      assert_equal ["/tmp", "/tmp", "////some/path", "/some/path", "/dir", "/", "/", "/foo"],
        [File.expand_path("/tmp/"), File.expand_path("/tmp/../../../tmp"), File.expand_path("////some/path"), File.expand_path("/some////path"),
         File.expand_path("/./dir"), File.expand_path(".", "/"), File.expand_path("///a/../.."), File.dirname("/////foo/bar/")]
      assert_equal [true, true, "ArgumentError"],
        [File.expand_path("~#{ENV["USER"]}/a") == "#{Dir.home}/a", File.expand_path("../foo", "~/dir") == "#{Dir.home}/foo",
         StdlibHardeningTests.raised { File.expand_path("~a_not_existing_user_rugaroo") }]
      Dir.mktmpdir do |d|
        real = File.realpath(d)
        Dir.mkdir(File.join(d, "x"))
        assert_equal true, File.realpath("x", d) == File.join(real, "x")
      end
    end

    # File::FNM_DOTMATCH: wildcards take dot entries, and `**/*` lists `.` at zero depth only, as MRI (Homebrew's
    # Quarantine.propagate)
    def test_dir_glob_dotmatch
      Dir.mktmpdir do |d|
        Dir.mkdir(File.join(d, "a"))
        Dir.mkdir(File.join(d, "a", ".h"))
        Dir.mkdir(File.join(d, ".d"))
        %w[x .y a/z a/.h/w .d/v].each { |f| File.write(File.join(d, f), "") }
        Dir.chdir(d) do
          assert_equal %w[. .d .d/v .y a a/.h a/.h/w a/z x], Dir.glob("**/*", File::FNM_DOTMATCH)
          assert_equal %w[. .d .y a x], Dir.glob("*", File::FNM_DOTMATCH)
          assert_equal %w[a a/z x], Dir.glob("**/*")
          assert_equal %w[a/. a/.h a/.h/w a/z], Pathname.glob(Pathname("a") / "**/*", File::FNM_DOTMATCH).map(&:to_s)
        end
      end
    end

    def test_dir_glob_edges
      Dir.mktmpdir do |d|
        Dir.mkdir(File.join(d, "deeply"))
        Dir.mkdir(File.join(d, "deeply", "nested"))
        Dir.mkdir(File.join(d, ".dotsubdir"))
        File.write(File.join(d, ".dotfile"), "")
        File.write(File.join(d, "deeply", "nested", "f"), "")
        Dir.chdir(d) do
          assert_equal [[], [], %w[. .dotfile .dotsubdir], %w[. .dotfile .dotsubdir], %w[. .dotfile .dotsubdir deeply], %w[deeply/ deeply/nested/], ["deeply//nested/f"]],
            ["", "{}", ".*", ".**", "{,.}*", "deeply/**/", "{deeply//**/f}"].map { |pat| Dir.glob(pat).sort }
          assert_equal "ArgumentError", StdlibHardeningTests.raised { Dir.glob("a\0b") }
        end
      end
    end

    def test_env_and_spawn_names
      got = [
        StdlibHardeningTests.raised { ENV["a=b"] = "x" }, StdlibHardeningTests.raised { ENV[""] = "x" },
        StdlibHardeningTests.raised { ENV.merge!("" => "c") }, StdlibHardeningTests.raised { ENV["a=b"] = nil },
        StdlibHardeningTests.raised { Process.spawn("echo\0x") }, StdlibHardeningTests.raised { Process.spawn("echo", "a\0b") },
        StdlibHardeningTests.raised { Process.spawn({ "A=B" => "x" }, "echo") }, StdlibHardeningTests.raised { Process.spawn("") },
        StdlibHardeningTests.raised { Process.spawn({}, "true", unsetenv_others: 1) }
      ]
      assert_equal ["Errno::EINVAL"] * 3 + ["none"] + ["ArgumentError"] * 3 + ["Errno::ENOENT", "ArgumentError"], got
      assert_equal ["ENV", "TypeError"], [ENV.to_s, StdlibHardeningTests.raised { ENV.dup }]
    end
  end
end
