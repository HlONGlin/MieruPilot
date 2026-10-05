BIN_DIR := dist

.PHONY: all manager agent fmt vet clean

all: manager agent

manager:
	mkdir -p $(BIN_DIR)
	GOOS=linux  GOARCH=amd64 go build -ldflags "-s -w" -o $(BIN_DIR)/merit-manager-linux-amd64 ./cmd/manager
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o $(BIN_DIR)/merit-manager-windows-amd64.exe ./cmd/manager

agent:
	mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o $(BIN_DIR)/merit-agent-linux-amd64 ./cmd/agent
	GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o $(BIN_DIR)/merit-agent-linux-arm64 ./cmd/agent

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -rf $(BIN_DIR)
