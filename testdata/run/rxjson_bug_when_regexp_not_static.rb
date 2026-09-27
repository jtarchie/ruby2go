# rbs_inline: enabled

ur = /x/ #: untyped
opt = /d/ #: Regexp?
["xy", "ad", "zz"].each do |s|
  r = case s
      when ur then "untyped"
      when opt then "optional"
      else "none"
      end
  puts r
end
