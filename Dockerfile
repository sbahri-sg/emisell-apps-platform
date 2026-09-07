FROM golang:1.26.6-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server && \
    CGO_ENABLED=0 go build -trimpath -o /out/cli ./cmd/cli && \
    CGO_ENABLED=0 go build -trimpath -o /out/worker ./cmd/worker

FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/* && groupadd -g 10001 emisell && useradd -u 10001 -g emisell emisell
WORKDIR /app
RUN mkdir -p /app/.local && chown -R emisell:emisell /app
COPY --from=build /out/ /usr/local/bin/
USER 10001:10001
EXPOSE 8087
CMD ["server"]
