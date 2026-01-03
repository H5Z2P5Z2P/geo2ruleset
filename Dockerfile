FROM golang:1.24.4-alpine AS build

WORKDIR /app
COPY . .
RUN go build -o surge-geosite .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /app/surge-geosite /app/surge-geosite

EXPOSE 8080
ENTRYPOINT ["/app/surge-geosite"]
