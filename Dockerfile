FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY web ./web
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/codex .

FROM alpine:3.22
RUN adduser -D -H -u 10001 codex
WORKDIR /app
COPY --from=build /out/codex /app/codex
COPY v20_db/chroma.sqlite3 /app/v20_db/chroma.sqlite3
COPY codex_data/chromem.tar.gz /tmp/chromem.tar.gz
RUN mkdir -p /app/codex_data \
    && tar -xzf /tmp/chromem.tar.gz -C /app/codex_data \
    && rm /tmp/chromem.tar.gz \
    && chown -R codex:codex /app
USER codex
ENV CHROMA_DB_PATH=/app/v20_db/chroma.sqlite3 \
    CHROMEM_DB_PATH=/app/codex_data/chromem \
    PORT=3000
EXPOSE 3000
CMD ["/app/codex"]
