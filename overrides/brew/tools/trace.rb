$__rb2go_started = []
TracePoint.new(:script_compiled) do |tp|
  $__rb2go_started << tp.instruction_sequence.path if tp.eval_script.nil?
end.enable
at_exit do
  File.write("/w/work/loaded.#{Process.pid}.txt", ([$0, ARGV.inspect] + ($__rb2go_started & $LOADED_FEATURES) + ($LOADED_FEATURES - $__rb2go_started)).join("\n")) rescue nil
end
