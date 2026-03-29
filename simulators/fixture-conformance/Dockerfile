FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev git

WORKDIR /build

COPY go.mod go.sum ./
COPY hiveproxy/go.mod hiveproxy/go.sum ./hiveproxy/
RUN go mod download

COPY xrplsim/ ./xrplsim/
COPY internal/ ./internal/
COPY simulators/fixture-conformance/*.go ./simulators/fixture-conformance/

RUN go build -o /simulator ./simulators/fixture-conformance

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /simulator /simulator

# Copy fixtures from local build context.
# Place fixtures at <repo-root>/fixtures/ before building,
# e.g.: ln -s ../../fixtures/rippled-2.6.2-v2 fixtures
COPY fixtures/ /fixtures/

ENTRYPOINT ["/simulator"]
