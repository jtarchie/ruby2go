# The lib dirs of the gems that provide ARGV's require names, with their runtime dependencies (decision 173).
specs = {}
add = lambda do |s|
  next if specs[s.name]

  specs[s.name] = s
  s.runtime_dependencies.each { |d| add.(Gem.loaded_specs[d.name] || d.to_spec) }
end
ARGV.each do |name|
  s = Gem::Specification.find_by_path(name)
  if s.nil?
    next if $LOAD_PATH.resolve_feature_path(name) # the standard library: a no-op, as before

    abort "cannot load such file -- #{name}: no prelude library, -I file or installed gem has it"
  end
  add.(s) unless s.default_gem?
end
puts specs.values.flat_map(&:full_require_paths)
