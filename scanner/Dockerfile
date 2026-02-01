FROM golang:1.24-bookworm AS builder

WORKDIR /app

# Install zgrab2
RUN go install github.com/zmap/zgrab2/cmd/zgrab2@latest

COPY go.mod ./
RUN go mod download

COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /scanner main.go

FROM debian:bookworm-slim

# Install zmap and dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    zmap \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Remove the default blacklist and config that blocks private networks
RUN rm -f /etc/zmap/blacklist.conf && \
    touch /etc/zmap/blacklist.conf && \
    sed -i 's/^blacklist-file/#blacklist-file/' /etc/zmap/zmap.conf

COPY --from=builder /scanner /usr/local/bin/scanner
COPY --from=builder /go/bin/zgrab2 /usr/local/bin/zgrab2

CMD ["scanner"]
