# rbs_inline: enabled
# args: --seed 1
require "pathname"
require "tmpdir"
require "minitest/autorun"

# Pathname: a fluent OO wrapper over File/Dir (decision 62), delegating path
# math and filesystem checks instead of reimplementing them (decision 65).

#: (Pathname) -> String
def parts_of(p)
  "#{p}: basename=#{p.basename} dirname=#{p.dirname} extname=#{p.extname.inspect} parent=#{p.parent}"
end

class PathMathTest < Minitest::Test
  def test_basename_dirname_extname_parent
    assert_equal "a/b/report.tar.gz: basename=report.tar.gz dirname=a/b extname=\".gz\" parent=a/b",
                 parts_of(Pathname.new("a/b/report.tar.gz"))
    assert_equal "/usr/lib/: basename=lib dirname=/usr extname=\"\" parent=/usr",
                 parts_of(Pathname.new("/usr/lib/"))
    assert_equal ".profile: basename=.profile dirname=. extname=\"\" parent=.",
                 parts_of(Pathname.new(".profile"))
    assert_equal "report.tar", Pathname.new("report.tar.gz").basename(".gz").to_s
    assert_equal "report.tar", Pathname.new("report.tar.gz").basename(".*").to_s
  end

  def test_slash_plus_and_join_build_paths
    root = Pathname("logs")
    assert_equal "logs/2026/app.log", (root / "2026" / "app.log").to_s
    assert_equal "logs/2026/app.log", (root + "2026" + "app.log").to_s
    assert_equal "logs/2026/app.log", root.join("2026", "app.log").to_s
    assert_equal "/abs", root.join("/abs").to_s
  end

  def test_equality_and_ordering
    assert_equal true, Pathname.new("/a") == Pathname.new("/a")
    assert_equal false, Pathname.new("/a") == "/a"
    assert_equal true, Pathname.new("/a").eql?(Pathname.new("/a"))
    assert_equal(-1, Pathname.new("/a") <=> Pathname.new("/b"))
    assert_equal 1, Pathname.new("/b") <=> Pathname.new("/a")
    assert_equal 0, Pathname.new("/a") <=> Pathname.new("/a")
    assert_equal ["/a", "/b"], [Pathname.new("/b"), Pathname.new("/a")].sort.map(&:to_s)
  end

  def test_each_filename
    parts = [] #: Array[String]
    Pathname.new("a/b/c.txt").each_filename { |part| parts << part }
    assert_equal ["a", "b", "c.txt"], parts
  end

  def test_relative_path_from
    deep = Pathname.new("/tmp/project/src/lib")
    other = Pathname.new("/tmp/project/build")
    assert_equal "../src/lib", deep.relative_path_from(other).to_s
    assert_equal "../../build", other.relative_path_from(deep).to_s
    assert_equal ".", deep.relative_path_from(deep).to_s
    assert_raises(ArgumentError) { Pathname.new("rel/path").relative_path_from("/abs") }
  end
end

class PathnameFilesystemTest < Minitest::Test
  def test_write_read_and_list_children
    Dir.mktmpdir do |dir|
      root = Pathname.new(dir)
      (root / "a.txt").write("alpha\n")
      (root / "b.txt").write("beta\n")
      Dir.mkdir((root / "sub").to_s)
      (root / "sub" / "c.txt").write("gamma\n")

      assert (root / "a.txt").exist?
      assert (root / "a.txt").file?
      assert (root / "sub").directory?
      assert_equal "alpha\n", (root / "a.txt").read
      assert_equal "gamma\n", (root / "sub" / "c.txt").read

      assert_equal ["a.txt", "b.txt", "sub"], root.children.map { |c| c.relative_path_from(root).to_s }.sort
      assert_equal ["a.txt", "b.txt", "sub"], root.children(false).map(&:to_s).sort

      sub = root / "sub"
      assert_equal "sub", sub.relative_path_from(root).to_s
      assert_equal "..", root.relative_path_from(sub).to_s

      refute (root / "nope.txt").exist?
    end
  end
end
