# rbs_inline: enabled

class AppError < StandardError; end
class NotFound < AppError; end
class Fatal < Exception; end

class ValidationError < StandardError
  attr_reader :field #: String

  #: (String, String) -> void
  def initialize(field, msg)
    super(msg)
    @field = field
  end
end

class DefaultMsg < StandardError
  #: (?String) -> void
  def initialize(msg = "default message")
    super(msg)
  end
end

#: (Integer) -> void
def boom(kind)
  case kind
  when 0 then raise ArgumentError, "bad arg"
  when 1 then raise "plain"
  when 2 then raise NotFound
  when 3 then raise NotFound.new("built")
  when 4 then raise KeyError, "k"
  when 5 then raise AppError.new
  when 6 then raise ValidationError.new("name", "too short")
  when 7 then raise DefaultMsg
  when 8 then raise IndexError, ""
  when 9 then raise TypeError, "ünïcode ✓"
  end
  puts "no raise #{kind}"
end

11.times do |k|
  begin
    boom(k)
  rescue => e
    puts "#{e.class}: #{e.message.inspect} #{e.to_s.inspect}"
  end
end

#: (Integer) -> String
def which(k)
  boom(k)
  "none"
rescue NotFound => e
  "NotFound #{e.message}"
rescue AppError, KeyError => e
  "AppError|KeyError #{e.class} #{e.message}"
rescue ValidationError => e
  "#{e.field}: #{e.message}"
rescue StandardError => e
  "StandardError #{e.class}"
end
puts which(2), which(5), which(4), which(6), which(0), which(1), which(10)

begin
  raise NotFound, "nf"
rescue AppError => e
  puts "superclass clause first: #{e.class}"
rescue NotFound
  puts "never"
end

begin
  raise StopIteration, "stop"
rescue IndexError => e
  puts "index family: #{e.class} #{e.message}"
end

#: () -> String
def outer
  begin
    begin
      raise Fatal, "deep"
    rescue => e
      "inner caught #{e.class}"
    end
  rescue Exception => e
    "outer caught #{e.class} #{e.message}"
  end
end
puts outer

begin
  begin
    raise NotImplementedError, "nie"
  rescue StandardError
    puts "never"
  end
rescue ScriptError => e
  puts "script: #{e.class} #{e.message} #{e.is_a?(StandardError)}"
end

err = ArgumentError.new("stored")
begin
  raise err
rescue => e
  puts e.equal?(err), e.message, e.class.name, e.is_a?(StandardError), e.is_a?(IndexError)
end

puts ArgumentError.new.message.inspect, ArgumentError.new.inspect, ArgumentError.new.to_s
puts RuntimeError.new("ö ü").inspect, StandardError.new("x").message
puts Exception.new("e").inspect, RuntimeError.new(nil).message
puts Exception.new.backtrace.inspect

#: (bool) -> Integer
def ordered(fail)
  puts "body"
  raise "f" if fail
  1
rescue
  puts "rescue"
  2
ensure
  puts "ensure"
end
puts ordered(false), ordered(true)

#: () -> String
def ensure_value
  "body value"
ensure
  "ensure value"
end
puts ensure_value

#: (Integer) -> String
def value_if(n)
  if n > 0 then "pos" else raise ArgumentError, "nonpositive" end
rescue ArgumentError => e
  "rescued #{e.message}"
end
puts value_if(1), value_if(0)

begin
  begin
    begin
      raise ArgumentError, "x"
    ensure
      puts "e1"
    end
  ensure
    puts "e2"
  end
rescue ArgumentError => e
  puts "caught #{e.message}"
ensure
  puts "e3"
end

begin
  begin
    raise "first"
  rescue => e
    raise AppError, "second from #{e.message}"
  ensure
    puts "inner ensure"
  end
rescue AppError => e
  puts e.message
end

#: () -> Integer
def ensure_overrides
  return 1
ensure
  return 2
end
puts ensure_overrides

#: () -> String
def swallow
  raise ArgumentError, "lost"
ensure
  return "ensure wins"
end
puts swallow

#: (Integer) -> String
def ensure_after_rescue(n)
  raise ArgumentError, "a" if n > 0
  "body"
rescue ArgumentError
  raise "from rescue"
ensure
  return "ensure #{n}" if n > 1
end
puts ensure_after_rescue(0), ensure_after_rescue(2)
begin
  ensure_after_rescue(1)
