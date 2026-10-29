# rbs_inline: enabled

# Kernel#system, backticks, %x() and $? (decision 97). stdout is flushed before each spawn, as MRI does.
print "a"
system("echo b")
r = system("true")
p r, $?&.exitstatus
r = system("false")
p r, $?&.exitstatus
r = system("nonexistent_cmd_xyz")
p r, $?&.exitstatus
p system("echo", "x y", "$HOME")
out = `echo hi`
p out, $?&.success?
name = "there"
p `echo hi #{name} | tr a-z A-Z`
p %x(printf "%s" a b)
out = `exit 3`
p out, $?&.exitstatus
st = $?
p st.to_s.sub(/\d+/, "N"), st.class if st
begin
  `nonexistent_cmd_xyz`
rescue SystemCallError => e
  p e.class, e.message
end
