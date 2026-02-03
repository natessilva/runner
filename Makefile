.PHONY: generate build test run clean

generate:
	sqlc generate

build: generate
	go build -o runner .

test: generate
	go test ./...

run: build
	./runner

clean:
	rm -f runner
	rm -rf internal/store/sqlc/
