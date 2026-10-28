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

    # setuid/setgid/sticky survive File.chmod, and symbolic modes that do not name them leave them alone
    def test_chmod_special_bits
      Dir.mktmpdir do |d|
        f = File.join(d, "f")
        sd = File.join(d, "sd")
        File.write(f, "x")
        Dir.mkdir(sd)
        File.chmod(0o6755, f)
        modes = [File.stat(f).mode & 0o7777]
        FileUtils.chmod("u+x", f)
        modes << (File.stat(f).mode & 0o7777)
        FileUtils.chmod("g-s", f)
        modes << (File.stat(f).mode & 0o7777)
        File.chmod(0o755, f)
        modes << (File.stat(f).mode & 0o7777)
        FileUtils.chmod("u+s", f)
        modes << (File.stat(f).mode & 0o7777)
        File.chmod(0o1777, sd)
        modes << (File.stat(sd).mode & 0o7777)
        FileUtils.chmod("u+w", sd)
        modes << (File.stat(sd).mode & 0o7777)
        FileUtils.chmod("o-t", sd)
        modes << (File.stat(sd).mode & 0o7777)
        assert_equal [0o6755, 0o6755, 0o4755, 0o755, 0o4755, 0o1777, 0o1777, 0o777], modes
      end
    end

    # a read-only source dir copies: the new dir is writable while its children go in
    def test_cp_r_readonly_dir
      Dir.mktmpdir do |d|
        src = File.join(d, "src")
        FileUtils.mkdir_p(File.join(src, "sub"))
        File.write(File.join(src, "a"), "A")
        File.write(File.join(src, "sub", "b"), "B")
        File.chmod(0o555, File.join(src, "sub"))
        File.chmod(0o555, src)
        out = File.join(d, "out")
        err = begin
          FileUtils.cp_r(src, out)
          nil
        rescue SystemCallError => e
          e.class
        end
        copied = [File.read(File.join(out, "a")), File.read(File.join(out, "sub", "b"))] unless err
        assert_equal [nil, ["A", "B"], true], [err, copied, File.stat(out).mode.anybits?(0o200)]
      ensure
        File.chmod(0o755, File.join(d, "src", "sub")) if File.directory?(File.join(d, "src", "sub"))
        File.chmod(0o755, File.join(d, "src")) if File.directory?(File.join(d, "src"))
        File.chmod(0o755, File.join(d, "out", "sub")) if File.directory?(File.join(d, "out", "sub"))
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

  # mode:, noop: and verbose: on the methods MRI gives them; verbose prints the equivalent shell command to $stdout (fu_output_message, fileutils 1.8)
  class OptionsTest < Minitest::Test
    #: (String) -> Integer
    def fu_perm(path) = File.stat(path).mode & 0o7777

    # the class only: assert_raises would trace the message, which names the temporary directory
    #: () { () -> void } -> untyped
    def fu_error_class
      yield
      nil
    rescue SystemCallError => e
      e.class
    end

    #: (String) { () -> void } -> String
    def fu_err(dir)
      out, = capture_io { yield }
      out.gsub(dir, "D")
    end

    def test_dir_mkdir_takes_a_mode
      Dir.mktmpdir do |d|
        Dir.mkdir(File.join(d, "a"), 0o700)
        Dir.mkdir(File.join(d, "b"), 0o750)
        Dir.mkdir(File.join(d, "c"))
        assert_equal [0o700, 0o750, 0o755], [fu_perm(File.join(d, "a")), fu_perm(File.join(d, "b")), fu_perm(File.join(d, "c"))]
      end
    end

    def test_pathname_mkdir_and_mkpath_take_a_mode
      Dir.mktmpdir do |d|
        Pathname(d).join("a").mkdir(0o700)
        Pathname(d).join("p/q/r").mkpath(mode: 0o700)
        assert_equal [0o700, 0o700, 0o700, 0o700], [fu_perm(File.join(d, "a")), fu_perm(File.join(d, "p")), fu_perm(File.join(d, "p/q")), fu_perm(File.join(d, "p/q/r"))]
        Pathname(d).join("p/q/r").mkpath # an existing directory is fine
      end
    end

    def test_mkdir_and_mkpath_modes
      Dir.mktmpdir do |d|
        FileUtils.mkdir(File.join(d, "a"), mode: 0o700)
        FileUtils.mkdir_p(File.join(d, "x/y"), mode: 0o700)
        FileUtils.mkpath(File.join(d, "m/n"), mode: 0o711)
        FileUtils.mkdir_p([File.join(d, "l1"), File.join(d, "l2/z")], mode: 0o750)
        got = %w[a x x/y m m/n l1 l2 l2/z].map { |n| fu_perm(File.join(d, n)) }
        assert_equal [0o700, 0o700, 0o700, 0o711, 0o711, 0o750, 0o750, 0o750], got
        FileUtils.mkdir_p(File.join(d, "x/y"), mode: 0o755) # existing: untouched
        assert_equal 0o700, fu_perm(File.join(d, "x/y"))
      end
    end

    def test_mkdir_p_errors_like_mri
      Dir.mktmpdir do |d|
        f = File.join(d, "f")
        File.write(f, "x")
        assert_equal [Errno::EEXIST, Errno::EEXIST], [fu_error_class { FileUtils.mkdir_p(f) }, fu_error_class { FileUtils.mkdir_p(File.join(f, "z"), mode: 0o700) }]
      end
    end

    def test_noop_does_nothing
      Dir.mktmpdir do |d|
        a = File.join(d, "a")
        File.write(a, "x")
        FileUtils.mkdir(File.join(d, "m"), noop: true)
        FileUtils.mkdir_p(File.join(d, "n/o"), noop: true)
        FileUtils.mkpath(File.join(d, "q"), noop: true)
        FileUtils.cp(a, File.join(d, "b"), noop: true)
        FileUtils.cp_r(a, File.join(d, "c"), noop: true)
        FileUtils.mv(a, File.join(d, "e"), noop: true)
        FileUtils.ln_s(a, File.join(d, "l"), noop: true)
        FileUtils.ln(a, File.join(d, "h"), noop: true)
        FileUtils.install(a, File.join(d, "i"), noop: true)
        FileUtils.touch(File.join(d, "t"), noop: true)
        FileUtils.rm(a, noop: true)
        FileUtils.rm_f(a, noop: true)
        FileUtils.rm_r(a, noop: true)
        FileUtils.rm_rf(a, noop: true)
        FileUtils.chmod(0o600, a, noop: true)
        assert_equal ["a"], Dir.children(d)
        assert_equal 0o644, fu_perm(a)
      end
    end

    def test_verbose_messages_match_mri
      Dir.mktmpdir do |d|
        a = File.join(d, "a")
        File.write(a, "x")
        got = [] #: Array[String]
        got << fu_err(d) { FileUtils.mkdir(File.join(d, "m"), mode: 0o700, verbose: true) }
        got << fu_err(d) { FileUtils.mkdir([File.join(d, "m2"), File.join(d, "m3")], verbose: true) }
        got << fu_err(d) { FileUtils.mkdir_p(File.join(d, "x/y"), mode: 0o700, verbose: true) }
        got << fu_err(d) { FileUtils.mkdir_p([File.join(d, "p"), File.join(d, "q")], verbose: true) }
        got << fu_err(d) { FileUtils.mkpath(File.join(d, "m/n"), mode: 0o711, verbose: true) }
        got << fu_err(d) { FileUtils.cp(a, File.join(d, "b"), verbose: true) }
        got << fu_err(d) { FileUtils.cp(a, File.join(d, "b"), preserve: true, verbose: true) }
        got << fu_err(d) { FileUtils.cp_r(File.join(d, "x"), File.join(d, "z"), verbose: true) }
        got << fu_err(d) { FileUtils.cp_r(File.join(d, "x"), File.join(d, "z2"), preserve: true, verbose: true) }
        got << fu_err(d) { FileUtils.mv(File.join(d, "b"), File.join(d, "b2"), verbose: true) }
        got << fu_err(d) { FileUtils.mv(File.join(d, "b2"), File.join(d, "b3"), force: true, verbose: true) }
        got << fu_err(d) { FileUtils.rm(File.join(d, "b3"), verbose: true) }
        got << fu_err(d) { FileUtils.rm(File.join(d, "b3"), force: true, verbose: true) }
        got << fu_err(d) { FileUtils.rm_f(File.join(d, "nope"), verbose: true) }
        got << fu_err(d) { FileUtils.rm_r(File.join(d, "z"), verbose: true) }
        got << fu_err(d) { FileUtils.rm_rf(File.join(d, "z2"), verbose: true) }
        got << fu_err(d) { FileUtils.rm_rf([File.join(d, "p"), File.join(d, "q")], verbose: true) }
        got << fu_err(d) { FileUtils.rmdir(File.join(d, "m2"), verbose: true) }
        got << fu_err(d) { FileUtils.rmdir(File.join(d, "x/y"), parents: true, verbose: true) }
        got << fu_err(d) { FileUtils.touch(File.join(d, "t"), verbose: true) }
        got << fu_err(d) { FileUtils.touch(File.join(d, "t"), nocreate: true, verbose: true) }
        got << fu_err(d) { FileUtils.ln_s(a, File.join(d, "l"), verbose: true) }
        got << fu_err(d) { FileUtils.ln_s(a, File.join(d, "l2"), force: true, verbose: true) }
        got << fu_err(d) { FileUtils.ln_sf(a, File.join(d, "l3"), verbose: true) }
        got << fu_err(d) { FileUtils.ln(a, File.join(d, "h"), verbose: true) }
        got << fu_err(d) { FileUtils.ln(a, File.join(d, "h"), force: true, verbose: true) }
        got << fu_err(d) { FileUtils.chmod(0o600, a, verbose: true) }
        got << fu_err(d) { FileUtils.chmod("u+x", a, verbose: true) }
        got << fu_err(d) { FileUtils.chmod_R(0o755, File.join(d, "m"), verbose: true) }
        got << fu_err(d) { FileUtils.install(a, File.join(d, "i"), mode: 0o644, verbose: true) }
        got << fu_err(d) { FileUtils.install(a, File.join(d, "i2"), verbose: true) }
        got << fu_err(d) { FileUtils.mkdir(File.join(d, "noop"), noop: true, verbose: true) }
        got << fu_err(d) { FileUtils.chown(nil, nil, a, verbose: true) }
        want = <<~MSG
          mkdir -m 700 D/m
          mkdir D/m2 D/m3
          mkdir -p -m 700 D/x/y
          mkdir -p D/p D/q
          mkdir -p -m 711 D/m/n
          cp D/a D/b
          cp -p D/a D/b
          cp -r D/x D/z
          cp -rp D/x D/z2
          mv D/b D/b2
          mv -f D/b2 D/b3
          rm D/b3
          rm -f D/b3
          rm -f D/nope
          rm -r D/z
          rm -rf D/z2
          rm -rf D/p D/q
          rmdir D/m2
          rmdir -p D/x/y
          touch D/t
          touch -c D/t
          ln -s D/a D/l
          ln -sf D/a D/l2
          ln -sf D/a D/l3
          ln D/a D/h
          ln -f D/a D/h
          chmod 600 D/a
          chmod u+x D/a
          chmod -R 755 D/m
          install -c -m 644 D/a D/i
          install -c D/a D/i2
          mkdir D/noop
          chown : D/a
        MSG
        assert_equal want, got.join
      end
    end

    def test_rmdir_parents_and_rm_force
      Dir.mktmpdir do |d|
        FileUtils.mkdir_p(File.join(d, "x/y/z"))
        File.write(File.join(d, "x/keep"), "k")
        FileUtils.rmdir(File.join(d, "x/y/z"), parents: true)
        assert_equal [false, true], [File.exist?(File.join(d, "x/y")), File.exist?(File.join(d, "x"))]
        FileUtils.mkdir(File.join(d, "dd"))
        File.write(File.join(d, "dd", "k"), "k")
        FileUtils.rm(File.join(d, "dd"), force: true) # force swallows any error, as MRI's
        assert_equal true, File.directory?(File.join(d, "dd"))
        assert_equal false, fu_error_class { FileUtils.rm(File.join(d, "dd")) }.nil?
      end
    end

    def test_cp_preserve_keeps_mode_and_mtime
      Dir.mktmpdir do |d|
        src = File.join(d, "src")
        File.write(src, "x")
        File.chmod(0o751, src)
        File.utime(Time.at(1_000_000), Time.at(1_000_000), src)
        FileUtils.cp(src, File.join(d, "plain"))
        FileUtils.cp(src, File.join(d, "kept"), preserve: true)
        assert_equal [0o751, 1_000_000], [fu_perm(File.join(d, "kept")), File.mtime(File.join(d, "kept")).to_i]
        assert_equal false, File.mtime(File.join(d, "plain")).to_i == 1_000_000
        FileUtils.mkdir_p(File.join(d, "t/sub"))
        File.write(File.join(d, "t/sub/f"), "y")
        File.chmod(0o640, File.join(d, "t/sub/f"))
        File.utime(Time.at(2_000_000), Time.at(2_000_000), File.join(d, "t/sub/f"))
        FileUtils.cp_r(File.join(d, "t"), File.join(d, "t2"), preserve: true)
        assert_equal [0o640, 2_000_000], [fu_perm(File.join(d, "t2/sub/f")), File.mtime(File.join(d, "t2/sub/f")).to_i]
      end
    end
  end
