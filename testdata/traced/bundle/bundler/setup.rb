# What `bundle install --standalone` writes: RubyGems shims, then the gems' lib dirs.
require 'rbconfig'
module Kernel
  remove_method(:gem) if private_method_defined?(:gem)

  def gem(*); end

  private :gem
end
$:.unshift File.expand_path("#{__dir__}/../../lib")
