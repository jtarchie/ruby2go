# Loaded by ./hits into MRI's brew: every method called and every file:line run, for the work list.
$__calls = {}
$__lines = {}
TracePoint.new(:call) { |tp| $__calls["#{tp.path}\t#{tp.method_id}"] = true }.enable
TracePoint.new(:line) { |tp| $__lines["#{tp.path}:#{tp.lineno}"] = true }.enable
at_exit do
  dir = ENV.fetch("HOMEBREW_RB2GO_CALLTRACE_DIR", "/w/work") # ./host-hits: brew.sh passes on only HOMEBREW_*
  File.write("#{dir}/calls.txt", $__calls.keys.join("\n"))
  File.write("#{dir}/lines.txt", $__lines.keys.join("\n"))
end
