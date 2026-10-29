# rbs_inline: enabled

require "English"
require "minitest/autorun"
require "tmpdir"
require "fileutils"
require "find"
require "weakref"

# Runtime system helpers (#101): spawn/popen/Find/Dir.glob hardening, compared with MRI.
module ProcessTests
  #: () -> Integer
  def self.open_fds
    Dir.children(File.exist?("/proc/self/fd") ? "/proc/self/fd" : "/dev/fd").size
  end

  class SpawnTest < Minitest::Test
    def test_popen_empty_command_array
      msgs = [] #: Array[String]
      cmds = [Array.new, [{ "A" => "b" }]] #: Array[Array[untyped]]
      cmds.each do |cmd|
        IO.popen(cmd, &:read)
        msgs << "no error"
      rescue ArgumentError => e
        msgs << e.message
      end
      assert_equal ["wrong number of arguments (given 0, expected 1+)"] * 2, msgs
    end

    # Redirected paths were opened and never closed, one descriptor per spawn.
    def test_spawn_redirect_closes_descriptors
      Dir.mktmpdir do |d|
        target = File.join(d, "out")
        Process.wait(Process.spawn({}, "true", out: File::NULL, err: target, in: File::NULL))
        before = ProcessTests.open_fds
        200.times do
          Process.wait(Process.spawn({}, "true", out: File::NULL, err: target, in: File::NULL))
        end
        assert_operator ProcessTests.open_fds - before, :<, 10
      end
    end

    # A trailing options Hash with a descriptor number (Homebrew's sandbox inheritance fd), as SystemCommand#exec3 passes it
    def test_spawn_trailing_options_with_fd
      Dir.mktmpdir do |d|
        src = File.join(d, "src")
        out = File.join(d, "out")
        File.write(src, "inherited")
        File.open(src) do |f|
          opts = { 9 => f, out: out } #: Hash[untyped, untyped]
          Process.wait(Process.spawn({}, %w[sh sh], "-c", "cat <&9", opts))
        end
        assert_equal "inherited", File.read(out)
      end
    end

    def test_popen_redirect_closes_descriptors
      before = ProcessTests.open_fds
      100.times do
        IO.popen(["true"], "r", { err: File::NULL }, &:read)
      end
      assert_operator ProcessTests.open_fds - before, :<, 10
    end

    def test_signalled_child_status
      system("kill -9 $$")
      st = $CHILD_STATUS
      assert_equal [true, 9, nil, nil, false], [st&.signaled?, st&.termsig, st&.exitstatus, st&.success?, st&.exited?]
      assert_equal "pid N SIGKILL (signal 9)", st&.to_s&.sub(/\d+/, "N")
      assert_equal 9, st&.to_i

      Process.wait(Process.spawn("sh", "-c", "kill -15 $$"))
      st = $CHILD_STATUS
      assert_equal [true, 15, nil], [st&.signaled?, st&.termsig, st&.exitstatus]
    end

    def test_exited_child_status
      system("exit 3")
      st = $CHILD_STATUS
      assert_equal [false, nil, 3, false, true], [st&.signaled?, st&.termsig, st&.exitstatus, st&.success?, st&.exited?]
      assert_equal 768, st&.to_i
    end

    def test_popen_signalled_status
      IO.popen(["sh", "-c", "kill -9 $$"], &:read)
      st = $CHILD_STATUS
      assert_equal [true, 9, nil], [st&.signaled?, st&.termsig, st&.exitstatus]
    end

    def test_spawn_not_executable_is_eacces
      Dir.mktmpdir do |d|
        f = File.join(d, "plain")
        File.write(f, "echo hi\n")
        File.chmod(0o644, f)
        got = [] #: Array[String]
        [-> { Process.spawn(f) }, -> { Process.spawn({}, f) }, -> { Process.spawn(File.join(d, "missing")) }].each do |go|
          go.call
        rescue SystemCallError => e
          got << "#{e.class}: #{e.message.gsub(d, "D")}"
        end
        assert_equal ["Errno::EACCES: Permission denied - D/plain"] * 2 + ["Errno::ENOENT: No such file or directory - D/missing"], got
      end
    end
  end

  class FindTest < Minitest::Test
    #: (String, String?) -> Array[String]
    def walk(root, prune)
      seen = [] #: Array[String]
      Find.find(root) do |path|
        seen << path.delete_prefix(root)
        Find.prune if prune && path.end_with?(prune)
      end
      seen
    end

    #: (String, String) -> String
    def mktree(d, name)
      root = File.join(d, name)
      FileUtils.mkdir_p(File.join(root, "a/deep"))
      FileUtils.mkdir_p(File.join(root, "b"))
      %w[a/1 a/deep/2 b/3 top].each { |f| File.write(File.join(root, f), "") }
      root
    end

    def test_prune_skips_subtree
      Dir.mktmpdir do |d|
        root = mktree(d, "t")
        assert_equal ["", "/a", "/b", "/b/3", "/top"], walk(root, "/a")
        assert_equal ["", "/a", "/a/1", "/a/deep", "/a/deep/2", "/b", "/b/3", "/top"], walk(root, nil)
      end
    end

    # The prune flag was one global: a prune in one thread leaked into the other.
    def test_prune_is_per_thread
      Dir.mktmpdir do |d|
        r1 = mktree(d, "one")
        r2 = mktree(d, "two")
        one = Thread.new { walk(r1, "/a") }
        two = Thread.new { walk(r2, nil) }
        assert_equal ["", "/a", "/b", "/b/3", "/top"], one.value
        assert_equal ["", "/a", "/a/1", "/a/deep", "/a/deep/2", "/b", "/b/3", "/top"], two.value
      end
    end

    def test_prune_outside_find
      Find.prune
      flunk "no throw"
    rescue UncaughtThrowError => e
      assert_equal "uncaught throw :prune", e.message
    end

    def test_prune_in_nested_find_hits_innermost
      Dir.mktmpdir do |d|
        r1 = mktree(d, "one")
        r2 = mktree(d, "two")
        seen = [] #: Array[String]
        Find.find(r1) do |p|
          seen << p.delete_prefix(r1)
          next unless p == File.join(r1, "a")

          Find.find(r2) { |q| Find.prune if q.end_with?("/a") }
        end
        assert_equal ["", "/a", "/a/1", "/a/deep", "/a/deep/2", "/b", "/b/3", "/top"], seen
      end
    end
  end

  #: (bool) -> WeakRef[Thread]
  def self.finished_thread(fiber)
    t = Thread.new { fiber ? Fiber.current.alive? : true }
    t.join
    WeakRef.new(t)
  end

  class LifetimeTest < Minitest::Test
    # Fiber.current in a thread pinned that Thread in a global map for good.
    def test_finished_thread_is_collectable_after_fiber_current
      ProcessTests.finished_thread(false)
      plain = ProcessTests.finished_thread(false)
      used = ProcessTests.finished_thread(true)
      3.times { GC.start }
      assert_nil plain.weakref_alive?
      assert_nil used.weakref_alive?
    end

    # The pin list was appended to from any thread without a lock.
    def test_autoclose_false_from_many_threads
      pipes = Array.new(16) { IO.pipe }
      ts = pipes.map do |r, _w|
        Thread.new do
          50.times { r.autoclose = false }
          true
        end
      end
      ts.each(&:join)
      assert_equal([false] * 16, pipes.map { |r, _w| r.autoclose? })
      pipes.each do |r, w|
        w.close
        r.close
      end
    end
  end

  class GlobTest < Minitest::Test
    def test_double_star_does_not_follow_symlinks
      Dir.mktmpdir do |d|
        Dir.mkdir(File.join(d, "a"))
        Dir.mkdir(File.join(d, "b"))
        File.write(File.join(d, "a/f.rb"), "")
        File.write(File.join(d, "b/x"), "")
        File.symlink(".", File.join(d, "loop"))
        File.symlink("a", File.join(d, "lnk"))
        Dir.chdir(d) do
          assert_equal ["a/", "b/"], Dir.glob("**/")
          assert_equal ["a", "a/f.rb", "b", "b/x", "lnk", "loop"], Dir.glob("**/*")
          assert_equal ["a/f.rb"], Dir.glob("**/*.rb")
          assert_equal ["a/", "b/", "lnk/", "loop/"], Dir.glob("*/")
        end
      end
    end
  end
end
