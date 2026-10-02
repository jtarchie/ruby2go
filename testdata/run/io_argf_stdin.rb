# rbs_inline: enabled
# stdin: "x\ny\nz\n"

# IO#lineno counts $stdin.gets (ruby/spec core/io, #49)
p $stdin.gets, $stdin.lineno
$stdin.lineno = 5
p $stdin.lineno

# With ARGV empty at the first read, ARGF is stdin, named "-".
p gets
p ARGF.filename
p ARGF.readlines
p gets
