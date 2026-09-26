# skip: a `when` value holding a Regexp that is not statically Regexp (untyped, Regexp?) is compared with == instead of ===, so it never matches

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
