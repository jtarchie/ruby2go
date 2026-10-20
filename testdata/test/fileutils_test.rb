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

    # Homebrew's with_verified_snapshot: a copy to the exact path
    def test_copy_file
      Dir.mktmpdir do |d|
        a = File.join(d, "a")
        File.write(a, "hello")
        FileUtils.copy_file(a, File.join(d, "b"))
        assert_equal ["hello", false], [File.read(File.join(d, "b")), File.directory?(File.join(d, "b"))]
      end
    end

    # Homebrew's keg code: rm_r, symbolic chmod and chmod_R, install, ln, chown, mkdir/rmdir (#100)
    # mv and rm take MRI's options (Homebrew's UnpackStrategy::Directory: `FileUtils.mv(src, dst, verbose:)`).
    def test_mv_rm_options
      Dir.mktmpdir do |d|
        a = File.join(d, "a")
        File.write(a, "x")
        FileUtils.mv(a, File.join(d, "b"), verbose: false)
        moved = Dir.children(d)
        FileUtils.rm(File.join(d, "b"), verbose: false)
        FileUtils.rm(File.join(d, "nope"), force: true)
        assert_equal [["b"], []], [moved, Dir.children(d)]
      end
    end

    def test_install_path_methods
      Dir.mktmpdir do |d|
        a = File.join(d, "a")
        f = File.join(a, "f")
        FileUtils.mkdir([a])
        File.write(f, "x")
        FileUtils.chmod("u+x,go-r", f)
        modes = [File.stat(f).mode & 0o777]
        FileUtils.chmod(0o644, f)
        FileUtils.chmod_R("u-w", a)
        modes << (File.stat(f).mode & 0o777)
        FileUtils.chmod_R("u+w", a)
        FileUtils.install(f, File.join(d, "g"), mode: 0o755)
        modes << (File.stat(File.join(d, "g")).mode & 0o777)
        FileUtils.ln(File.join(d, "g"), File.join(d, "h"))
        FileUtils.chown(nil, Process.gid, File.join(d, "g"))
        same = [FileUtils.identical?(File.join(d, "g"), File.join(d, "h")), FileUtils.compare_file(f, File.join(d, "g")),
                FileUtils.uptodate?(File.join(d, "g"), [f])]
        File.write(File.join(d, "g"), "y")
        same << FileUtils.cmp(f, File.join(d, "g"))
        FileUtils.rm_r(a)
        err = begin
          FileUtils.rm_r(a)
          nil
        rescue Errno::ENOENT => e
          e.class
        end
        FileUtils.rm_r(a, force: true)
        FileUtils.mkdir(File.join(d, "e"))
        FileUtils.rmdir(File.join(d, "e"))
        assert_equal [[0o700, 0o444, 0o755], [true, true, true, false], true, false, Errno::ENOENT, false],
          [modes, same, File.identical?(File.join(d, "g"), File.join(d, "h")), File.exist?(a), err, File.exist?(File.join(d, "e"))]
      end
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

    # Homebrew's create_symlink_to_cached_download: Pathnames, force over an existing file, into a directory
    def test_ln_s_force
      Dir.mktmpdir do |d|
        File.write(File.join(d, "a"), "x")
        File.write(File.join(d, "b"), "y")
        Dir.mkdir(File.join(d, "dir"))
        FileUtils.ln_s(Pathname("a"), Pathname(File.join(d, "b")), force: true)
        FileUtils.ln_sf("a", File.join(d, "dir"))
        assert_equal ["a", "a", "x"], [File.readlink(File.join(d, "b")), File.readlink(File.join(d, "dir", "a")), File.read(File.join(d, "b"))]
      end
    end

    def test_touch_mtime_and_nocreate
      Dir.mktmpdir do |d|
        f = File.join(d, "a")
        FileUtils.touch(f, mtime: Time.at(0))
        assert_equal 0, File.mtime(f).to_i
        FileUtils.touch([f], mtime: Time.at(86_400))
        assert_equal 86_400, File.mtime(f).to_i
        missing = File.join(d, "b")
        err = begin
          FileUtils.touch(missing, nocreate: true)
          nil
        rescue SystemCallError => e
          e
        end
        assert_equal Errno::ENOENT, err&.class
        refute File.exist?(missing)
      end
    end
  end
end
