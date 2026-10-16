# Homebrew on Linux plus Go, pinned to the brew commit the patches apply to.
FROM ghcr.io/homebrew/brew:latest
USER root
RUN curl -fsSL https://go.dev/dl/go1.27.2.linux-arm64.tar.gz | tar -C /usr/local -xz
ENV PATH=/usr/local/go/bin:/home/linuxbrew/.linuxbrew/Homebrew/Library/Homebrew/vendor/portable-ruby/current/bin:$PATH
USER linuxbrew
ARG BREW_COMMIT=cdc1ca503231b9a96da64526774915d913c3ec9f
RUN git -C /home/linuxbrew/.linuxbrew/Homebrew fetch --depth=1 origin "$BREW_COMMIT" && git -C /home/linuxbrew/.linuxbrew/Homebrew checkout -q "$BREW_COMMIT"
