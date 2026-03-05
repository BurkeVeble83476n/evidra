.PHONY: build test clean golden-update docker-mcp docker-cli fmt lint tidy \
	test-mcp-inspector test-mcp-inspector-local-rest test-mcp-inspector-hosted test-mcp-inspector-hosted-rest

build:
	go build -o bin/evidra ./cmd/evidra
	go build -o bin/evidra-mcp ./cmd/evidra-mcp

test:
	go test ./... -v -count=1

test-mcp-inspector:
	bash tests/inspector/run_inspector_tests.sh

test-mcp-inspector-local-rest:
	EVIDRA_TEST_MODE=local-rest bash tests/inspector/run_inspector_tests.sh

test-mcp-inspector-hosted:
	EVIDRA_TEST_MODE=hosted-mcp bash tests/inspector/run_inspector_tests.sh

test-mcp-inspector-hosted-rest:
	EVIDRA_TEST_MODE=hosted-rest bash tests/inspector/run_inspector_tests.sh

golden-update:
	EVIDRA_UPDATE_GOLDEN=1 go test -run TestGolden -update ./internal/canon/...

docker-mcp:
	docker build -t evidra-mcp:dev -f Dockerfile .

docker-cli:
	docker build -t evidra:dev -f Dockerfile.cli .

fmt:
	gofmt -w .

lint:
	golangci-lint run

tidy:
	go mod tidy

clean:
	rm -rf bin/
