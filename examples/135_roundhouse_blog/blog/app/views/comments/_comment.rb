require_relative ".."
require_relative "../../route_helpers"

module Views
  module Comments
    def self.comment_into(io, comment, notice, alert)
      io << "<div id=\""
      io << ActionView::ViewHelpers.dom_id(comment)
      io << "\" class=\"p-4 bg-gray-50 rounded\">\n  <p class=\"font-semibold\">"
      io << ActionView::ViewHelpers.html_escape(comment.commenter.to_s)
      io << "</p>\n  <p class=\"text-gray-700\">"
      io << ActionView::ViewHelpers.html_escape(comment.body.to_s)
      io << "</p>\n  "
      io << "<form action=\"#{ActionView::ViewHelpers.html_escape(RouteHelpers.article_comment_path(comment.article_id, comment.id))}\" method=\"post\" class=\"#{"button_to"}\">#{ActionView::ViewHelpers.method_override_input(:delete)}<button type=\"submit\" class=\"#{"text-red-600 text-sm mt-2"}\" data-turbo-confirm=\"#{"Are you sure?"}\">#{"Delete"}</button>#{ActionView::ViewHelpers.csrf_token_hidden_input}</form>"
      io << "\n</div>\n"
      nil
    end

    def self.comment(comment, notice = nil, alert = nil)
      io = String.new
      Views::Comments.comment_into(io, comment, notice, alert)
      io
    end
  end
end
