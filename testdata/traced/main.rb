# load_path: lib
# env: TRACED_SPAWN=1
# env: TRACED_LATE=1
require_relative "bundle/bundler/setup" # its $LOAD_PATH edits are the trace's; it compiles to nothing (decision 296)
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
p TracedReq.late_lib
p TracedReq.std_lib
p TRACED_LIB
p TracedReq.arch_lib
if ENV["TRACED_LATE"] # unset in TestTraceRun's trace: compiled from main's own require, not from the run
  require "traced_late"
  p TracedLate.name
end
