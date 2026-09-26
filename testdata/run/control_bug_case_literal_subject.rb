# skip: a case whose subject is a bare literal (`case 3`) stores it in an untyped Go temp, so go build fails (t.Eq undefined on int)

# rbs_inline: enabled

v = case 3
    when 1 then "one"
    when 3 then "three"
    end
puts v.inspect
w = case "s"
    when "s" then :matched
    else :no
    end
puts w.inspect
