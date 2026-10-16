# typed: true
# frozen_string_literal: true

# rb2go overlay for standalone/init.rb: MRI's load path, RubyGems and Bundler setup mean nothing to a compiled brew;
# only the constants the rest of Homebrew reads stay.
require "pathname"

HOMEBREW_LIBRARY_PATH = Pathname(File.expand_path("..", __dir__)).freeze
HOMEBREW_USING_PORTABLE_RUBY = false
HOMEBREW_BUNDLER_VERSION = ENV.fetch("HOMEBREW_BUNDLER_VERSION").freeze
