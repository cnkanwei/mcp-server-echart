package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

//go:embed template.html
var htmlTemplate string

// PageData holds data for the HTML template
type PageData struct {
	Title     string
	Width     int
	Height    int
	Option    template.JS
	OptionStr string
}

func init() {
	// 从 .env 文件加载环境变量（如果存在）
	if err := godotenv.Load(); err != nil {
		log.Info("未找到 .env 文件，将使用默认值或系统环境变量")
	}

	// 初始化日志配置
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	level, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		level = log.InfoLevel
	}
	log.SetLevel(level)
}

func main() {
	// 创建 MCP Server
	s := server.NewMCPServer(
		"ECharts 可视化图表生成服务", // 专业航班查询MCP服务
		"1.0.0",             // 版本号
	)

	// 注册 generate_echarts_page 工具
	generateEchartsPage := mcp.NewTool("generate_echarts_page",
		mcp.WithDescription("根据 ECharts 的 JSON 配置生成一个 HTML 图表页面"),
		mcp.WithObject("inputSchema",
			mcp.Description("ECharts 的 JSON 配置对象"),
			mcp.Required(),
		),
		mcp.WithString("title",
			mcp.Description("图表页面的标题"),
			mcp.Required(),
		),
		mcp.WithNumber("width",
			mcp.Description("图表的宽度（像素）"),
		),
		mcp.WithNumber("height",
			mcp.Description("图表的高度（像素）"),
		),
	)
	s.AddTool(generateEchartsPage, GenerateEchartsPage)

	// 使用 SSE Server，配置更多选项
	sseServer := server.NewSSEServer(
		s,
		server.WithUseFullURLForMessageEndpoint(true),
		server.WithMessageEndpoint("/message"),
		server.WithSSEEndpoint("/sse"),
		server.WithKeepAlive(true),
		server.WithKeepAliveInterval(15*time.Second),
	)

	// 创建路由
	mux := http.NewServeMux()

	// 注册 SSE 和 message 处理器
	mux.Handle("/sse", sseServer.SSEHandler())
	mux.Handle("/message", sseServer.MessageHandler())

	// 确保静态文件目录存在
	staticDir := getEnv("STATIC_DIR", "static")
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		if err := os.MkdirAll(staticDir, 0755); err != nil {
			log.Fatalf("创建静态目录失败: %v", err)
		}
	}

	mux.Handle("/", http.FileServer(http.Dir(staticDir))) // 提供静态文件服务

	// 创建 HTTP 服务器
	port := getEnv("PORT", "8989")
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// 在单独的 goroutine 中启动服务器
	go func() {
		log.Infof("HTTP 服务器正在启动，监听端口: %s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP 服务器启动失败: %v\n", err)
		}
	}()

	// 设置优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("正在关闭服务器...")

	// 创建带超时的上下文用于关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 优雅关闭 SSE 服务器
	if err := sseServer.Shutdown(ctx); err != nil {
		log.Fatalf("SSE 服务器关闭错误: %v", err)
	}

	// 优雅关闭 HTTP 服务器
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("HTTP 服务器关闭错误: %v", err)
	}

	log.Info("服务器已成功关闭")
}

func GenerateEchartsPage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 使用辅助函数安全地获取参数
	args := request.GetArguments()

	inputSchema, ok := args["inputSchema"].(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("参数 'inputSchema' 必须是一个 JSON 对象"), nil
	}

	title, ok := args["title"].(string)
	if !ok {
		return mcp.NewToolResultError("参数 'title' 必须是字符串"), nil
	}

	// GetInt 会自动处理类型转换，并在 key 不存在时返回默认值
	width := request.GetInt("width", 1000)
	height := request.GetInt("height", 600)

	// 将 inputSchema 编码为 JSON 字符串
	optionBytes, err := json.Marshal(inputSchema)
	if err != nil {
		return mcp.NewToolResultError("无法序列化 inputSchema: " + err.Error()), nil
	}

	// 准备用于显示和注入的数据
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, optionBytes, "", "  "); err != nil {
		log.Warnf("无法格式化JSON: %v", err)
		prettyJSON.WriteString(string(optionBytes)) // Fallback to raw string
	}

	data := PageData{
		Title:     title,
		Width:     width,
		Height:    height,
		Option:    template.JS(prettyJSON.String()),
		OptionStr: prettyJSON.String(),
	}

	// 解析并执行模板
	tmpl, err := template.New("echarts").Parse(getTemplate())
	if err != nil {
		return mcp.NewToolResultError("模板解析失败: " + err.Error()), nil
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return mcp.NewToolResultError("模板执行失败: " + err.Error()), nil
	}

	// 保存 HTML 文件到 static/charts 目录
	staticDir := getEnv("STATIC_DIR", "static")
	chartsDir := filepath.Join(staticDir, "charts")
	if err := os.MkdirAll(chartsDir, 0755); err != nil {
		return mcp.NewToolResultError("创建charts目录失败: " + err.Error()), nil
	}

	fileName := fmt.Sprintf("echarts_%d.html", time.Now().UnixNano())
	filePath := filepath.Join(chartsDir, fileName)

	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		return mcp.NewToolResultError("保存 HTML 文件失败: " + err.Error()), nil
	}

	// 返回结果 URL
	port := getEnv("PORT", "8989")
	publicURL := getEnv("PUBLIC_URL", fmt.Sprintf("http://localhost:%s", port))
	// 确保 publicURL 没有尾部斜杠
	publicURL = strings.TrimSuffix(publicURL, "/")

	resultURL := fmt.Sprintf("%s/charts/%s", publicURL, fileName)
	return mcp.NewToolResultText(resultURL), nil
}

func getTemplate() string {
	// 优先从文件读取，方便开发时热重载
	data, err := os.ReadFile("template.html")
	if err == nil {
		return string(data)
	}
	// 如果文件读取失败（例如在部署环境中），使用 embed 的模板
	return htmlTemplate
}

// getEnv 读取环境变量，如果不存在则返回默认值
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// getEnvAsInt 读取环境变量并解析为整数，如果不存在或解析失败则返回默认值
func getEnvAsInt(name string, fallback int) int {
	valueStr := getEnv(name, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return fallback
}
