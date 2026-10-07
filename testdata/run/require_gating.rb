# Libraries exist only when required (decision 155): none of these is required here, so each answers as MRI does.
p defined?(JSON), defined?(CSV), defined?(Date), defined?(StringIO), defined?(BigDecimal), defined?(OptionParser)
p defined?(Set), defined?(Pathname), defined?(Monitor), defined?(GC), defined?(Process::Status)
p 1.respond_to?(:to_json), Time.respond_to?(:parse), "".respond_to?(:shellsplit), [].respond_to?(:to_csv), 1.respond_to?(:to_d)
p Time.respond_to?(:now), [1, 2].respond_to?(:pack)
