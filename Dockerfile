FROM golang:1.19-alpine as build
WORKDIR /build
RUN apk add build-base
COPY go.sum go.mod ./
RUN go mod download
COPY . .
RUN go build -o tf2bdd

FROM alpine:latest
RUN apk add dumb-init
WORKDIR /app
COPY --from=build /build/tf2bdd .
EXPOSE 8899
ENTRYPOINT ["dumb-init", "--"]
CMD ["./tf2bdd"]
