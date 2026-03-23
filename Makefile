.PHONY: generate test bench

generate:
	go run ./cmd/main.go

test:
	go test ./pkg/... -v

bench:
	go test ./pkg/... -bench=. -benchmem
