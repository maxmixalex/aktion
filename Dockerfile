FROM debian:bookworm-slim

ARG TARGETARCH

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY dist/aktion-listener-linux-${TARGETARCH} /usr/local/bin/aktion-listener
COPY packaging/start-listener.sh /usr/local/bin/start-listener.sh
RUN chmod +x /usr/local/bin/aktion-listener /usr/local/bin/start-listener.sh

ENTRYPOINT ["/usr/local/bin/start-listener.sh"]
