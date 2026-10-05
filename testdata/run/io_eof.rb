# rbs_inline: enabled
# stdin: "one\r\ntwo\nxé"

# $stdin's readers raise EOFError at end of input (#43)
p $stdin.readline(chomp: true)
p $stdin.readline
p $stdin.readchar, $stdin.readbyte
bytes = [] #: Array[Integer]
$stdin.each_byte { |b| bytes << b }
p bytes
p $stdin.getc, $stdin.getbyte
begin
  $stdin.readline
rescue EOFError => e
  p e, e.class.superclass
end
begin
  $stdin.readchar
rescue EOFError => e
  p e.message
end
begin
  $stdin.readbyte
rescue IOError => e
  p e.class
end
p STDIN.readline(chomp: true)
