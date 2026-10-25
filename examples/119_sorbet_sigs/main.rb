# typed: strict
# frozen_string_literal: true

# A Sorbet-typed program: rugaroo reads `sig {}` and `T.let` as its types,
# the way it reads `#:` comments, and MRI runs it with sorbet-runtime.
require "sorbet-runtime"

class Package
  extend T::Sig

  sig { returns(String) }
  attr_reader :name

  sig { returns(T::Array[String]) }
  attr_reader :deps

  sig { params(name: String, version: String, deps: T::Array[String]).void }
  def initialize(name, version, deps = [])
    @name = name
    @version = version
    @deps = deps
    @note = T.let(nil, T.nilable(String))
  end

  sig { params(text: String).returns(T.self_type) }
  def note(text)
    @note = text
    self
  end

  sig { params(width: Integer, verbose: T::Boolean).returns(String) }
  def describe(width: 10, verbose: false)
    line = "#{@name.ljust(width)} #{@version}"
    line += " (#{@deps.join(", ")})" if verbose && !@deps.empty?
    note = @note
    line += " # #{note}" if note
    line
  end
end

class Registry
  extend T::Sig

  sig { void }
  def initialize
    @packages = T.let({}, T::Hash[String, Package])
  end

  sig { params(pkg: Package).returns(Registry) }
  def add(pkg)
    @packages[pkg.name] = pkg
    self
  end

  sig { params(name: String).returns(T.nilable(Package)) }
  def find(name) = @packages[name]

  # Dependencies first, each once.
  sig { params(name: String, seen: T::Array[String]).returns(T::Array[String]) }
  def install_order(name, seen = [])
    pkg = T.must(find(name))
    pkg.deps.each { |dep| install_order(dep, seen) }
    seen << name unless seen.include?(name)
    seen
  end

  sig do
    type_parameters(:U)
      .params(blk: T.proc.params(pkg: Package).returns(T.type_parameter(:U)))
      .returns(T::Array[T.type_parameter(:U)])
  end
  def map_packages(&blk)
    @packages.values.map(&blk)
  end
end

registry = Registry.new
registry.add(Package.new("openssl", "3.4.0"))
        .add(Package.new("readline", "8.2"))
        .add(Package.new("python", "3.13.1", %w[openssl readline]).note("keg-only"))
        .add(Package.new("pipx", "1.7.1", %w[python]))

registry.map_packages { |p| p.describe(width: 9, verbose: true) }.each { |l| puts l }
puts "install pipx: #{registry.install_order("pipx").join(" -> ")}"
puts "name lengths: #{registry.map_packages { |p| p.name.length }.sum}"

count = T.let(0, Integer)
%w[openssl zsh python].each { |n| count += 1 if registry.find(n) }
puts "found #{count} of 3"

begin
  T.must(registry.find("zsh"))
rescue TypeError => e
  puts "TypeError: #{e.message}"
end
