require_relative ".."
require_relative "../../route_helpers"

module Views
  module Articles
    def self.edit_into(io, article, notice, alert)
      ActionView::ViewHelpers.content_for_set(:title, "Editing article")
      io << "\n<div class=\"md:w-2/3 w-full\">\n  <h1 class=\"font-bold text-4xl\">Editing article</h1>\n\n  "
      Views::Articles.form_into(io, article, nil, nil)
      io << "\n\n  "
      io << "<a href=\"#{ActionView::ViewHelpers.html_escape(RouteHelpers.article_path(article.id))}\" class=\"#{"w-full sm:w-auto text-center mt-2 sm:mt-0 sm:ml-2 rounded-md px-3.5 py-2.5 bg-gray-100 hover:bg-gray-50 inline-block font-medium"}\">#{"Show this article"}</a>"
      io << "\n  "
      io << "<a href=\"#{ActionView::ViewHelpers.html_escape(RouteHelpers.articles_path)}\" class=\"#{"w-full sm:w-auto text-center mt-2 sm:mt-0 sm:ml-2 rounded-md px-3.5 py-2.5 bg-gray-100 hover:bg-gray-50 inline-block font-medium"}\">#{"Back to articles"}</a>"
      io << "\n</div>\n"
      nil
    end

    def self.edit(article, notice = nil, alert = nil)
      io = String.new
      Views::Articles.edit_into(io, article, notice, alert)
      io
    end
  end
end
