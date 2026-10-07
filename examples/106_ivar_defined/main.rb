# rbs_inline: enabled

# `defined?(@x)` asks whether an instance variable was ever assigned, which
# MRI tracks per object. rb2go gives each object of a class that asks a hidden
# bit per ivar, set on every write, so memoizing with `defined?` works even
# when the memoized value is nil or false. Classes that never ask pay nothing.

class Lookup
  #: (Hash[String, String?]) -> void
  def initialize(table)
    @table = table
    @calls = 0
  end

  # Computed once, even though the answer may be nil.
  #: () -> String?
  def admin
    return @admin if defined?(@admin)

    @calls += 1
    @admin = @table["admin"] #: String?
  end

  #: () -> Integer
  def calls = @calls

  #: () -> void
  def reset = remove_instance_variable(:@admin)
end

found = Lookup.new({ "admin" => "ann" })
missing = Lookup.new({})
2.times { p [found.admin, missing.admin] }
p [found.calls, missing.calls]
p missing.instance_variables

missing.reset
p missing.instance_variables
p [missing.admin, missing.calls]
