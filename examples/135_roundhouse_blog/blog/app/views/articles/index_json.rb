require_relative ".."

module Views
  module Articles
    def self.index_json(articles)
      io = String.new
      io << "["
      io << articles.map { |article| Views::Articles.article_json(article) }.join(",")
      io << "]"
      io
    end
  end
end
