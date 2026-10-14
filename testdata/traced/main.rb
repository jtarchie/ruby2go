# load_path: lib
# env: TRACED_SPAWN=1
require "traced_req"
require "cmds/info"
dir = File.join(__dir__, "lib", "cmds")
p TracedReq.require?(File.join(dir, "info")), TracedReq.require?(File.join(dir, "nope")), TracedReq.require?("set")
p TracedReq.rel("cmds/info"), TracedReq.rel("cmds/info.rb"), TracedCmds.info
begin
  TracedReq.rel("cmds/nope")
rescue LoadError => e
  p e.message
end
p TracedReq.popen_mode
