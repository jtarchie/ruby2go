# prelude/rackup.rb
# rbs_inline: enabled

require_relative "webrick"
require_relative "stringio"
#
# rackup's WEBrick handler over prelude/webrick.rb's server, so config[:Port] and shutdown work as there (decision 174).

module Rackup
  module Handler
    class WEBrick
      # @dynamic
      #: (untyped, **untyped) ?{ (::WEBrick::HTTPServer) -> void } -> void
      def self.run(app, **options)
        default_host = (ENV["RACK_ENV"] || "development") == "development" ? "localhost" : nil
        options[:BindAddress] = options.delete(:Host) || default_host if !options[:BindAddress] || options[:Host]
        options[:Port] ||= 8080
        server = ::WEBrick::HTTPServer.new(options)
        rack = __proc(app) || ->(env) { app.call(env) } #: ^(Hash[String, untyped]) -> untyped
        __serve(server) do |env, input|
          env["rack.input"] = StringIO.new(input)
          env["rack.errors"] = $stderr
          status, headers, body = rack.call(env)
          begin
            [status, headers, __chunks(body)]
          ensure
            body.close if body.respond_to?(:close)
          end
        end
        @server = server #: ::WEBrick::HTTPServer?
        yield server if block_given?
        server.start
      end

      #: () -> void
      def self.shutdown
        @server&.shutdown
        @server = nil
      end

      #: (::WEBrick::HTTPServer) { (Hash[String, untyped], String) -> untyped } -> void
      def self.__serve(server) = %x{ server.srv.Handler = rbRackHandler(blk) }

      # A Proc held untyped has no dynamic call (decision 47), so a lambda app is unwrapped here.
      #: (untyped) -> (^(Hash[String, untyped]) -> untyped)?
      def self.__proc(app) = %x{ return rbRackProc(app) }

      #: (untyped) -> Array[String]
      def self.__chunks(body) = %x{ return rbRackChunks(body) }
    end
  end
end
