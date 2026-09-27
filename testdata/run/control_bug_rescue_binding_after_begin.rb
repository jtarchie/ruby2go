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
