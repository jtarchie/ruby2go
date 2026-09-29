# rbs_inline: enabled
# args: --seed 1
require "tmpdir"
require "minitest/autorun"

# File and Dir: read a fixture next to the program, write and reread files
# in a temp dir, and rescue the Errno classes MRI raises.

#: (String) -> Hash[String, Integer]
def load_scores(path)
  scores = {} #: Hash[String, Integer]
  File.foreach(path) do |line|
    name, score = line.chomp.split(",")
    next if name.nil?
    next if name == "name"

    scores[name] = (score || "0").to_i
  end
  scores
end

class FixtureTest < Minitest::Test
  def test_foreach_reads_the_fixture_line_by_line
    scores = load_scores("scores.csv")
    assert_equal [["alice", 90], ["carol", 85], ["bob", 72]], scores.sort_by { |_, s| -s }
  end

  def test_file_queries
    assert_equal 4, File.readlines("scores.csv").size
    assert File.exist?("scores.csv")
    assert File.file?("scores.csv")
    refute File.directory?("scores.csv")
    assert_equal 36, File.size("scores.csv")
  end
end

class PathTest < Minitest::Test
  def test_basename_dirname_extname
    parts = %w[a/b.rb /usr/lib/ report.tar.gz .profile x.].map do |p|
      [File.basename(p), File.basename(p, ".*"), File.dirname(p), File.extname(p)]
    end
    assert_equal [
      ["b.rb", "b", "a", ".rb"],
      ["lib", "lib", "/usr", ""],
      ["report.tar.gz", "report.tar", ".", ".gz"],
      [".profile", ".profile", ".", ""],
      ["x.", "x", ".", "."],
    ], parts
  end

  def test_join_and_expand_path
    assert_equal "logs/2026/app.log", File.join("logs", "2026", "app.log")
    assert_equal "root/leaf", File.join("root/", "/leaf")
    assert File.absolute_path?(File.expand_path("scores.csv"))
    assert_equal File.join(Dir.pwd, "scores.csv"), File.expand_path("scores.csv")
  end
end

class TempDirTest < Minitest::Test
  def test_write_append_and_read_back
    scores = load_scores("scores.csv")
    Dir.mktmpdir do |dir|
      report = File.join(dir, "report.txt")
      assert_equal 10, File.write(report, "total #{scores.values.sum}\n")

      File.open(report, "a") do |f|
        f.puts "best #{scores.max_by { |_, s| s }&.first}"
        f.print "count ", scores.size, "\n"
        f << "done" << "\n"
        f.printf("%05.1f\n", 72.25)
      end
      assert_equal "total 247\nbest alice\ncount 3\ndone\n072.2\n", File.read(report)

      first = File.open(report) do |f|
        f.gets
      end
      assert_equal "total 247\n", first

      File.open(report) do |f|
        lines = [] #: Array[String]
        f.each_line { |l| lines << l }
        assert_equal ["total 247\n", "best alice\n", "count 3\n", "done\n", "072.2\n"], lines
        assert f.eof?
        assert_nil f.gets
      end
    end
  end

  def test_directories_rename_and_glob
    Dir.mktmpdir do |dir|
      File.write(File.join(dir, "report.txt"), "r")
      Dir.mkdir(File.join(dir, "sub"))
      File.write(File.join(dir, "sub", "x.txt"), "x")
      File.rename(File.join(dir, "report.txt"), File.join(dir, "final.txt"))
      assert_equal ["final.txt", "sub"], Dir.children(dir).sort
      assert_equal [".", "..", "final.txt", "sub"], Dir.entries(dir).sort
      assert_equal ["final.txt"], Dir.glob(File.join(dir, "*.txt")).map { |f| File.basename(f) }
      assert Dir.exist?(File.join(dir, "sub"))

      assert_raises(Errno::ENOTEMPTY) { Dir.rmdir(File.join(dir, "sub")) }
      File.delete(File.join(dir, "sub", "x.txt"))
      Dir.rmdir(File.join(dir, "sub"))
      assert_equal ["final.txt"], Dir.children(dir)

      assert_raises(Errno::EEXIST) { Dir.mkdir(dir) }
    end
  end
end

class ErrnoTest < Minitest::Test
  def test_missing_file
    e = assert_raises(Errno::ENOENT) { File.read("missing.txt") }
    assert_equal "No such file or directory @ rb_sysopen - missing.txt", e.message
  end

  def test_reading_a_directory
    e = assert_raises(Errno::EISDIR) { File.read(".") }
    assert_equal "Is a directory @ io_fread - .", e.message
  end

  def test_writing_a_read_only_handle
    e = assert_raises(IOError) { File.open("scores.csv") { |f| f.write("nope") } }
    assert_equal "not opened for writing", e.message
  end

  def test_missing_directory
    e = assert_raises(SystemCallError) { Dir.children("no_such_dir") }
    assert_equal "No such file or directory @ dir_initialize - no_such_dir", e.message
  end
end
