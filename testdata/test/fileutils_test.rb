# rbs_inline: enabled

require "minitest/autorun"
require "fileutils"
require "tmpdir"

# The old open-with-truncate emptied the source when src and dst were the same file; refuse masks the tmpdir because assert_raises would trace it.
module FileUtilsTests
  class SameFileTest < Minitest::Test
    #: (String) { () -> void } -> String
    def refuse(dir)
      yield
      "no error"
    rescue ArgumentError => e
      e.message.gsub(dir, "D")
    end

    def test_cp_same_path
      Dir.mktmpdir do |d|
        a = File.join(d, "a")
        File.write(a, "hello")
        m = refuse(d) { FileUtils.cp(a, a) }
        assert_equal "hello", File.read(a)
        assert_equal "same file: D/a and D/a", m
      end
    end

    def test_cp_through_symlink_and_hard_link
      Dir.mktmpdir do |d|
        a = File.join(d, "a")
        sl = File.join(d, "sl")
        hl = File.join(d, "hl")
        File.write(a, "hello")
        File.symlink(a, sl)
        File.link(a, hl)
        msgs = [refuse(d) { FileUtils.cp(a, sl) }, refuse(d) { FileUtils.cp(a, hl) }, refuse(d) { FileUtils.cp(hl, a) }]
        assert_equal ["hello", "hello"], [File.read(a), File.read(hl)]
        assert_equal ["same file: D/a and D/sl", "same file: D/a and D/hl", "same file: D/hl and D/a"], msgs
      end
    end

    def test_cp_into_directory_holding_itself
      Dir.mktmpdir do |d|
        a = File.join(d, "a")
        File.write(a, "hello")
        assert_equal "same file: D/a and D/a", refuse(d) { FileUtils.cp(a, d) }
        assert_equal "hello", File.read(a)
      end
    end

    def test_cp_r_same_file
      Dir.mktmpdir do |d|
        a = File.join(d, "a")
        File.write(a, "hello")
        assert_equal "same file: D/a and D/a", refuse(d) { FileUtils.cp_r(a, a) }
        assert_equal "hello", File.read(a)
      end
    end

    # Relative paths: MRI compares names textually and only catches these (an absolute /var vs /private/var tmpdir slips past it).
    def test_cp_r_dir_into_itself
      Dir.mktmpdir do |d|
        Dir.chdir(d) do
          Dir.mkdir("t")
          File.write("t/f", "1")
          Dir.mkdir("t/sub")
          File.write("t/sub/g", "2")
          before = Dir.glob("t/**/*").sort
          m = refuse(d) { FileUtils.cp_r("t", "t/b") }
          assert_equal "cannot copy directory X to itself t/b", m.sub(%r{directory /\S*/t to}, "directory X to")
          m = refuse(d) { FileUtils.cp_r("t", "t") }
          assert_equal "cannot copy directory X to itself t/t", m.sub(%r{directory /\S*/t to}, "directory X to")
          assert_equal before, Dir.glob("t/**/*").sort
          assert_equal "1", File.read("t/f")
        end
      end
    end

    def test_mv_same_file
      Dir.mktmpdir do |d|
        a = File.join(d, "a")
        hl = File.join(d, "hl")
        File.write(a, "hello")
        File.link(a, hl)
        assert_equal "same file: D/a and D/a", refuse(d) { FileUtils.mv(a, a) }
        assert_equal "same file: D/a and D/hl", refuse(d) { FileUtils.mv(a, hl) }
        assert_equal ["hello", "hello"], [File.read(a), File.read(hl)]
      end
    end

    def test_cp_distinct_files_still_copy
      Dir.mktmpdir do |d|
        a = File.join(d, "a")
        b = File.join(d, "b")
        File.write(a, "hello")
        File.write(b, "old and longer")
        FileUtils.cp(a, b)
        assert_equal ["hello", "hello"], [File.read(a), File.read(b)]
        FileUtils.mkdir_p(File.join(d, "out"))
        FileUtils.cp_r(a, File.join(d, "out"))
        assert_equal "hello", File.read(File.join(d, "out", "a"))
      end
    end
  end
end
