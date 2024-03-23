FROM golang:1.22-alpine as build
WORKDIR /build
RUN apk add build-base
COPY go.sum go.mod ./
RUN go mod download
COPY . .
RUN go build -o tf2bdd

FROM golang:latest
RUN apt update && apt install dumb-init -y
WORKDIR /app
COPY --from=build /build/tf2bdd .

EXPOSE 8899
ENTRYPOINT ["dumb-init", "--"]
CMD ["./tf2bdd"]
