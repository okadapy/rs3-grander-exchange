FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download || true
COPY shared ./shared
COPY gateway ./gateway
RUN CGO_ENABLED=0 go build -o /out/app ./gateway

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/app /app/app
COPY gateway/config.yaml /app/config.yaml
COPY openapi /app/openapi
ENV RS3_CONFIG=/app/config.yaml
EXPOSE 8080
ENTRYPOINT ["/app/app"]
