require_relative ".."

module Views
  module Layouts
    def self.mailer_into(io, body, notice, alert)
      io << "<!DOCTYPE html>\n<html>\n  <head>\n    <meta http-equiv=\"Content-Type\" content=\"text/html; charset=utf-8\">\n    <style>\n      /* Email styles need to be inline */\n    </style>\n  </head>\n\n  <body>\n    "
      io << body
      io << "\n  </body>\n</html>\n"
      nil
    end

    def self.mailer(body, notice = nil, alert = nil)
      io = String.new
      Views::Layouts.mailer_into(io, body, notice, alert)
      io
    end
  end
end
