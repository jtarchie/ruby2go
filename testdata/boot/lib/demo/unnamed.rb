# Boot loads it through a computed require, but nothing rb2go compiles names Demo::Unnamed, so it is never compiled (decision 175).
module Demo
  module Unnamed
    MAKER = [1].map { |x| x.instance_eval { binding } }
  end
end
