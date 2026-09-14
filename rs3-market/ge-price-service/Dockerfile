FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download || true
COPY shared ./shared
COPY ge-price-service ./ge-price-service
RUN CGO_ENABLED=0 go build -o /out/app ./ge-price-service

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/app /app/app
COPY ge-price-service/config.yaml /app/config.yaml
ENV RS3_CONFIG=/app/config.yaml
EXPOSE 8083
ENTRYPOINT ["/app/app"]
