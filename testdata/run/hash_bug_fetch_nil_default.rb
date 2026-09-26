# skip: fetch(k, nil) returns nil in MRI; rb2go treats a nil default as no default and raises KeyError (typed and untyped values alike)

# rbs_inline: enabled

h = { "a" => 1 } #: Hash[String, Integer]
puts h.fetch("a", nil).inspect
puts h.fetch("x", nil).inspect
cfg = { debug: false } #: Hash[Symbol, untyped]
puts cfg.fetch(:nope, nil).inspect
