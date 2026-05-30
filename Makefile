.PHONY: run

run:
	cd backend && go run ./cmd

test:
	cd backend && go test ./...