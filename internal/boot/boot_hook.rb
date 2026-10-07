# Boot-snapshot tracing hook (docs/design.md decision 161).
#
# Loaded with `ruby -I <hookdir> -r boot_hook <entry>` before the program's
# own requires. It records what a boot does that the compiler cannot read from
# source: methods built by define_method or a string eval, the files autoload
# actually loaded, every class/ancestor defined, and the objects held in class
# state. The manifest is written as JSON to the file named by RB2GO_BOOT_OUT.

require "json"

module Boot
  @classes = {}       # name => { name:, super:, path:, line: }
  @define_methods = [] # { owner:, name:, file:, line:, source: }
  @evals = []          # { file:, line:, source:, caller: }
  @autoloads = []      # { owner:, const:, path:, registered: }
  @features = nil

  class << self
    attr_reader :classes, :define_methods, :evals, :autoloads

    def install
      @features = $LOADED_FEATURES.dup
      install_class_trace
      install_define_method
      install_evals
      install_autoload
    end

    def install_class_trace
      @class_tp = TracePoint.new(:class) do |tp|
        klass = tp.self
        next unless klass.is_a?(Class) || klass.is_a?(Module)
        name = safe_name(klass)
        next if name.nil? || name.empty? || @classes.key?(name)
        @classes[name] = { "name" => name, "super" => safe_name(klass.is_a?(Class) ? klass.superclass : nil),
                           "path" => tp.path, "line" => tp.lineno }
      end
      @class_tp.enable
    end

    def install_define_method
      hook = Module.new do
        def define_method(name, *args, &blk)
          ::Boot.record_define_method(self, name, blk, args)
          super(name, *args, &blk)
        end
      end
      Module.prepend(hook)
    end

    # A string eval reaches the compiler only through the manifest. Kernel#eval,
    # Module#class_eval/module_eval and BasicObject#instance_eval are each
    # hooked for their String form; the block form is the source already.
    def install_evals
      Kernel.prepend(Module.new do
        def eval(*args, **kw, &blk)
          ::Boot.record_eval(args[0], args[2], args[3]) if args[0].is_a?(String)
          super(*args, **kw, &blk)
        end
      end)
      Module.prepend(Module.new do
        def class_eval(*args, **kw, &blk)
          ::Boot.record_eval(args[0], args[1], args[2]) if args[0].is_a?(String)
          super(*args, **kw, &blk)
        end

        def module_eval(*args, **kw, &blk)
          ::Boot.record_eval(args[0], args[1], args[2]) if args[0].is_a?(String)
          super(*args, **kw, &blk)
        end
      end)
      BasicObject.prepend(Module.new do
        def instance_eval(*args, **kw, &blk)
          ::Boot.record_eval(args[0], args[1], args[2]) if args[0].is_a?(String)
          super(*args, **kw, &blk)
        end
      end)
    end

    def install_autoload
      hook = Module.new do
        def autoload(name, path)
          ::Boot.record_autoload(self, name, path)
          super(name, path)
        end
      end
      Module.prepend(hook)
    end

    def record_define_method(owner, name, blk, args)
      loc = blk&.source_location || [nil, nil]
      @define_methods << { "owner" => safe_name(owner), "name" => name.to_s,
                           "file" => loc[0], "line" => loc[1],
                           "source" => nil,
                           "from" => args.empty? ? nil : safe_name(args[0]) }
    end

    def record_eval(src, file, line)
      loc = caller_locations(2, 1).first
      @evals << { "file" => file, "line" => line, "source" => src,
                  "caller" => loc ? "#{loc.path}:#{loc.lineno}" : nil }
    end

    def record_autoload(owner, name, path)
      @autoloads << { "owner" => safe_name(owner), "const" => name.to_s,
                      "path" => path, "registered" => true }
    end

    def safe_name(obj)
      return nil if obj.nil?
      obj.is_a?(Module) ? obj.name : obj.to_s
    rescue StandardError
      nil
    end

    def resolve_autoloads
      @autoloads.map do |a|
        owner = Object.const_get(a["owner"]) rescue nil
        pending = owner.respond_to?(:autoload?) ? owner.autoload?(a["const"]) : nil
        resolved = owner && pending.nil?
        loaded = nil
        if resolved
          base = a["path"].to_s
          loaded = $LOADED_FEATURES.find { |f| f.end_with?("/#{base}.rb") || f == "#{base}.rb" }
        end
        a.merge("resolved" => !!resolved, "loaded" => loaded)
      end
    end

    def class_state
      out = []
      @classes.each_key do |name|
        owner = Object.const_get(name) rescue next
        next unless owner.is_a?(Module)
        owner.instance_variables.each do |ivar|
          value = owner.instance_variable_get(ivar)
          out << describe_state(name, ivar.to_s, value)
        end
      end
      out
    end

    def describe_state(owner, ivar, value)
      kind = safe_name(value.class)
      recreate = value.is_a?(Mutex) || value.is_a?(Queue)
      desc = case value
             when Mutex, Queue then "<#{kind}>"
             when Hash then "Hash(size=#{value.size}, default=#{!value.default.nil? || !value.default_proc.nil?})"
             when Array then "Array(size=#{value.size})"
             when String then value.dup
             else kind.to_s
             end
      { "owner" => owner, "ivar" => ivar, "kind" => kind, "desc" => desc,
        "recreate" => recreate, "frozen" => value.frozen? }
    end

    def dump(path)
      @class_tp&.disable
      classes = @classes.map do |name, info|
        owner = Object.const_get(name) rescue nil
        anc = owner.is_a?(Module) ? owner.ancestors.map { |a| safe_name(a) } : []
        info.merge("ancestors" => anc)
      end
      manifest = {
        "loaded_features" => ($LOADED_FEATURES - @features),
        "classes" => classes,
        "define_methods" => @define_methods,
        "evals" => @evals.uniq { |e| [e["source"], e["file"], e["line"]] },
        "autoloads" => resolve_autoloads,
        "class_state" => class_state,
      }
      File.write(path, JSON.pretty_generate(manifest))
    end
  end
end

Boot.install
at_exit { Boot.dump(ENV["RB2GO_BOOT_OUT"]) if ENV["RB2GO_BOOT_OUT"] }
