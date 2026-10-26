# load_path: lib
require "traced_cask"
c = TracedCaskLoader.new(File.join(__dir__, "lib", "casks", "demo.rb")).load
p [c.token, c.ver, c.run_post]
