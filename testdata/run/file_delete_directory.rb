require "fileutils"
require "tmpdir"

# The program never names Errno::EPERM: the compiler still has to load it for File.delete of a directory (macOS)
def report
  yield
rescue SystemCallError => e
  puts "#{e.class}: #{e.message}"
end

Dir.mktmpdir do |d|
  Dir.chdir(d) do
    Dir.mkdir("sub")
    report { File.delete("sub") }
    report { File.unlink("sub") }
    report { FileUtils.rm("sub") }
    puts File.directory?("sub")
  end
end
