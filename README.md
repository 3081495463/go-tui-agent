# 终端岗位分析助手（go-tui-agent）

基于火山引擎（方舟 Ark）OpenAI 兼容接口与 Bubble Tea 构建的终端 Agent，支持联网搜索、读取本地文件、打开浏览器，并以卡片形式输出岗位分析结果。

## 功能

- 双模式：输入岗位名称走「分析模式」输出结构化报告；输入闲聊/提问走「对话模式」纯文本回复
- 联网搜索：调用 Tavily 获取岗位薪资、技能、趋势信息
- 本地文件：读取本地简历或文档（`read_local_file`，单次最多 4000 字）
- 打开浏览器：跳转到指定招聘页面（`open_browser`）
- 执行 Shell：`exec_shell`
- TUI：历史记录可滚动查看，输入框固定在底部，跨轮对话记忆（最多保留 40 条消息）

## 环境要求

- Go 1.21 及以上
- Windows（`open_browser` 使用 `rundll32`，其他系统需自行调整）

## 配置

运行前必须设置两个环境变量：

```bat
set ARK_API_KEY=你的火山引擎 API Key
set TAVILY_API_KEY=你的 Tavily API Key
```

PowerShell：

```powershell
$env:ARK_API_KEY="你的火山引擎 API Key"
$env:TAVILY_API_KEY="你的 Tavily API Key"
```

`agent.go` 中的 `Model` 为火山引擎推理接入点 ID（形如 `ep-xxxx`），请替换为你自己的接入点。

## MCP 扩展

项目内置 MCP 客户端，会把 `mcp.json` 中配置的服务器工具自动注册给模型（工具名格式为 `服务器名_工具名`，描述前缀 `[MCP:服务器名]`）。

仓库自带一个用 Go 写的 MCP 服务器 [cmd/browser-mcp](cmd/browser-mcp/main.go)，通过 `open_url` 工具打开浏览器：

```bat
go build -o bin/browser-mcp.exe ./cmd/browser-mcp
```

`mcp.json` 已指向它：

```json
{
  "mcpServers": {
    "browser": {
      "command": "bin/browser-mcp.exe"
    }
  }
}
```

启动主程序后模型会看到工具 `browser_open_url`。注意 `command` 是相对路径，需要在仓库根目录运行主程序。

新增其他服务器只需继续往 `mcpServers` 里加条目，`args`、`env` 均可选；用 npm 生态的服务器时 Windows 下 `command` 要写 `npx.cmd` 并先安装 Node.js。未配置（`{}`）时程序行为与之前完全一致，单个服务器启动失败只会在 stderr 提示，不影响内置工具和其他服务器。


## 运行

```bat
go build .
.\agentOpenai.exe
```

操作方式：输入内容回车提交，`↑/↓`、`PgUp/PgDn` 滚动历史，`Esc` / `Ctrl+C` 退出。

## 目录结构

```text
agentOpenai/
├── main.go              # Bubble Tea TUI 入口
├── agent.go             # Agent 主循环、工具注册、系统提示词
├── mcp.json             # MCP 服务器配置
├── cmd/
│   └── browser-mcp/     # 自带的 MCP 服务器（stdio），提供打开浏览器工具
├── core/
│   └── session.go       # 会话消息管理
└── tools/
    ├── tool.go          # ToolDef 定义与转换
    ├── mcp_client.go    # MCP 客户端，把外部 MCP 工具接入模型
    ├── get_web_search.go
    ├── read_local_file.go
    ├── open_browser.go
    └── ...
```

## 说明

- 项目不包含任何硬编码 API Key，全部通过环境变量读取
- 分析结果由模型生成，实际薪资与行情请以招聘平台原始信息为准
