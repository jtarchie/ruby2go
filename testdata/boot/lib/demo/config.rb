# Not after main, where rb2go once ran an autoloaded file: app.rb reads DEFAULTS filled (decision 175).
module Demo
  module Config
    DEFAULTS = {} #: Hash[String, Integer]
    DEFAULTS["level"] = 3
  end
end
