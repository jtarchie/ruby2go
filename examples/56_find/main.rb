# rbs_inline: enabled
# args: --seed 1
require "find"
require "tmpdir"
require "minitest/autorun"

# Find.find over a scratch tree: plain traversal, Find.prune, break, multiple roots, and the missing-root error.

#: (String, String) -> String
def rel(dir, path) = path.sub(dir, ".")

class FindTest < Minitest::Test
  # The scratch tree every test walks.
  #: (String) -> void
  def build_tree(dir)
    Dir.mkdir(File.join(dir, "a"))
    Dir.mkdir(File.join(dir, "a", "b"))
    Dir.mkdir(File.join(dir, "a", "c"))
    Dir.mkdir(File.join(dir, "skip_me"))
    Dir.mkdir(File.join(dir, "z"))
    File.write(File.join(dir, "a", "f1.txt"), "x")
    File.write(File.join(dir, "a", "b", "f2.txt"), "x")
    File.write(File.join(dir, "skip_me", "ignored.txt"), "x")
    File.write(File.join(dir, "z", "f3.txt"), "x")
    File.write(File.join(dir, "top.txt"), "x")
  end

  def test_plain_traversal_is_depth_first_and_sorted
    Dir.mktmpdir do |dir|
      build_tree(dir)
      seen = [] #: Array[String]
      Find.find(dir) { |path| seen << rel(dir, path) }
      assert_equal %w[. ./a ./a/b ./a/b/f2.txt ./a/c ./a/f1.txt ./skip_me ./skip_me/ignored.txt ./top.txt ./z ./z/f3.txt],
                   seen
    end
  end

  def test_prune_skips_a_subtree
    Dir.mktmpdir do |dir|
      build_tree(dir)
      seen = [] #: Array[String]
      Find.find(dir) do |path|
        if File.basename(path) == "skip_me"
          Find.prune
        else
          seen << rel(dir, path)
        end
      end
      assert_equal %w[. ./a ./a/b ./a/b/f2.txt ./a/c ./a/f1.txt ./top.txt ./z ./z/f3.txt], seen
    end
  end

  def test_break_stops_the_walk
    Dir.mktmpdir do |dir|
      build_tree(dir)
      seen = [] #: Array[String]
      Find.find(dir) do |path|
        break if File.basename(path) == "top.txt"
        seen << rel(dir, path)
      end
      assert_equal %w[. ./a ./a/b ./a/b/f2.txt ./a/c ./a/f1.txt ./skip_me ./skip_me/ignored.txt], seen
    end
  end

  def test_multiple_roots_are_walked_in_order
    Dir.mktmpdir do |dir|
      build_tree(dir)
      seen = [] #: Array[String]
      Find.find(File.join(dir, "a"), File.join(dir, "z")) { |path| seen << rel(dir, path) }
      assert_equal %w[./a ./a/b ./a/b/f2.txt ./a/c ./a/f1.txt ./z ./z/f3.txt], seen
    end
  end

  def test_missing_root_raises
    e = assert_raises(Errno::ENOENT) { Find.find("/no/such/rb2go/find/target") { |path| flunk "visited #{path}" } }
    assert_equal "No such file or directory - /no/such/rb2go/find/target", e.message
  end
end
