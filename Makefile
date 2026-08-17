.PHONY: build test fmt fmt-check

build:
	go build -o bd-tui ./cmd/bd-tui

test:
	go test ./...

# Rewrite Go files in place.
fmt:
	@gofmt -w .

# Fail if any Go file needs gofmt. Run before committing.
fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "not gofmt-clean:"; \
		echo "$$unformatted" | sed 's/^/  /'; \
		echo; \
		echo "fix with: make fmt"; \
		exit 1; \
	fi
	@echo "gofmt: clean"
