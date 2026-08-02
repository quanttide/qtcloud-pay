BINARY := provider-server

.PHONY: build run test vet lint docker-build docker-run clean

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

docker-build: ## 构建 Docker 镜像（含 SQLite CGO 依赖）
	docker build -t qtcloud-pay/provider:latest .

docker-run: ## 本地以 Docker 运行（SQLite 文件库挂载到 ./data）
	docker run --rm -p 8080:8080 \
		-e DB_DRIVER=sqlite -e DB_SQLITE_DSN=/data/pay.db \
		-v "$(PWD)/data:/data" \
		qtcloud-pay/provider:latest

clean: ## 清理构建产物
	rm -rf bin
