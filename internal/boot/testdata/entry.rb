require "demo"

Demo::Thing.new("hi")
Demo::Lazy.new

Demo::Thing.class_eval("define_method(:from_eval) { :from_eval }")
