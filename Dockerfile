ARG GO_VERSION=1.26.9
FROM golang:${GO_VERSION}-bookworm AS builder

WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN go build -v -o /disco .

FROM node:22-bookworm-slim AS node_modules

WORKDIR /usr/src/app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev

FROM debian:bookworm

RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /disco
COPY --from=builder /disco ./disco
COPY --from=node_modules /usr/src/app/node_modules ./node_modules
COPY templates ./templates
COPY js ./js
COPY css ./css
COPY img ./img
CMD ["./disco"]
