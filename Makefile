all: build

vet:
	@go vet . ./...

fmt:
	@go fmt . ./...

deps:
	@go get github.com/golangci/golangci-lint/cmd/golangci-lint

build: clean deps fmt lint vet
	@go build -o tf2bdd

run:
	@go run $(GO_FLAGS) -race main.go

test:
	@go test $(GO_FLAGS) -race -cover . ./...

testcover:
	@go test -race -coverprofile c.out $(GO_FLAGS) ./...

lint:
	golangci-lint run

bench:
	@go test -run=NONE -bench=. $(GO_FLAGS) ./...

clean:
	@go clean $(GO_FLAGS) -i

image:
	@docker build -t leighmacdonald/tf2bdd:1.0.0 .

runimage:
	@docker run --rm --name tf2bdd -it \
		--mount type=bind,source=$(CURDIR)/db.sqlite,target=/app/db.sqlite \
		leighmacdonald/tf2bdd:1.0.0 || true

fmt:
	gci write . --skip-generated -s standard -s default
	gofumpt -l -w .

static:
	@staticcheck -go 1.22 ./...

check_deps:
	go install github.com/daixiang0/gci@v0.13.1
	go install mvdan.cc/gofumpt@v0.6.0
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.57.0
	go install honnef.co/go/tools/cmd/staticcheck@v0.4.7
	go install github.com/goreleaser/goreleaser@v1.24.0