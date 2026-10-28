require_relative ".."
require_relative "../../../runtime/inflector"
require_relative "../../route_helpers"

module Views
  module Articles
    def self.article_into(io, article, _notice, _alert)
      io << "<div id=\""
      io << ActionView::ViewHelpers.dom_id(article)
      io << "\" class=\"flex flex-col sm:flex-row justify-between items-center pb-5 sm:pb-0\">\n  <div class=\"p-4 border rounded mb-4 flex-grow\">\n    <h2 class=\"text-xl font-bold\">\n      "
      io << "<a href=\"#{ActionView::ViewHelpers.html_escape(RouteHelpers.article_path(article.id))}\" class=\"#{"text-blue-600 hover:underline"}\">#{ActionView::ViewHelpers.html_escape(article.title.to_s)}</a>"
      io << "\n      <span id=\""
      io << ActionView::ViewHelpers.dom_id(article, :comments_count)
      io << "\" class=\"text-gray-500 text-sm font-normal ml-2\">\n        ("
      io << Inflector.pluralize(article.comments.size, "comment")
      io << ")\n      </span>\n    </h2>\n    <p class=\"text-gray-700 mt-2\">"
      io << ActionView::ViewHelpers.html_escape(ActionView::ViewHelpers.truncate(article.body.to_s, length: 100))
      io << "</p>\n  </div>\n  <div class=\"w-full sm:w-auto flex flex-col sm:flex-row space-x-2 space-y-2\">\n    "
      io << "<a href=\"#{ActionView::ViewHelpers.html_escape(RouteHelpers.article_path(article.id))}\" class=\"#{"w-full sm:w-auto text-center rounded-md px-3.5 py-2.5 bg-gray-100 hover:bg-gray-50 inline-block font-medium"}\">#{"Show"}</a>"
      io << "\n    "
      io << "<a href=\"#{ActionView::ViewHelpers.html_escape(RouteHelpers.edit_article_path(article.id))}\" class=\"#{"w-full sm:w-auto text-center rounded-md px-3.5 py-2.5 bg-gray-100 hover:bg-gray-50 inline-block font-medium"}\">#{"Edit"}</a>"
      io << "\n    "
      io << "<form action=\"#{ActionView::ViewHelpers.html_escape(RouteHelpers.article_path(article.id))}\" method=\"post\" class=\"#{"button_to"}\">#{ActionView::ViewHelpers.method_override_input(:delete)}<button type=\"submit\" class=\"#{"w-full sm:w-auto rounded-md px-3.5 py-2.5 text-white bg-red-600 hover:bg-red-500 font-medium cursor-pointer"}\" data-turbo-confirm=\"#{"Are you sure?"}\">#{"Destroy"}</button>#{ActionView::ViewHelpers.csrf_token_hidden_input}</form>"
      io << "\n  </div>\n</div>\n"
      nil
    end

    def self.article(article, notice = nil, alert = nil)
      io = String.new
      Views::Articles.article_into(io, article, notice, alert)
      io
    end
  end
end
