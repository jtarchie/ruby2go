# A gem method that edits Strings in place, as rack's Utils.forwarded_values does (decision 172).
module Demo
  module Text
    ALLOWED_FORWARDED_PARAMS = %w[by for host proto].map { |name| [name, name.to_sym] }.to_h.freeze
    def forwarded_values(forwarded_header)
      return unless forwarded_header
      header = forwarded_header.to_s.tr("\n", ";")
      header.sub!(/\A[\s;,]+/, '')
      num_params = num_escapes = 0
      max_params = max_escapes = 1024
      params = {}
  
      # Parse parameter list
      while i = header.index('=')
        # Only parse up to max parameters, to avoid potential denial of service
        num_params += 1
        return if num_params > max_params
  
        # Found end of parameter name, ensure forward progress in loop
        param = header.slice!(0, i+1)
  
        # Remove ending equals and preceding whitespace from parameter name
        param.chomp!('=')
        param.strip!
        param.downcase!
        return unless param = ALLOWED_FORWARDED_PARAMS[param]
  
        if header[0] == '"'
          # Parameter value is quoted, parse it, handling backslash escapes
          header.slice!(0, 1)
          value = String.new
  
          while i = header.index(/(["\\])/)
            c = $1
  
            # Append all content until ending quote or escape
            value << header.slice!(0, i)
  
            # Remove either backslash or ending quote,
            # ensures forward progress in loop
            header.slice!(0, 1)
  
            # stop parsing parameter value if found ending quote
            break if c == '"'
  
            # Only allow up to max escapes, to avoid potential denial of service
            num_escapes += 1
            return if num_escapes > max_escapes
            escaped_char = header.slice!(0, 1)
            value << escaped_char
          end
        else
          if i = header.index(/[;,]/)
            # Parameter value unquoted (which may be invalid), value ends at comma or semicolon
            value = header.slice!(0, i)
            value.sub!(/[\s;,]+\z/, '')
          else
            # If no ending semicolon, assume remainder of line is value and stop parsing
            header.strip!
            value = header
            header = ''
          end
          value.lstrip!
        end
  
        (params[param] ||= []) << value
  
        # skip trailing semicolons/commas/whitespace, to proceed to next parameter
        header.sub!(/\A[\s;,]+/, '') unless header.empty?
      end
  
      params
    end
    module_function :forwarded_values

    # a local rebound by bang calls, then reassigned another class
    def self.first_param(header)
      param = header.slice!(0, 3)
      param.strip!
      return unless param = ALLOWED_FORWARDED_PARAMS[param]
      [param, header]
    end
  end
end
