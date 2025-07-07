.PHONY: test coverage

test:
	@go test -cover -race -count=1 ./...

coverage:
	@go test -coverprofile=coverage.txt -race -count=1 ./... 
	go tool cover -html=coverage.txt
	rm coverage.txt
