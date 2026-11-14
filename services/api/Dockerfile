FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/api /api
# The server needs no privileges: it listens on a high port and writes nothing.
USER 65534:65534
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/api"]