end

module FileUtilsTests
  # compare_file streams in chunks after a size check; the edges are the 64 KB chunk size and where the files differ.
  class CompareFileStreamTest < Minitest::Test
    def test_compare_file_chunk_edges
      Dir.mktmpdir do |d|
        a = File.join(d, "a")
        b = File.join(d, "b")
        got = [] #: Array[bool]
        [0, 1, 65_535, 65_536, 65_537, 196_608].each do |n|
          File.write(a, "x" * n)
          File.write(b, "x" * n)
          got << FileUtils.compare_file(a, b)
          got << FileUtils.compare_file(a, a)
          File.write(b, "x" * (n + 1))
          got << FileUtils.compare_file(a, b)
          next if n.zero?

          File.write(b, "x" * (n - 1) + "y")
          got << FileUtils.compare_file(a, b)
          File.write(b, "y" + "x" * (n - 1))
          got << FileUtils.compare_file(a, b)
          File.write(b, "x" * (n / 2) + "y" + "x" * (n - (n / 2) - 1))
          got << FileUtils.compare_file(a, b)
        end
        assert_equal [true, true, false] + ([true, true, false, false, false, false] * 5), got
        assert_equal [true, true], [FileUtils.identical?(a, a), FileUtils.cmp(a, a)]
      end
    end

    def test_compare_file_missing_raises_enoent
      Dir.mktmpdir do |d|
        a = File.join(d, "a")
        File.write(a, "x")
        raised = [] #: Array[Class]
        begin
          FileUtils.compare_file(a, File.join(d, "nope"))
        rescue Errno::ENOENT => e
          raised << e.class
        end
        begin
          FileUtils.compare_file(File.join(d, "nope"), a)
        rescue Errno::ENOENT => e
          raised << e.class
        end
        assert_equal [Errno::ENOENT, Errno::ENOENT], raised
      end
    end
  end
