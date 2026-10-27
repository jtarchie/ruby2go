require_relative ".."
require_relative "../../../runtime/inflector"
require_relative "../../route_helpers"

module Views
  module Articles
    def self.form_into(io, article, notice, alert)
      form_method = if article.persisted?
        :patch
      else
        :post
      end
      io << "<form action=\"#{ActionView::ViewHelpers.html_escape(if article.persisted?
        RouteHelpers.article_path(article.id)
      else
        RouteHelpers.articles_path
      end)}\" accept-charset=\"UTF-8\" method=\"post\" class=\"#{"contents"}\">"
      io << ActionView::ViewHelpers.method_override_input(form_method)
      io << ActionView::ViewHelpers.csrf_token_hidden_input
      io << "\n"
      if ! (article.errors || "").empty?
        io << "    <div id=\"error_explanation\" class=\"bg-red-50 text-red-500 px-3 py-2 font-medium rounded-md mt-3\">\n      <h2>"
        io << Inflector.pluralize(article.errors.count, "error")
        io << " prohibited this article from being saved:</h2>\n\n      <ul class=\"list-disc ml-6\">\n"
        article.errors.each do |error|
          io << "          <li>"
          io << ActionView::ViewHelpers.html_escape(error.to_s)
          io << "</li>\n"
        end
        io << "      </ul>\n    </div>\n"
      end
      io << "\n  <div class=\"my-5\">\n    "
      io << "<label for=\"article_title\">Title</label>"
      io << "\n    "
      io << "<input type=\"text\" name=\"article[title]\" id=\"article_title\"#{ActionView::ViewHelpers.optional_value_attr(article[:title])} class=\"#{"block shadow-sm rounded-md border px-3 py-2 mt-2 w-full border-gray-400 focus:outline-blue-600"}\">"
      io << "\n  </div>\n\n  <div class=\"my-5\">\n    "
      io << "<label for=\"article_body\">Body</label>"
      io << "\n    "
      io << "<textarea name=\"article[body]\" id=\"article_body\" rows=\"#{ActionView::ViewHelpers.html_escape(4.to_s)}\" class=\"#{"block shadow-sm rounded-md border px-3 py-2 mt-2 w-full border-gray-400 focus:outline-blue-600"}\">\n#{ActionView::ViewHelpers.escape_or_empty(article[:body])}</textarea>"
      io << "\n  </div>\n\n  <div class=\"inline\">\n    "
      io << "<input type=\"submit\" name=\"commit\" value=\"#{ActionView::ViewHelpers.html_escape(if form_method == :patch
        "Update Article"
      else
        "Create Article"
      end)}\" data-disable-with=\"#{ActionView::ViewHelpers.html_escape(if form_method == :patch
        "Update Article"
      else
        "Create Article"
      end)}\" class=\"#{"w-full sm:w-auto rounded-md px-3.5 py-2.5 bg-blue-600 hover:bg-blue-500 text-white inline-block font-medium cursor-pointer"}\">"
      io << "\n  </div>\n"
      io << "</form>"
      nil
    end

    def self.form(article, notice = nil, alert = nil)
      io = String.new
      Views::Articles.form_into(io, article, notice, alert)
      io
    end
  end
end
