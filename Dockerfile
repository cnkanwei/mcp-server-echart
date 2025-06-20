# ---- Build Stage ----
# 使用官方的 Go 镜像作为构建环境
FROM golang:1.24-alpine AS builder

# 设置工作目录
WORKDIR /app

# 预先复制 go.mod 和 go.sum 文件，以便利用 Docker 的缓存机制
COPY go.mod go.sum ./
RUN go mod download

# 复制所有源代码
COPY . .

# 编译应用，-ldflags "-w -s" 用于减小可执行文件大小
# CGO_ENABLED=0 确保静态链接，以便在 alpine 这种最小化镜像中运行
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /app/mcp-server-echart .

# ---- Final Stage ----
# 使用一个非常小的基础镜像
FROM alpine:latest

# 设置工作目录
WORKDIR /app

# 从 builder 阶段复制编译好的可执行文件
COPY --from=builder /app/mcp-server-echart /app/mcp-server-echart

# 复制模板文件和静态资源目录
# 注意：这里我们假设 static 目录可能在运行时创建，但 template.html 是必须的
COPY template.html ./

# 创建静态文件目录，并确保其权限正确
RUN mkdir -p /app/static/charts && \
    chown -R nobody:nogroup /app && \
    chmod -R 755 /app
    
# 切换到非 root 用户以增强安全性
USER nobody:nogroup

# 暴露应用端口（默认 8989）
# 这个端口可以通过 PORT 环境变量在运行时覆盖
EXPOSE 8989

# 设置默认环境变量
ENV PORT=8989
ENV LOG_LEVEL=info
ENV STATIC_DIR=/app/static
ENV PUBLIC_URL="http://localhost:8989"

# 运行应用
CMD ["/app/mcp-server-echart"] 