FROM golang:1.26-alpine AS build
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/sapie-server ./cmd/sapie-server

FROM alpine:3.23
RUN apk add --no-cache ca-certificates \
    && addgroup -g 10001 -S sapie \
    && adduser -u 10001 -G sapie -S -D -H sapie \
    && mkdir -p /data \
    && chown 10001:10001 /data
COPY --from=build /out/sapie-server /usr/local/bin/sapie-server
USER 10001:10001
WORKDIR /data
VOLUME ["/data"]
EXPOSE 47584 8080
ENTRYPOINT ["/usr/local/bin/sapie-server"]
CMD ["--listen=:47584", "--admin-listen=:8080", "--db=/data/sapie.db"]
