FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY media-server/go.mod ./
COPY media-server/*.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/media-server .

FROM alpine:3.20

RUN addgroup -S media-server && adduser -S -G media-server media-server

ENV LISTEN_ADDR=:80
ENV MEDIA_ROOT=/media

WORKDIR /app
COPY --from=builder /out/media-server /usr/local/bin/media-server

RUN mkdir -p /media && chown -R media-server:media-server /media

USER media-server
VOLUME ["/media"]

ENTRYPOINT ["media-server"]
