FROM golang:1.21-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o gtp5g-exporter .

# distroless/static has no shell — reduces attack surface
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /build/gtp5g-exporter /gtp5g-exporter
EXPOSE 9090
ENTRYPOINT ["/gtp5g-exporter"]
