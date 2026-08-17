.PHONY: 运行 测试 检查 构建 容器启动 容器停止

运行:
	go run ./cmd/heromail

测试:
	go test ./...

检查:
	gofmt -w ./cmd ./internal
	go vet ./...

构建:
	go build ./cmd/heromail

容器启动:
	docker compose up --build

容器停止:
	docker compose down
