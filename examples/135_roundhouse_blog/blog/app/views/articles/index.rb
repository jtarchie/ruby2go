require_relative ".."
require_relative "../../route_helpers"

module Views
  module Articles
    def self.index_into(io, articles, notice, _alert)
      io << ActionView::ViewHelpers.turbo_stream_from("articles", "Turbo::StreamsChannel")
      io << "\n\n"
      ActionView::ViewHelpers.content_for_set(:title, "Articles")
      io << "\n"
      ActionView::ViewHelpers.content_for_set(:head, "<meta name=\"turbo-cache-control\" content=\"no-cache\">")
      io << "\n<div class=\"w-full\">\n"
      if !notice.nil? && !(notice || "").empty?
        io << "    <p class=\"py-2 px-3 bg-green-50 mb-5 text-green-500 font-medium rounded-md inline-block\" id=\"notice\">"
        io << ActionView::ViewHelpers.html_escape(notice.to_s)
        io << "</p>\n"
      end
      io << "\n  <div class=\"flex justify-between items-center\">\n    <h1 class=\"font-bold text-4xl\">Articles</h1>\n    "
      io << "<a href=\"#{ActionView::ViewHelpers.html_escape(RouteHelpers.new_article_path)}\" class=\"#{"rounded-md px-3.5 py-2.5 bg-blue-600 hover:bg-blue-500 text-white block font-medium"}\">#{"New article"}</a>"
      io << "\n  </div>\n\n  <div id=\"articles\" class=\"min-w-full divide-y divide-gray-200 space-y-5\">\n"
      if !(articles || "").empty?
        io << "      "
        articles.each { |a| Views::Articles.article_into(io, a, nil, nil) }
        io << "\n"
      else
        io << "      <p class=\"text-center my-10\">No articles found.</p>\n"
      end
      io << "  </div>\n</div>\n"
      nil
    end

    def self.index(articles, notice = nil, alert = nil)
      io = String.new
      Views::Articles.index_into(io, articles, notice, alert)
      io
    end
  end
end
