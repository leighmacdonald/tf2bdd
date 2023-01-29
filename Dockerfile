FROM golang:latest as build
WORKDIR /build
COPY go.sum go.mod ./
RUN go mod download
COPY . .
RUN go build

FROM golang:latest
RUN apt update && apt install dumb-init -y
WORKDIR /app
COPY --from=build /build/tf2bdd .

EXPOSE 8899
ENTRYPOINT ["dumb-init", "--"]
CMD ["./tf2bdd"]
