test-quiet:
	@go test ./...

test-loud:
	@go test -v ./...

bumper: test-quiet
	@go run ./scripts/bumper
