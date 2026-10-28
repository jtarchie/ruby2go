# Fiddle: call C functions in a shared library without writing a C extension.
require "fiddle"
require "fiddle/import"

# Importer binds C declarations as module methods; the C library the program already links is dlopen(nil).
module LibC
  extend Fiddle::Importer
  dlload Fiddle.dlopen(nil)

  typealias "size_type", "unsigned long"
  extern "size_type strlen(const char*)"
  extern "double pow(double, double)"
  extern "int abs(int)"
  extern "void *memset(void *, int, size_t)"

  Point = struct ["int x", "int y", "double weight", "char tag[4]"]
end

puts "strlen(\"hello\") = #{LibC.strlen("hello")}"
puts "pow(2, 10) = #{LibC.pow(2, 10)}"
puts "abs(-42) = #{LibC.abs(-42)}"

# A struct lives in C memory; its members read and write that memory.
pt = LibC::Point.malloc(Fiddle::RUBY_FREE)
LibC.memset(pt, 0, LibC::Point.size)
pt.x = 3
pt.y = -4
pt.weight = 0.5
pt.tag = [79, 75, 0, 0]
puts "Point is #{LibC::Point.size} bytes; weight at offset #{LibC::Point.offsetof("weight")}"
puts "point: #{pt.to_h.inspect}"

# A function by address, with explicit argument and return types.
libc = Fiddle.dlopen(nil)
toupper = Fiddle::Function.new(libc["toupper"], [Fiddle::TYPE_INT], Fiddle::TYPE_INT)
puts "toupper('q') = #{toupper.call("q".ord).chr}"

# qsort calls back into Ruby: a Closure is a C function pointer.
qsort = Fiddle::Function.new(
  libc["qsort"],
  [Fiddle::TYPE_VOIDP, Fiddle::TYPE_SIZE_T, Fiddle::TYPE_SIZE_T, Fiddle::TYPE_VOIDP],
  Fiddle::TYPE_VOID
)
compare = Fiddle::Closure::BlockCaller.new(Fiddle::TYPE_INT, [Fiddle::TYPE_VOIDP, Fiddle::TYPE_VOIDP]) do |a, b|
  a[0] <=> b[0]
end
buf = Fiddle::Pointer.malloc(6, Fiddle::RUBY_FREE)
buf[0, 6] = "fiddle"
qsort.call(buf, 6, 1, compare)
puts "sorted: #{buf[0, 6]}"

# Pointer arithmetic and C strings.
greeting = Fiddle::Pointer["hello, world"]
puts "C string: #{greeting}, from offset 7: #{(greeting + 7)}"

begin
  Fiddle.dlopen("/nonexistent/libnope.so")
rescue Fiddle::DLError
  puts "a missing library raises Fiddle::DLError"
end
