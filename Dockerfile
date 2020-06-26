FROM golang:latest as build
WORKDIR /build
COPY go.sum go.mod ./
RUN go mod download
COPY . .
RUN go build

FROM alpine:latest
WORKDIR /app
COPY --from=build /build/tf2bdd .

EXPOSE 27015

CMD ["./tf2bdd"]
