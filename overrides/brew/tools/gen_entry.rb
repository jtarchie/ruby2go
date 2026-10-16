# Turns an MRI $LOADED_FEATURES trace into an rb2go entry: one require per feature, in load-completion order.
lib = "/home/linuxbrew/.linuxbrew/Homebrew/Library/Homebrew"
# Boot plumbing for MRI (load path, RubyGems, Bundler) that the overlays replace or drop.
SKIP = %w[utils/gem_setup].freeze
# Build-time facts for this target (Linux): what MRI decides at load time from the host.
OVERLAYS = { "standalone/init.rb" => "/w/overlay/standalone_init.rb", "standalone/sorbet.rb" => "/w/overlay/standalone_sorbet.rb" }.freeze
main = ARGV.fetch(1)
reqs = []
dirs = [lib]
prelude_gems = { "sorbet-runtime" => "sorbet-runtime", "json" => "json", "logger" => "logger", "tsort" => "tsort" }
File.readlines(ARGV.fetch(0), chomp: true).drop(2).each do |f|
  next if f == "/w/tools/trace.rb"
  name =
    case f
    when %r{\A#{lib}/vendor/bundle/ruby/[^/]+/gems/([^/]+?)-\d[^/]*/(lib(?:/concurrent-ruby)?)/(.+)\.rb\z}
      gem, sub, rel = $1, $2, $3
      if prelude_gems[gem]
        prelude_gems[gem]
      else
        dirs << f[%r{\A.*?/gems/[^/]+/#{sub}}]
        rel
      end
    when %r{\A#{lib}/vendor/gems/([^/]+)/lib/(.+)\.rb\z}
      dirs << "#{lib}/vendor/gems/#{$1}/lib"
      $2
    when %r{\A#{lib}/vendor/}
      next
    when %r{\A#{lib}/(.+)\.rb\z}
      $1
    when %r{/lib/ruby/4\.0\.0/(?:aarch64-linux/)?(.+)\.(?:rb|so)\z}
      $1
    else
      next
    end
  next if name.nil? || name.start_with?("enc/", "trans/") || %w[rbconfig portable_ruby_gems encdb].include?(name)
  next if f == main || SKIP.include?(name)
  reqs << name unless reqs.include?(name)
end
OVERLAYS.each { |from, to| puts "# rb2go_replace: #{lib}/#{from} => #{to}" }
puts "# Generated from an MRI trace of `brew #{ARGV.fetch(2)}`: what MRI loaded, in order (decision 197)."
reqs.each { |r| puts "require #{r.inspect}" }
File.write("/w/dirs.txt", dirs.uniq.join("\n") + "\n")
