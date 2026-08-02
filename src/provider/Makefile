BINARY := provider-server

.PHONY: build run test vet lint clean

build: ## 编译服务端
	go build -o bin/$(BINARY) ./cmd/server

run: ## 本地运行
	go run ./cmd/server

test: ## 运行全部测试
	go test ./...

vet: ## 静态检查
	go vet ./...

lint: ## 代码规范检查（需安装 golangci-lint）
	golangci-lint run ./...

clean: ## 清理构建产物
	rm -rf bin
