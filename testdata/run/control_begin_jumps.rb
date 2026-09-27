# rbs_inline: enabled

#: (Integer) -> Integer
def nested(n)
  i = 0
  while i < 10
    i += 1
    begin
      case i
      when 3
        begin
          next
        ensure
          puts "inner ensure #{i}"
        end
      when 6
        begin
          break
        rescue
          puts "no"
        end
      end
      begin
        return i * 100 if i == n
      ensure
        puts "ens #{i}"
      end
    rescue => e
      puts e.message
    end
  end
  -i
end
puts nested(4)
puts nested(8)

#: () -> String
def ens_ret
  begin
    puts "body"
  ensure
    return "from ensure"
  end
  "after"
end
puts ens_ret

#: () -> Integer
def ens_ret2
  x = 0
  [1, 2, 3].each do |v|
    begin
      x += v
    ensure
      return x if v == 2
    end
  end
  x
end
puts ens_ret2


[1, 2, 3].each do |v|
  begin
    [10, 20].each do |w|
      begin
        next if w == 10
        break if v == 2
        puts "#{v} #{w}"
      ensure
        puts "e #{v} #{w}"
      end
    end
  ensure
    puts "outer e #{v}"
  end
end

i = 0
while i < 4
  i += 1
  begin
    raise "boom #{i}" if i.even?
    puts "body #{i}"
  ensure
    next if i == 2
    break if i == 4
  end
  puts "after #{i}"
end
puts i

[1, 2, 3].each do |x|
  begin
    raise "a" if x == 2
  ensure
    next if x == 2
  end
  puts "x #{x}"
end
puts "ok"

#: (Integer) -> Integer
def tail_ret(n)
  begin
    return 5 if n == 1
    raise "x" if n == 2
    n * 10
  rescue
    return -1
  ensure
    puts "ens #{n}"
  end
end
puts tail_ret(1), tail_ret(2), tail_ret(3)

#: (Integer) -> void
def tail_void(n)
  puts "start"
  begin
    return if n == 1
    puts "mid #{n}"
  ensure
    puts "ens #{n}"
  end
end
tail_void(1)
tail_void(2)
