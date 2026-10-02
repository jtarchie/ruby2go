# DATA reads the text after __END__ (ruby/spec language/predefined, #49)
DATA.each_line { |l| puts l.upcase }
p DATA.read
__END__
alpha
beta
