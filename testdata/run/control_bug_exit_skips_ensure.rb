# skip: Kernel#exit is os.Exit, so ensure blocks (and `rescue Exception`, which sees SystemExit in MRI) do not run

# rbs_inline: enabled

#: () -> void
def work
  puts "working"
  exit 2
ensure
  puts "method ensure runs on exit"
end

begin
  work
ensure
  puts "outer ensure runs on exit"
end
