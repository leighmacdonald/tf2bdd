all: build

vet:
	@go vet . ./...

fmt:
	@go fmt . ./...

deps:
	@go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.27.0

build: clean deps fmt lint vet
	@go build -o tf2bdd

run:
	@go run $(GO_FLAGS) -race main.go

test:
	@go test $(GO_FLAGS) -race -cover . ./...

testcover:
	@go test -race -coverprofile c.out $(GO_FLAGS) ./...

lint:
	@golangci-lint run

bench:
	@go test -run=NONE -bench=. $(GO_FLAGS) ./...

clean:
	@go clean $(GO_FLAGS) -i

image:
	@docker build -t leighmacdonald/tf2bdd:latest .

runimage:
	@docker run --rm --name tf2bdd -it \
		--mount type=bind,source=$(CURDIR)/db.sqlite,target=/app/db.sqlite \
		leighmacdonald/tf2bdd:latest || true
