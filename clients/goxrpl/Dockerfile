ARG baseimage=goxrpl
ARG tag=latest

FROM golang:1.25-alpine AS builder

ARG github=https://github.com/LeJamon/go-xrpl.git
ARG tag=main

# Same toolchain as go-xrpl's own Dockerfile: the native OpenSSL and
# secp256k1 dependencies are built by scripts/setup-mpt-crypto.sh with Conan.
RUN apk add --no-cache \
    autoconf automake bash cmake g++ git libtool linux-headers make \
    musl-dev perl pipx pkgconf \
 && pipx install conan==2.18.1

ENV CGO_ENABLED=1 \
    PATH="/root/.local/bin:${PATH}" \
    PKG_CONFIG_PATH="/build/.mpt-crypto"

WORKDIR /build
RUN git clone --depth 1 --branch $tag $github . || (git clone $github . && git checkout $tag)
RUN ./scripts/setup-mpt-crypto.sh setup
RUN go build -tags mptcrypto -trimpath \
    -ldflags="-s -w -linkmode external -extldflags '-static' -X=github.com/LeJamon/go-xrpl/version.Version=$(git rev-parse --short=8 HEAD)" \
    -o /goxrpl ./cmd/goxrpl

# List the amendments this build supports, for XRPL_FEATURES=all.
RUN mkdir -p hiveamendments && printf '%s\n' \
    'package main' \
    'import (' '"fmt"' '"github.com/LeJamon/go-xrpl/amendment"' ')' \
    'func main() {' \
    'for _, f := range amendment.SupportedFeatures() {' \
    'if !f.Retired {' 'fmt.Println(f.Name)' '}' \
    '}' '}' > hiveamendments/main.go \
 && go run -tags mptcrypto ./hiveamendments > /all_amendments.txt

FROM alpine:latest

RUN apk add --no-cache python3 bash ca-certificates

COPY --from=builder /goxrpl /usr/local/bin/goxrpl
COPY --from=builder /all_amendments.txt /etc/goxrpl/all_amendments.txt

# Write version.
RUN /usr/local/bin/goxrpl version > /version.txt 2>&1 || echo "goxrpl-unknown" > /version.txt

RUN mkdir -p /etc/goxrpl /tmp/goxrpl/db /hive-bin

ADD xrpl_start.sh /xrpl_start.sh
RUN chmod +x /xrpl_start.sh

EXPOSE 5005 6006 51235

ENTRYPOINT ["/xrpl_start.sh"]
