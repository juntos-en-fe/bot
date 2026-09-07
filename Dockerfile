# syntax=docker/dockerfile:1

FROM golang:1.27-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/jef-bot ./cmd/bot

FROM debian:bookworm-slim

RUN apt-get update \
	&& apt-get install --no-install-recommends -y ca-certificates tzdata \
	&& groupadd --gid 10001 bot \
	&& useradd --uid 10001 --gid bot --no-create-home --shell /usr/sbin/nologin bot \
	&& install -d --mode 0750 --owner bot --group bot /data \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=build --chown=bot:bot /out/jef-bot /usr/local/bin/jef-bot

USER bot
ENV DATABASE_PATH=/data/bot.db

ENTRYPOINT ["/usr/local/bin/jef-bot"]
