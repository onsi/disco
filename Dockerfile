ARG GO_VERSION=1.23.0
FROM golang:${GO_VERSION}-bookworm as builder

WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN go build -v -o /disco .

FROM debian:bookworm

RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /usr/src/app/. /disco/
COPY --from=builder /disco /disco/disco
WORKDIR /disco
CMD ["./disco"]