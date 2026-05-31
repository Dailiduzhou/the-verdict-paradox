ARG GO_VERSION=1.26
ARG NODE_VERSION=22

FROM golang:${GO_VERSION} AS builder

ARG GOPROXY
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

ENV GOPROXY=${GOPROXY:-https://goproxy.cn,direct}
ENV HTTP_PROXY=${HTTP_PROXY}
ENV HTTPS_PROXY=${HTTPS_PROXY}
ENV NO_PROXY=${NO_PROXY}
ENV CGO_ENABLED=0
ENV GOOS=linux

WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend ./

RUN mkdir -p bin && \
    go build -ldflags "-X main.Version=docker-build" -o ./bin/server ./app/game/cmd/game

FROM node:${NODE_VERSION}-alpine AS frontend-builder

WORKDIR /src/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend ./
RUN npm run build

FROM debian:stable-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        netbase \
    && rm -rf /var/lib/apt/lists/ \
    && apt-get autoremove -y && apt-get autoclean -y

COPY --from=builder /src/bin /app
COPY --from=frontend-builder /src/frontend/dist /app/frontend-dist

WORKDIR /app

EXPOSE 8000
EXPOSE 9000
VOLUME /data/conf

CMD ["./server", "-conf", "/data/conf"]
