# rbs_inline: enabled
# stdin: "x\ny\nz\n"

# With ARGV empty at the first read, ARGF is stdin, named "-".
p gets
p ARGF.filename
p ARGF.readlines
p gets
