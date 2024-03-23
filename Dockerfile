FROM golang:1.22-alpine as build
WORKDIR /build
RUN apk add build-base
COPY go.sum go.mod ./
RUN go mod download
COPY . .
RUN go build -o build/tf2bdd ./cmd/tf2bdd/main.go

FROM alpine:latest
RUN apk add dumb-init
WORKDIR /app
COPY --from=build /build/build/tf2bdd .

EXPOSE 8899
ENTRYPOINT ["dumb-init", "--"]
CMD ["./tf2bdd"]