rescue => e
  puts "escaped #{e.class}: #{e.message}"
end

#: (Array[Integer]) -> String
def scan(list)
  i = 0
  while i < list.size
    begin
      begin
        raise ArgumentError, "neg #{list[i]}" if (list[i] || 0) < 0
        return "found #{list[i]}" if list[i] == 7
      ensure
        puts "inner #{i}"
      end
    rescue ArgumentError => e
      return "error #{e.message}"
    end
    i += 1
  end
  "none"
end
puts scan([1, 7, 3]), scan([2, -1]), scan([])

#: (bool) -> void
def void_ensure(flag)
  begin
    return if flag
    puts "not returned"
  ensure
    puts "ensure #{flag}"
  end
  puts "after"
end
void_ensure(true)
void_ensure(false)

#: (Array[Integer]) -> Integer
def first_big(list)
  list.each do |x|
    begin
      return x if x > 10
    ensure
      puts "checked #{x}"
    end
  end
  -1
end
puts first_big([1, 20, 30]), first_big([1])

out = [] #: Array[String]
[1, 0, 2].each do |d|
  out << (10 / d).to_s
rescue ZeroDivisionError => e
  out << e.message
ensure
  out << "e#{d}"
end
puts out.inspect

#: () { () -> void } -> void
def risky
  yield
rescue StandardError => e
  puts "caught #{e.message}"
end
risky { raise "in block" }
risky { puts "no error" }

[0, 3, -3].each do |d|
  begin
    puts 10 / d, 10 % d, -7 / d
  rescue ZeroDivisionError => e
    puts "zde #{e.message} #{e.class}"
  end
end

arr = [1, 2] #: Array[Integer]
begin
  arr[-10] = 5
rescue IndexError => e
  puts "go panic as #{e.class}"
end
begin
  arr[-10] = 5
rescue => e
  puts "catch-all sees #{e.class} #{e.is_a?(StandardError)}"
end

none = nil #: Integer?
begin
  puts none + 1
rescue NoMethodError => e
  puts e.message
end
begin
  puts none.abs
rescue NameError => e
  puts "#{e.class}: #{e.message}"
end

#: (Integer) -> String
def name_of(n)
  case n
  when 1 then "one"
  else raise ArgumentError, "unknown #{n}"
  end
end
puts name_of(1)
begin
  name_of(7)
rescue ArgumentError => e
  puts e.message
end

#: (Integer?) -> Integer
def must(v)
  x = v || raise(KeyError, "missing")
  x + 1
end
puts must(1)
begin
  must(nil)
rescue KeyError => e
  puts e.message
end

#: (bool) -> Integer
def tern(c)
  c ? 10 : raise("no")
end
puts tern(true)

#: (Integer) -> Integer
def risky_int(n)
  raise ArgumentError, "neg" if n < 0
  raise NotImplementedError, "nie" if n == 99
  n * 2
end

a = risky_int(2) rescue 0
b = risky_int(-1) rescue 0
puts a, b
c = (risky_int(-5) rescue nil)
puts c.inspect
d = risky_int(-1) rescue "failed"
puts d.inspect
e2 = risky_int(3) rescue "failed"
puts e2.inspect
puts (tern(false) rescue -10)

hits = [] #: Array[String]
f = risky_int(1) rescue hits.push("f").size
f2 = risky_int(-1) rescue hits.push("f2").size
puts f, f2, hits.inspect

begin
  g = risky_int(99) rescue -1
  puts g
rescue NotImplementedError => ex
  puts "escaped modifier: #{ex.message}"
end

list = [1, 2, 3] #: Array[Integer]
puts list.map { |v| risky_int(v - 2) rescue 100 }.inspect

#: (String) -> Integer
def safe_fetch(k)
  { "a" => 1 }.fetch(k) rescue -1
end
puts safe_fetch("a"), safe_fetch("b")

y = begin
  1 / 0
rescue ZeroDivisionError
  -5
ensure
  puts "y ensure"
end
puts y
z = begin 5 end
puts z

begin
  raise IOError, "closed stream"
rescue StandardError => e
  puts "#{e.class}: #{e.message} #{e.is_a?(StandardError)}"
end
begin
  raise RangeError, "out of range"
rescue RangeError => e
  puts e.inspect, e.is_a?(IndexError)
end
