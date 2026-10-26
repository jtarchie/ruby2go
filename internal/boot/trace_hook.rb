# rugaroo -trace (decision 296): requires nothing itself, so the program's load list is its own.
$__rugaroo_trace_started = []
$__rugaroo_trace_evals = {} # "caller path:line\teval's file\tbase64 source" of each run-time instance_eval of a String
$__rugaroo_trace_pid = $$
TracePoint.new(:script_compiled) do |tp|
  if tp.eval_script.nil?
    $__rugaroo_trace_started << tp.instruction_sequence.path
  elsif caller_locations(1, 1)&.first&.label == "BasicObject#instance_eval" # the only kind compiled where it runs (decision 374)
    $__rugaroo_trace_evals["#{tp.path}:#{tp.lineno}\t#{tp.instruction_sequence.path}\t#{[tp.eval_script].pack("m0")}"] = true
  end
end.enable

at_exit do
  out = ENV.fetch("RUGAROO_TRACE_OUT", nil)
  next unless out && $$ == $__rugaroo_trace_pid # a forked child exits through here too

  loaded = ($__rugaroo_trace_started & $LOADED_FEATURES) + ($LOADED_FEATURES - $__rugaroo_trace_started)
  lines = loaded.map { |f| "F\t#{f}" }
  lines.concat($LOAD_PATH.map { |d| "L\t#{d}" })
  lines.concat($__rugaroo_trace_evals.keys.map { |e| "E\t#{e}" })
  require "rbconfig" # after the list: a program that never loaded it still has not
  lines.concat([RbConfig::CONFIG["rubylibdir"], RbConfig::CONFIG["archdir"]].map { |d| "S\t#{d}" })
  File.write(out, lines.join("\n") + "\n")
end
