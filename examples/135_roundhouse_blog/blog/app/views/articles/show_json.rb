require_relative ".."

module Views
  module Articles
    def self.show_json(article)
      io = String.new
      io << Views::Articles.article_json(article)
      io
    end
  end
end
