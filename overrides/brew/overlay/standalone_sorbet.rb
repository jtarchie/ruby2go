# typed: true
# frozen_string_literal: true

# rb2go overlay for standalone/sorbet.rb: it tunes sorbet-runtime's run-time checks, which rb2go does at compile time (#95).
require "sorbet-runtime"
require "extend/module"
