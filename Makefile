# Build k/v database
all: test build

build:
	go build -o mykv ./cmd/mykv

c-tools:
	$(MAKE) -C c-src

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test -v -coverprofile coverage.out ./...

race:
	go test -race ./...

bench:
	go test -bench=. -benchmem ./...

clean:
	rm -rf mykv data.db
	$(MAKE) -C c-src clean

run:
	./mykv -o put -c e2e -k Foo -v Bar && hexdump -C data.db

.PHONY: test build c-tools fmt vet race bench clean run
