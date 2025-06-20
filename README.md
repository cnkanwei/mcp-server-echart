# mcp-server-echart

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

一个基于 `mcp-go` 框架构建的 Go 服务，它提供了一个能动态生成 ECharts 图表页面的工具。

## ✨ 功能特性

- **动态图表生成**：通过调用工具，传入 ECharts 的 JSON 配置，即可生成一个独立的 HTML 图表页面。
- **高度可配置**：支持自定义图表的标题、宽度和高度。
- **现代化页面**：生成的图表页面拥有一个干净、现代化的外观。
- **Docker 支持**：提供 `Dockerfile`，可以轻松构建轻量、可移植的 Docker 镜像。
- **配置灵活**：通过环境变量或 `.env` 文件进行配置，轻松适配不同环境。

---

## 🚀 快速开始

### 1. 准备工作

- [Go](https://golang.org/) (版本 1.21 或更高)
- [Docker](https://www.docker.com/) (可选，用于容器化部署)

### 2. 克隆与配置

克隆本仓库到您的本地：
```bash
git clone https://github.com/cnkanwei/mcp-server-echart.git
cd mcp-server-echart
```

复制配置文件模板，并根据需要进行修改：
```