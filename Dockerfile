FROM golang:1.22-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/church-api ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget && addgroup -S church && adduser -S church -G church
WORKDIR /app
COPY --from=build /out/church-api /app/church-api
USER church
EXPOSE 8080
ENTRYPOINT ["/app/church-api"]

