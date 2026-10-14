# rbs_inline: enabled
# Open3.capture2 lets the child's stderr through to the program's (it went to /dev/null); standalone for the file-wide stderr comparison.
# stderr: match

require "open3"

out, st = Open3.capture2("sh", "-c", "echo err 1>&2; echo out")
p out, st.success?

out, err, st = Open3.capture3("sh", "-c", "echo err 1>&2; echo out")
p out, err, st.success?
