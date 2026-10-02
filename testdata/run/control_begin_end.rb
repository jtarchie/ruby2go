# BEGIN runs before the rest of the file, in its scope; END registers once,
# with at_exit's last-in-first-out order (ruby/spec language, #49).
p :first
BEGIN { p :begin1; b = 5 }
p defined?(b), b
x = 10
2.times { END { p [:end_in_loop, x] } }
END { p :end2 }
at_exit { p :at_exit }
BEGIN { p :begin2 }
p :last
