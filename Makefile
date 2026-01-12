.PHONY: clean test lint maps static

all: lint maps

clean:
	@rm -rf build

lint:
	@gofmt -w .

test: 
	@go test -v -count=1 ./test

maps:
	@mkdir -p build
	@go build -ldflags "-s -w" -o build/maps ./cmd/maps

static:
	@mkdir -p build
	@CGO_ENABLED=0 go build -ldflags "-s -w -extldflags '-static'" -o build/maps ./cmd/maps

