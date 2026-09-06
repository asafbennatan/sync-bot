BINARY := sync-bot
MODULE := github.com/abennata/chai-cloner

.PHONY: build test install clean

build:
	go build -o $(BINARY) .

test:
	go test ./...

install:
	go install $(MODULE)

clean:
	rm -f $(BINARY)
