FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build

# Copy module files and download deps.
COPY go.mod go.sum ./
COPY hiveproxy/go.mod hiveproxy/go.sum ./hiveproxy/
RUN go mod download

# Copy xrplsim library and internal packages.
COPY xrplsim/ ./xrplsim/
COPY internal/ ./internal/

# Copy simulator source.
COPY simulators/smoke/ ./simulators/smoke/

# Build the simulator.
RUN go build -o /simulator ./simulators/smoke

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /simulator /simulator
ENTRYPOINT ["/simulator"]
