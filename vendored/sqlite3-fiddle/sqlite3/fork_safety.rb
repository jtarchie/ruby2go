# frozen_string_literal: true
# rbs_inline: enabled

require "weakref"

module SQLite3
  # based on Rails's active_support/fork_tracker.rb
  module ForkSafety
    module CoreExt # :nodoc:
      #: () -> Integer?
      def _fork
        pid = super
        if pid == 0
          ForkSafety.discard
        end
        pid
      end
    end

    @databases = [] #: Array[WeakRef[Database]]
    @mutex = Mutex.new #: Mutex
    @suppress = false #: bool

    if defined?(::Binding) # rb2go cannot prepend onto Process: a forked child there keeps its connections
      #: () -> void
      def self.hook! # :nodoc:
        ::Process.singleton_class.prepend(CoreExt)
      end
    else
      #: () -> void
      def self.hook!; end
    end

    #: (Database) -> void
    def self.track(database) # :nodoc:
      @mutex.synchronize do
        @databases << WeakRef.new(database)
      end
    end

    #: () -> void
    def self.discard # :nodoc:
      warned = @suppress
      @databases.each do |ref|
        next unless ref.weakref_alive?

        begin
          db = ref.__getobj__
          unless db.closed? || db.readonly?
            unless warned
              # why: https://github.com/sparklemotion/sqlite3-ruby/pull/558
              warn("Writable sqlite database connection(s) were inherited from a forked process. " \
                   "This is unsafe and the connections are being closed to prevent possible data " \
                   "corruption. Please close writable sqlite database connections before forking.",
                uplevel: 0)
              warned = true
            end
            db.close
          end
        rescue WeakRef::RefError
        end
      end
      @databases.clear
    end

    #: () -> void
    def self.suppress_warnings! # silences the fork warning
      @suppress = true
    end
  end
end

SQLite3::ForkSafety.hook!
