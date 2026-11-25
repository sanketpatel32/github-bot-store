# Multi-stage Dockerfile for any bot in the store — build with:
#   docker build -f deploy/Dockerfile --build-arg BOT=stale-bot -t stale-bot .
# Static Go binaries mean the runtime image carries nothing but the binary.
FROM golang:1.23-alpine AS build
ARG BOT
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bot ./bots/$BOT

FROM scratch
COPY --from=build /bot /bot
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
EXPOSE 8000
ENTRYPOINT ["/bot"]
