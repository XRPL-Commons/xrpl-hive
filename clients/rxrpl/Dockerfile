ARG tag=nightly/2026-04-27

FROM rust:1.85-bookworm AS builder

ARG github=https://github.com/RomThpt/rxrpl.git
ARG tag=nightly/2026-04-27

RUN apt-get update && apt-get install -y git protobuf-compiler && rm -rf /var/lib/apt/lists/*

WORKDIR /build
# COPY source from build context instead of git clone to bypass BuildKit's
# git/cargo layer caching. Caller must symlink or copy rxrpl source into
# this directory before building (see hive-launcher script).
COPY src /build
RUN cargo build --release --bin rxrpl --no-default-features
RUN grep -aoc 'ReferenceFeeUnits' /build/target/release/rxrpl && echo "OK: ReferenceFeeUnits in binary"

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y python3 bash ca-certificates socat && rm -rf /var/lib/apt/lists/*

COPY --from=builder /build/target/release/rxrpl /usr/local/bin/rxrpl

# Write version.
RUN /usr/local/bin/rxrpl --version > /version.txt 2>&1 || echo "rxrpl-unknown" > /version.txt

RUN mkdir -p /etc/rxrpl /var/lib/rxrpl/data /hive-bin

ADD xrpl_start.sh /xrpl_start.sh
RUN chmod +x /xrpl_start.sh

EXPOSE 5005 6006 51235

USER nobody

ENTRYPOINT ["/xrpl_start.sh"]
