FROM golang:1.24-alpine AS build
WORKDIR /app
ENV GOTOOLCHAIN=auto
ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd/main.go

FROM alpine:3.20
WORKDIR /
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /app/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
