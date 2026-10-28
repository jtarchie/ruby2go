require_relative ".."

module Views
  module Layouts
    HOISTED_STYLESHEET_LINK_TAG_APPLICATION = ActionView::ViewHelpers.stylesheet_link_tag("application", { "data-turbo-track": "reload" })
    HOISTED_STYLESHEET_LINK_TAG_TAILWIND = ActionView::ViewHelpers.stylesheet_link_tag("tailwind", { "data-turbo-track": "reload" })
    HOISTED_CONCAT = HOISTED_STYLESHEET_LINK_TAG_APPLICATION + "\n" + HOISTED_STYLESHEET_LINK_TAG_TAILWIND
    HOISTED_JAVASCRIPT_IMPORTMAP_TAGS = ActionView::ViewHelpers.javascript_importmap_tags(Importmap.pins, Importmap.entry)

    def self.application_into(io, body, _notice, _alert)
      io << "<!DOCTYPE html>\n<html>\n  <head>\n    <title>"
      io << ActionView::ViewHelpers.html_escape((ActionView::ViewHelpers.content_for_get(:title) || "Blog").to_s)
      io << "</title>\n    <meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n    <meta name=\"apple-mobile-web-app-capable\" content=\"yes\">\n    <meta name=\"application-name\" content=\"Blog\">\n    <meta name=\"mobile-web-app-capable\" content=\"yes\">\n    "
      io << ActionView::ViewHelpers.csrf_meta_tags
      io << "\n    "
      io << ActionView::ViewHelpers.csp_meta_tag
      io << "\n\n    "
      io << ActionView::ViewHelpers.get_slot(:head)
      io << "\n\n\n    <link rel=\"icon\" href=\"/icon.png\" type=\"image/png\">\n    <link rel=\"icon\" href=\"/icon.svg\" type=\"image/svg+xml\">\n    <link rel=\"apple-touch-icon\" href=\"/icon.png\">\n\n    "
      io << HOISTED_CONCAT
      io << "\n    "
      io << HOISTED_JAVASCRIPT_IMPORTMAP_TAGS
      io << "\n  </head>\n\n  <body>\n    <main class=\"container mx-auto mt-28 px-5 flex flex-col\">\n      "
      io << body
      io << "\n    </main>\n  </body>\n</html>\n"
      nil
    end

    def self.application(body, notice = nil, alert = nil)
      io = String.new
      Views::Layouts.application_into(io, body, notice, alert)
      io
    end
  end
end
