FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download || true
COPY shared ./shared
COPY realtime-service ./realtime-service
RUN CGO_ENABLED=0 go build -o /out/app ./realtime-service

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/app /app/app
COPY realtime-service/config.yaml /app/config.yaml
COPY openapi /app/openapi
ENV RS3_CONFIG=/app/config.yaml
EXPOSE 8085
ENTRYPOINT ["/app/app"]
