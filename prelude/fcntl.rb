# rbs_inline: enabled

# Fcntl's commands for IO#fcntl: the same numbers on Linux and macOS.
module Fcntl
  F_DUPFD = 0 #: Integer
  F_GETFD = 1 #: Integer
  F_SETFD = 2 #: Integer
  F_GETFL = 3 #: Integer
  F_SETFL = 4 #: Integer
  FD_CLOEXEC = 1 #: Integer
end