end

module FileUtilsTests
  # MRI's return values: lists as strings (the array as given for cp/mv/ln), 0 or nil for a lone path, nil under noop except mkdir_p. Paths are relative (a tmpdir cwd) so the trace operands match MRI's.
  class ReturnValueTest < Minitest::Test
    #: () { (String) -> void } -> void
    def fu_in
      Dir.mktmpdir do |d|
        old = Dir.pwd
        Dir.chdir(d)
        begin
          yield d
        ensure
          Dir.chdir(old)
        end
      end
    end

    #: (String, bool) -> String
    def fu_mk(name, is_dir)
      is_dir ? Dir.mkdir(name) : File.write(name, "x")
      name
    end

    def test_mkdir_touch_chmod_return_the_path_list
      fu_in do
        assert_equal ["a"], FileUtils.mkdir("a")
        assert_equal ["b", "c"], FileUtils.mkdir(["b", "c"])
        assert_equal ["p/q"], FileUtils.mkdir_p("p/q")
        assert_equal ["s/t"], FileUtils.makedirs("s/t")
        assert_equal ["u/v"], FileUtils.mkpath("u/v")
        assert_equal ["w", "x"], FileUtils.mkdir_p([Pathname("w"), "x"])
        assert_equal ["y"], FileUtils.mkdir_p("y", noop: true)
        assert_equal ["y"], FileUtils.mkpath("y", noop: true)
        assert_nil FileUtils.mkdir("z", noop: true)
        assert_equal ["f", "g"], FileUtils.touch(["f", "g"])
        assert_equal ["f"], FileUtils.touch(Pathname("f"))
        assert_nil FileUtils.touch("f", noop: true)
        assert_equal ["f"], FileUtils.chmod(0o600, "f")
        assert_equal ["f", "g"], FileUtils.chmod("u+w", ["f", "g"])
        assert_equal ["a"], FileUtils.chmod_R(0o755, "a")
        assert_equal ["f"], FileUtils.chown(nil, nil, "f")
        assert_equal ["a"], FileUtils.chown_R(nil, nil, "a")
        assert_nil FileUtils.chmod(0o600, "f", noop: true)
        assert_nil FileUtils.chmod_R(0o600, "f", noop: true)
      end
    end

    def test_remove_methods_return_the_path_list
      fu_in do
        assert_equal ["f1"], FileUtils.rm(fu_mk("f1", false))
        fu_mk("f2", false)
        fu_mk("f3", false)
        assert_equal ["f2", "f3"], FileUtils.rm(["f2", "f3"])
        assert_equal ["f4"], FileUtils.remove(fu_mk("f4", false))
        assert_equal ["f5", "missing"], FileUtils.rm_f([fu_mk("f5", false), "missing"])
        assert_equal ["f6"], FileUtils.rm(Pathname(fu_mk("f6", false)))
        assert_equal ["f7"], FileUtils.safe_unlink(fu_mk("f7", false))
        assert_equal ["d1"], FileUtils.rm_r(fu_mk("d1", true))
        assert_equal ["d2"], FileUtils.rm_rf(fu_mk("d2", true))
        assert_equal ["d3", "missing"], FileUtils.rm_rf([fu_mk("d3", true), "missing"])
        assert_equal ["d4"], FileUtils.rmtree(fu_mk("d4", true))
        assert_equal ["d5"], FileUtils.rm_r(Pathname(fu_mk("d5", true)))
        assert_equal ["e1"], FileUtils.rmdir(fu_mk("e1", true))
        assert_equal ["e2"], FileUtils.rmdir([fu_mk("e2", true)])
        keep = fu_mk("keep", false)
        assert_nil FileUtils.rm(keep, noop: true)
        assert_nil FileUtils.rm_rf(keep, noop: true)
        assert_nil FileUtils.rm_r(keep, noop: true)
        assert_nil FileUtils.rmdir(".", noop: true)
        assert File.exist?(keep)
      end
    end

    def test_remove_entry_returns_what_the_last_removal_returned
      fu_in do
        assert_equal 1, FileUtils.remove_entry(fu_mk("f", false))
        Dir.mkdir("dir")
        File.write("dir/in", "y")
        assert_equal 0, FileUtils.remove_entry("dir")
        assert_nil FileUtils.remove_entry("f", true)
        File.write("f", "x")
        File.symlink("f", "ln")
        assert_equal 1, FileUtils.remove_entry("ln")
        assert_nil FileUtils.remove_entry_secure("f")
        File.write("f", "x")
        assert_equal 1, FileUtils.remove_file("f")
        Dir.mkdir("dir")
        assert_equal 0, FileUtils.remove_dir("dir")
        assert_nil FileUtils.remove_file("f", true)
      end
    end

    def test_cp_install_return_nil_for_one_path_and_the_list_for_an_array
      fu_in do
        fu_mk("src", false)
        assert_nil FileUtils.cp("src", "c1")
        assert_nil FileUtils.copy("src", "c2")
        assert_nil FileUtils.cp_r("src", "c3")
        assert_nil FileUtils.copy_file("src", "c4")
        assert_nil FileUtils.install("src", "c5")
        assert_nil FileUtils.cp("src", "c6", noop: true)
        fu_mk("dir", true)
        assert_equal ["src"], FileUtils.cp(["src"], "dir")
        assert_equal [Pathname("src")], FileUtils.cp_r([Pathname("src")], "dir")
      end
    end

    def test_mv_and_ln_return_zero_for_one_path_and_the_list_for_an_array
      fu_in do
        fu_mk("src", false)
        assert_equal 0, FileUtils.ln("src", "h1")
        assert_equal 0, FileUtils.link("src", "h2")
        assert_equal 0, FileUtils.ln_s("src", "s1")
        assert_equal 0, FileUtils.ln_sf("src", "s1")
        assert_equal 0, FileUtils.symlink("src", "s2")
        assert_equal 0, FileUtils.mv("h1", "m1")
        assert_equal 0, FileUtils.move("h2", "m2")
        fu_mk("dir", true)
        assert_equal 0, FileUtils.mv("m1", "dir")
        assert_equal ["m2"], FileUtils.mv(["m2"], "dir")
        fu_mk("dir2", true)
        assert_equal [Pathname("src")], FileUtils.ln_s([Pathname("src")], "dir2")
        assert_equal ["src"], FileUtils.ln(["src"], "dir", force: true)
        assert_nil FileUtils.mv("src", "n", noop: true)
        assert_nil FileUtils.ln_s("src", "n", noop: true)
        assert_nil FileUtils.ln("src", "n", noop: true)
      end
    end

    def test_cd_and_pwd
      Dir.mktmpdir do |d|
        old = Dir.pwd
        begin
          assert_equal [:v, true], [FileUtils.cd(d) { :v }, Dir.pwd == old]
          assert_equal [7, true], [FileUtils.chdir(d) { 7 }, Dir.pwd == old]
          assert_equal 0, FileUtils.cd(d)
          assert_equal [false, true, true], [Dir.pwd == old, FileUtils.pwd == Dir.pwd, FileUtils.getwd == Dir.pwd]
        ensure
          Dir.chdir(old)
        end
      end
    end
  end

  # unlink(2) refuses a directory (EPERM on macOS, EISDIR on Linux); File.delete used to remove an empty one.
  class DeleteDirectoryTest < Minitest::Test
    #: () { () -> void } -> Array[untyped]
    def fu_failure
      yield
      [] #: Array[untyped]
    rescue SystemCallError => e
      [e.class, e.message.split(" @ ").last]
    end

    def test_file_delete_and_unlink_refuse_a_directory
      Dir.mktmpdir do |d|
        old = Dir.pwd
        Dir.chdir(d)
        begin
          Dir.mkdir("sub")
          klass = RUBY_PLATFORM.include?("darwin") ? Errno::EPERM : Errno::EISDIR
          want = [klass, "apply2files - sub"]
          assert_equal [want, want, want], [fu_failure { File.delete("sub") }, fu_failure { File.unlink("sub") }, fu_failure { FileUtils.rm("sub") }]
          assert_equal([], fu_failure { FileUtils.rm_f("sub") })
          assert File.directory?("sub")
          File.write("f", "x")
          assert_equal 1, File.delete("f")
        ensure
          Dir.chdir(old)
        end
      end
    end
  end

  class CpRKeywordsTest < Minitest::Test
    #: () { (String) -> void } -> void
    def fu_in
      Dir.mktmpdir do |d|
        old = Dir.pwd
        Dir.chdir(d)
        begin
          yield d
        ensure
          Dir.chdir(old)
        end
      end
    end

    #: () { () -> void } -> untyped
    def fu_raised
      yield
      nil
    rescue SystemCallError => e
      e.class
    end

    def test_remove_destination_replaces_a_symlink_instead_of_writing_through
      fu_in do
        File.write("src", "new")
        File.write("real", "old")
        File.symlink("real", "link")
        FileUtils.cp_r("src", "link", remove_destination: true)
        assert_equal [false, "new", "old"], [File.symlink?("link"), File.read("link"), File.read("real")]
        File.delete("link")
        File.symlink("real", "link")
        FileUtils.cp_r("src", "link")
        assert_equal [true, "new"], [File.symlink?("link"), File.read("real")]
      end
    end

    def test_remove_destination_replaces_a_file_that_is_in_the_way_of_a_directory
      fu_in do
        Dir.mkdir("tree")
        File.write("tree/a", "A")
        File.write("out", "file")
        err = fu_raised { FileUtils.cp_r("tree", "out") }
        assert_equal [Errno::EEXIST, "file"], [err, File.read("out")]
        FileUtils.cp_r("tree", "out", remove_destination: true)
        assert_equal [true, "A"], [File.directory?("out"), File.read("out/a")]
      end
    end

    def test_remove_destination_keeps_an_existing_directory
      fu_in do
        Dir.mkdir("tree")
        File.write("tree/a", "A")
        FileUtils.mkdir_p("out/tree")
        File.write("out/tree/a", "old")
        File.write("out/tree/keep", "K")
        FileUtils.cp_r("tree", "out", remove_destination: true)
        assert_equal ["A", "K"], [File.read("out/tree/a"), File.read("out/tree/keep")]
      end
    end

    def test_remove_destination_verbose_message
      out, = capture_io { FileUtils.cp_r("a", "b", noop: true, verbose: true, remove_destination: true) }
      assert_equal "cp -r --remove-destination a b\n", out
    end

    def test_dereference_root_follows_a_source_symlink
      fu_in do
        Dir.mkdir("real")
        File.write("real/a", "A")
        File.symlink("real", "link")
        FileUtils.cp_r("link", "deref")
        FileUtils.cp_r("link", "deref2", dereference_root: true)
        FileUtils.cp_r("link", "keep", dereference_root: false)
        assert_equal [false, true, "A", false], [File.symlink?("deref"), File.directory?("deref"), File.read("deref/a"), File.symlink?("deref2")]
        assert_equal [true, "real"], [File.symlink?("keep"), File.readlink("keep")]
      end
    end

    def test_dereference_root_for_a_file_symlink
      fu_in do
        File.write("real", "R")
        File.symlink("real", "link")
        FileUtils.cp_r("link", "copy")
        FileUtils.cp_r("link", "keep", dereference_root: false)
        assert_equal [false, "R", true, "real"], [File.symlink?("copy"), File.read("copy"), File.symlink?("keep"), File.readlink("keep")]
      end
    end

    def test_dereference_root_false_into_an_existing_destination
      fu_in do
        File.write("real", "R")
        File.symlink("real", "link")
        File.write("out", "old")
        assert_equal(Errno::EEXIST, fu_raised { FileUtils.cp_r("link", "out", dereference_root: false) })
        FileUtils.cp_r("link", "out", dereference_root: false, remove_destination: true)
        assert_equal [true, "R"], [File.symlink?("out"), File.read("real")]
      end
    end

    def test_missing_source_message
      fu_in do
        msgs = [] #: Array[String]
        [true, false].each do |deref|
          FileUtils.cp_r("missing", "x", dereference_root: deref)
        rescue SystemCallError => e
          msgs << "#{e.class} #{e.message}"
        end
        assert_equal ["Errno::ENOENT No such file or directory @ rb_check_realpath_internal - missing",
                      "Errno::ENOENT No such file or directory @ rb_file_s_lstat - missing"], msgs
      end
    end
  end
end
