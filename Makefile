test-quiet:
	@go test ./...

test-loud:
	@go test -v ./...

bump: test-quiet
	@go run ./scripts/bumper
