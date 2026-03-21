FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build

COPY go.mod go.sum ./
COPY hiveproxy/go.mod hiveproxy/go.sum ./hiveproxy/
RUN go mod download

COPY xrplsim/ ./xrplsim/
COPY internal/ ./internal/
COPY simulators/propagation/ ./simulators/propagation/

RUN go build -o /simulator ./simulators/propagation

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /simulator /simulator
ENTRYPOINT ["/simulator"]
