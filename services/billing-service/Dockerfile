FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /billing-service ./cmd/api

FROM alpine:3.21

RUN apk add --no-cache ca-certificates \
    && adduser -D -H -s /sbin/nologin appuser

COPY --from=build /billing-service /usr/local/bin/billing-service

USER appuser
EXPOSE 8081

ENTRYPOINT ["/usr/local/bin/billing-service"]
