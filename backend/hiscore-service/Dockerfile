FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download || true
COPY shared ./shared
COPY hiscore-service ./hiscore-service
RUN CGO_ENABLED=0 go build -o /out/app ./hiscore-service

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/app /app/app
COPY hiscore-service/config.yaml /app/config.yaml
COPY openapi /app/openapi
ENV RS3_CONFIG=/app/config.yaml
EXPOSE 8081
ENTRYPOINT ["/app/app"]
