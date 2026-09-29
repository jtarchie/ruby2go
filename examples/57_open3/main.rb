# rbs_inline: enabled
# args: --seed 1
require "open3"
require "minitest/autorun"

# Open3 runs commands without a shell: capture2/capture2e/capture3 collect output, popen3 streams it through a block.

class Open3Test < Minitest::Test
  def test_capture2_returns_stdout_and_status
    out, status = Open3.capture2("echo", "hello world")
    assert_equal "hello world\n", out
    assert_equal true, status.success?
    assert_equal 0, status.exitstatus
  end

  def test_capture2e_merges_stderr_into_stdout
    merged, status = Open3.capture2e("sh", "-c", "echo out; echo err 1>&2")
    assert_equal "out\nerr\n", merged
    assert_equal true, status.success?
  end

  def test_capture3_keeps_streams_apart_and_reports_the_exit_code
    out, err, status = Open3.capture3("sh", "-c", "echo out; echo err 1>&2; exit 3")
    assert_equal "out\n", out
    assert_equal "err\n", err
    assert_equal 3, status.exitstatus
    assert_equal false, status.success?
  end

  def test_popen3_streams_through_a_block
    checks = [] #: Array[bool]
    data, err_data, code = Open3.popen3("cat") do |stdin, stdout, stderr, wait_thr|
      stdin.puts "piped"
      stdin.close
      checks << stdin.closed?
      out = stdout.read
      checks << stdout.eof?
      errd = stderr.read
      checks << (wait_thr.pid > 0)
      [out, errd, wait_thr.value.exitstatus]
    end
    assert_equal [true, true, true], checks
    assert_equal "piped\n", data
    assert_equal true, err_data.empty?
    assert_equal 0, code
  end

  def test_a_missing_command_raises_enoent
    e = assert_raises(Errno::ENOENT) { Open3.capture2("rb2go_missing_command_xyz") }
    assert_equal "No such file or directory - rb2go_missing_command_xyz", e.message
  end
end
