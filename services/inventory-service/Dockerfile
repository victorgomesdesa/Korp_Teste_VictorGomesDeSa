FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /inventory-service ./cmd/api

FROM alpine:3.21

RUN apk add --no-cache ca-certificates \
    && adduser -D -H -s /sbin/nologin appuser

COPY --from=build /inventory-service /usr/local/bin/inventory-service

USER appuser
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/inventory-service"]
