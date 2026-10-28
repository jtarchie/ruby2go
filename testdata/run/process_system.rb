# rbs_inline: enabled

# Kernel#system, backticks, %x() and $? (decision 97). stdout is flushed before each spawn, as MRI does.
require "English"
print "a"
system("echo b")
r = system("true")
p r, $CHILD_STATUS&.exitstatus
r = system("false")
p r, $CHILD_STATUS&.exitstatus
r = system("nonexistent_cmd_xyz")
p r, $CHILD_STATUS&.exitstatus
p system("echo", "x y", "$HOME")
out = %x(echo hi)
p out, $CHILD_STATUS&.success?
name = "there"
p %x(echo hi #{name} | tr a-z A-Z)
p %x(printf "%s" a b)
out = %x(exit 3)
p out, $CHILD_STATUS&.exitstatus
st = $CHILD_STATUS
p st.to_s.sub(/\d+/, "N"), st.class if st
begin
  %x(nonexistent_cmd_xyz)
rescue SystemCallError => e
  p e.class, e.message
end
