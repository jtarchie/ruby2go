require_relative ".."
require_relative "../../route_helpers"

module Views
  module Articles
    def self.show_into(io, article, notice, alert)
      ActionView::ViewHelpers.content_for_set(:title, "Showing article")
      io << "\n"
      cf_head = String.new
      cf_head << "  "
      ActionView::ViewHelpers.content_for_set(:head, "<meta name=\"turbo-cache-control\" content=\"no-preview\">")
      cf_head << "\n  <meta name=\"article-id\" content=\""
      cf_head << ActionView::ViewHelpers.html_escape(article.id.to_s)
      cf_head << "\">\n"
      ActionView::ViewHelpers.content_for_set(:head, cf_head)
      io << "\n<div class=\"md:w-2/3 w-full\">\n"
      if ! notice.nil? && ! (notice || "").empty?
        io << "    <p class=\"py-2 px-3 bg-green-50 mb-5 text-green-500 font-medium rounded-md inline-block\" id=\"notice\">"
        io << ActionView::ViewHelpers.html_escape(notice.to_s)
        io << "</p>\n"
      end
      io << "\n  <h1 class=\"font-bold text-4xl\">"
      io << ActionView::ViewHelpers.html_escape(article.title.to_s)
      io << "</h1>\n\n  <div class=\"my-4\">\n    <p class=\"text-gray-700\">"
      io << ActionView::ViewHelpers.html_escape(article.body.to_s)
      io << "</p>\n  </div>\n\n  "
      io << "<a href=\"#{ActionView::ViewHelpers.html_escape(RouteHelpers.edit_article_path(article.id))}\" class=\"#{"w-full sm:w-auto text-center rounded-md px-3.5 py-2.5 bg-gray-100 hover:bg-gray-50 inline-block font-medium"}\">#{"Edit this article"}</a>"
      io << "\n  "
      io << "<a href=\"#{ActionView::ViewHelpers.html_escape(RouteHelpers.articles_path)}\" class=\"#{"w-full sm:w-auto text-center mt-2 sm:mt-0 sm:ml-2 rounded-md px-3.5 py-2.5 bg-gray-100 hover:bg-gray-50 inline-block font-medium"}\">#{"Back to articles"}</a>"
      io << "\n  "
      io << "<form action=\"#{ActionView::ViewHelpers.html_escape(RouteHelpers.article_path(article.id))}\" method=\"post\" class=\"#{"sm:inline-block mt-2 sm:mt-0 sm:ml-2"}\">#{ActionView::ViewHelpers.method_override_input(:delete)}<button type=\"submit\" class=\"#{"w-full rounded-md px-3.5 py-2.5 text-white bg-red-600 hover:bg-red-500 font-medium cursor-pointer"}\" data-turbo-confirm=\"#{"Are you sure?"}\">#{"Destroy this article"}</button>#{ActionView::ViewHelpers.csrf_token_hidden_input}</form>"
      io << "\n</div>\n\n<hr class=\"my-8\">\n\n<h2 class=\"text-xl font-bold mb-4\">Comments</h2>\n\n"
      io << ActionView::ViewHelpers.turbo_stream_from("article_#{article.id}_comments", "Turbo::StreamsChannel")
      io << "\n\n<div id=\"comments\" class=\"space-y-4 mb-8\">\n  "
      article.comments.each { |c| Views::Comments.comment_into(io, c, nil, nil) }
      io << "\n</div>\n\n<h3 class=\"text-lg font-semibold mb-2\">Add a Comment</h3>\n\n"
      form_record = Comment.new
      form_method = :post
      io << "<form action=\"#{ActionView::ViewHelpers.html_escape(RouteHelpers.article_comments_path(article.id))}\" accept-charset=\"UTF-8\" method=\"post\" class=\"#{"space-y-4"}\">"
      io << ActionView::ViewHelpers.method_override_input(form_method)
      io << ActionView::ViewHelpers.csrf_token_hidden_input
      io << "\n  <div>\n    "
      io << "<label for=\"comment_commenter\" class=\"#{"block font-medium"}\">Commenter</label>"
      io << "\n    "
      io << "<input type=\"text\" name=\"comment[commenter]\" id=\"comment_commenter\"#{ActionView::ViewHelpers.optional_value_attr(form_record[:commenter])} class=\"#{"block w-full border rounded p-2"}\">"
      io << "\n  </div>\n  <div>\n    "
      io << "<label for=\"comment_body\" class=\"#{"block font-medium"}\">Body</label>"
      io << "\n    "
      io << "<textarea name=\"comment[body]\" id=\"comment_body\" rows=\"#{ActionView::ViewHelpers.html_escape(3.to_s)}\" class=\"#{"block w-full border rounded p-2"}\">\n#{ActionView::ViewHelpers.escape_or_empty(form_record[:body])}</textarea>"
      io << "\n  </div>\n  "
      io << "<input type=\"submit\" name=\"commit\" value=\"#{"Add Comment"}\" data-disable-with=\"#{"Add Comment"}\" class=\"#{"bg-blue-600 text-white px-4 py-2 rounded"}\">"
      io << "\n"
      io << "</form>"
      nil
    end

    def self.show(article, notice = nil, alert = nil)
      io = String.new
      Views::Articles.show_into(io, article, notice, alert)
      io
    end
  end
end
