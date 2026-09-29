# rbs_inline: enabled
# args: --seed 1
require "fileutils"
require "tempfile"
require "tmpdir"
require "minitest/autorun"

# FileUtils and Tempfile, each test built and torn down entirely under its own
# Dir.mktmpdir so nothing here touches real paths or depends on test order.

class FileUtilsTest < Minitest::Test
  def test_mkdir_p_nests_is_idempotent_and_takes_arrays
    Dir.mktmpdir do |dir|
      FileUtils.mkdir_p(File.join(dir, "a/b/c"))
      assert Dir.exist?(File.join(dir, "a/b/c"))

      FileUtils.mkdir_p(dir) # already exists: no error

      FileUtils.mkdir_p([File.join(dir, "x"), File.join(dir, "y")])
      assert Dir.exist?(File.join(dir, "x"))
      assert Dir.exist?(File.join(dir, "y"))
    end
  end

  def test_touch_creates_files
    Dir.mktmpdir do |dir|
      FileUtils.touch(File.join(dir, "f.txt"))
      assert File.exist?(File.join(dir, "f.txt"))

      FileUtils.mkdir_p(File.join(dir, "x"))
      FileUtils.touch([File.join(dir, "x/g1.txt"), File.join(dir, "x/g2.txt")])
      assert_equal ["g1.txt", "g2.txt"], Dir.children(File.join(dir, "x")).sort
    end
  end

  # cp into a file, then into a directory (MRI: dest/basename(src))
  def test_cp_to_a_file_and_into_a_directory
    Dir.mktmpdir do |dir|
      FileUtils.mkdir_p([File.join(dir, "x"), File.join(dir, "y")])
      FileUtils.touch([File.join(dir, "f.txt"), File.join(dir, "x/g1.txt"), File.join(dir, "x/g2.txt")])

      FileUtils.cp(File.join(dir, "f.txt"), File.join(dir, "f2.txt"))
      assert File.exist?(File.join(dir, "f2.txt"))

      FileUtils.cp(File.join(dir, "f.txt"), File.join(dir, "y"))
      assert File.exist?(File.join(dir, "y/f.txt"))

      FileUtils.cp([File.join(dir, "x/g1.txt"), File.join(dir, "x/g2.txt")], File.join(dir, "y"))
      assert_equal ["f.txt", "g1.txt", "g2.txt"], Dir.children(File.join(dir, "y")).sort
    end
  end

  # cp_r copies a tree; mv renames in place, then moves into an existing directory
  def test_cp_r_and_mv
    Dir.mktmpdir do |dir|
      FileUtils.mkdir_p(File.join(dir, "a/b/c"))
      FileUtils.touch(File.join(dir, "a/b/c/f.txt"))

      FileUtils.cp_r(File.join(dir, "a"), File.join(dir, "acopy"))
      assert Dir.exist?(File.join(dir, "acopy/b/c"))
      assert File.exist?(File.join(dir, "acopy/b/c/f.txt"))

      FileUtils.mv(File.join(dir, "acopy"), File.join(dir, "amoved"))
      refute Dir.exist?(File.join(dir, "acopy"))
      assert Dir.exist?(File.join(dir, "amoved"))

      FileUtils.mkdir_p(File.join(dir, "movedest"))
      FileUtils.mv(File.join(dir, "amoved"), File.join(dir, "movedest"))
      assert Dir.exist?(File.join(dir, "movedest/amoved"))
    end
  end

  # ln_s: symlink to a file, and into a directory
  def test_ln_s
    Dir.mktmpdir do |dir|
      FileUtils.touch(File.join(dir, "f.txt"))
      FileUtils.ln_s(File.join(dir, "f.txt"), File.join(dir, "link1"))
      assert File.symlink?(File.join(dir, "link1"))

      FileUtils.mkdir_p(File.join(dir, "linkdest"))
      FileUtils.ln_s(File.join(dir, "f.txt"), File.join(dir, "linkdest"))
      assert File.symlink?(File.join(dir, "linkdest/f.txt"))
    end
  end

  def test_rm_rm_f_and_rm_rf
    Dir.mktmpdir do |dir|
      FileUtils.rm_f(File.join(dir, "does_not_exist.txt"))
      assert_raises(Errno::ENOENT) { FileUtils.rm(File.join(dir, "does_not_exist.txt")) }

      FileUtils.touch(File.join(dir, "f2.txt"))
      FileUtils.rm(File.join(dir, "f2.txt"))
      refute File.exist?(File.join(dir, "f2.txt"))

      FileUtils.mkdir_p(File.join(dir, "x"))
      FileUtils.touch([File.join(dir, "x/g1.txt"), File.join(dir, "x/g2.txt")])
      FileUtils.rm([File.join(dir, "x/g1.txt"), File.join(dir, "x/g2.txt")])
      assert_equal [], Dir.children(File.join(dir, "x"))

      FileUtils.mkdir_p(File.join(dir, "a/b/c"))
      FileUtils.rm_rf(File.join(dir, "a"))
      refute Dir.exist?(File.join(dir, "a"))
      FileUtils.rm_rf(File.join(dir, "a")) # already gone: no error
    end
  end
end

class TempfileTest < Minitest::Test
  # Tempfile.new: object-style, explicit close/unlink
  def test_new_close_unlink
    t = Tempfile.new("scratch-")
    tpath = t.path.to_s
    assert File.basename(tpath).start_with?("scratch-")
    t.write("hello tempfile\n")
    t.close
    assert_equal "hello tempfile\n", File.read(tpath)
    t.unlink
    assert_nil t.path
    refute File.exist?(tpath)
  end

  # Tempfile.create: block form, auto-unlinked at block exit, yields a File
  def test_create_block_cleans_up
    created_path = nil #: String?
    body = nil #: String?
    Tempfile.create("scratch-block-") do |f|
      created_path = f.path
      f.write("block body\n")
      f.close
      body = File.read(f.path)
    end
    assert_equal "block body\n", body
    refute File.exist?(created_path.to_s)
  end
end
