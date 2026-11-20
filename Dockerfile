FROM node:20-alpine AS web-build
WORKDIR /src
COPY web/package.json web/package-lock.json ./web/
RUN cd web && npm ci
COPY web/ ./web/
COPY design/ ./design/
RUN cd web && npm run build

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
    && mkdir -p /data /app/web/dist \
    && chown -R 10001:10001 /data /app
COPY --from=build /out/sapie-server /usr/local/bin/sapie-server
COPY --from=web-build /src/web/dist /app/web/dist
USER 10001:10001
WORKDIR /data
VOLUME ["/data"]
EXPOSE 47584 8081
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s CMD wget -q -O /dev/null http://127.0.0.1:8081/ || exit 1
ENTRYPOINT ["/usr/local/bin/sapie-server"]
CMD ["--admin-listen=:8080", "--ui-listen=0.0.0.0:8081", "--ui-dir=/app/web/dist"]
