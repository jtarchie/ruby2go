# FFI: call C functions in a shared library without writing a C extension, through the ffi gem's API.
require "ffi"

# A Library module binds C functions as its own methods; the argument and return types are Symbols.
module LibC
  extend FFI::Library
  ffi_lib FFI::Library::LIBC

  typedef :ulong, :count_t
  attach_function :strlen, [:string], :size_t
  attach_function :abs, [:int], :int
  attach_function :pow, %i[double double], :double
  attach_function :toupper, [:int], :int
  attach_function :memset, %i[pointer int count_t], :pointer
  attach_function :getenv, [:string], :string

  # qsort calls back into Ruby: a callback type is a Proc that C can call.
  callback :compare, %i[pointer pointer], :int
  attach_function :qsort, %i[pointer size_t size_t compare], :void

  # struct timeval, filled in by gettimeofday.
  class Timeval < FFI::Struct
    layout :tv_sec, :long, :tv_usec, :int
  end
  attach_function :gettimeofday, [Timeval.by_ref, :pointer], :int
end

puts "strlen(\"hello\") = #{LibC.strlen("hello")}"
puts "abs(-42) = #{LibC.abs(-42)}"
puts "pow(2, 10) = #{LibC.pow(2.0, 10.0)}"
puts "toupper('q') = #{LibC.toupper("q".ord).chr}"
puts "a missing variable is nil: #{LibC.getenv("RUGAROO_SURELY_UNSET").inspect}"

# Memory C can read and write.
buf = FFI::MemoryPointer.new(:int, 5)
buf.write_array_of_int([50, 10, 40, 20, 30])
compare = ->(a, b) { a.read_int <=> b.read_int } #: ^(FFI::Pointer, FFI::Pointer) -> Integer
LibC.qsort(buf, 5, 4, compare)
puts "sorted: #{buf.read_array_of_int(5).inspect}"
LibC.memset(buf, 0, 20)
puts "zeroed: #{buf.read_array_of_int(5).inspect}"

str = FFI::MemoryPointer.from_string("pointer")
puts "string: #{str.read_string}, from offset 3: #{(str + 3).read_string}"

# A struct lives in C memory; its fields read and write that memory.
tv = LibC::Timeval.new
puts "gettimeofday returned #{LibC.gettimeofday(tv, nil)}"
puts "seconds are recent: #{tv[:tv_sec] > 1_700_000_000}"
puts "Timeval is #{LibC::Timeval.size} bytes; tv_usec at offset #{LibC::Timeval.offset_of(:tv_usec)}"
tv[:tv_usec] = 123
puts "tv_usec = #{tv[:tv_usec]}"

begin
  Class.new { extend FFI::Library }.ffi_lib("/nonexistent/libnope.so")
rescue LoadError
  puts "a missing library raises LoadError"
end
