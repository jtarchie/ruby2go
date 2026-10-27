require_relative "../../route_helpers"

module Views
  module Articles
    def self.article_json(article)
      io = String.new
      io << "{"
      io << "\"id\":"
      io << JsonBuilder.encode_value(article.id)
      io << ","
      io << "\"title\":"
      io << JsonBuilder.encode_value(article.title)
      io << ","
      io << "\"body\":"
      io << JsonBuilder.encode_value(article.body)
      io << ","
      io << "\"created_at\":"
      io << JsonBuilder.encode_datetime(article.created_at_raw)
      io << ","
      io << "\"updated_at\":"
      io << JsonBuilder.encode_datetime(article.updated_at_raw)
      io << ","
      io << "\"url\":"
      io << JsonBuilder.encode_value(RouteHelpers.article_json_path(article.id))
      io << "}"
      io
    end
  end
end
