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
