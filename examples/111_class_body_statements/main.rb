# rbs_inline: enabled

# Class- and module-body statements run when the class is defined, in source
# order, as MRI's class body does (#87): here `%w(...).each` fills a constant.

class Table
  ROWS = {}
  %w(one two three).each do |word|
    ROWS[word] = word.upcase
  end
end

module Registry
  NAMES = []
  %w(alpha beta).each { |name| NAMES << name }
end

puts Table::ROWS.inspect
puts Registry::NAMES.inspect
