# skip: a `rescue => err` binding read after the begin/end fails to compile (undefined local err); in Ruby it is a method local, nil when nothing was rescued

# rbs_inline: enabled

#: (bool) -> void
def check(fail)
  begin
    raise ArgumentError, "bad" if fail
  rescue ArgumentError => err
    puts "rescued"
  end
  puts err.inspect
end
check(true)
check(false)
