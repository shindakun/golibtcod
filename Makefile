.PHONY: all test vet fmt sample clean

all: vet test

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

sample:
	go run ./cmd/sample

clean:
	rm -f sample_dungeon.png sample_dungeon.ans
