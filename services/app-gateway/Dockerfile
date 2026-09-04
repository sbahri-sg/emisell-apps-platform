FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app-gateway ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app-gateway-migrate ./cmd/migrate \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app-gateway-admin-user ./cmd/admin-user \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app-gateway-healthcheck ./cmd/healthcheck

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app-gateway /app-gateway
COPY --from=build /out/app-gateway-migrate /app-gateway-migrate
COPY --from=build /out/app-gateway-admin-user /app-gateway-admin-user
COPY --from=build /out/app-gateway-healthcheck /app-gateway-healthcheck
EXPOSE 8080
USER nonroot:nonroot
HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=5 CMD ["/app-gateway-healthcheck"]
ENTRYPOINT ["/app-gateway"]
