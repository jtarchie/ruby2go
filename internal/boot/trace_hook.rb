# rb2go -trace (decision 296): requires nothing itself, so the program's load list is its own.
$__rb2go_trace_started = []
$__rb2go_trace_pid = $$
TracePoint.new(:script_compiled) do |tp|
  $__rb2go_trace_started << tp.instruction_sequence.path if tp.eval_script.nil?
end.enable

at_exit do
  out = ENV.fetch("RB2GO_TRACE_OUT", nil)
  next unless out && $$ == $__rb2go_trace_pid # a forked child exits through here too

  loaded = ($__rb2go_trace_started & $LOADED_FEATURES) + ($LOADED_FEATURES - $__rb2go_trace_started)
  lines = loaded.map { |f| "F\t#{f}" }
  lines.concat($LOAD_PATH.map { |d| "L\t#{d}" })
  require "rbconfig" # after the list: a program that never loaded it still has not
  lines.concat([RbConfig::CONFIG["rubylibdir"], RbConfig::CONFIG["archdir"]].map { |d| "S\t#{d}" })
  File.write(out, lines.join("\n") + "\n")
end
