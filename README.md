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
├── core/
│   └── session.go       # 会话消息管理
└── tools/
    ├── tool.go          # ToolDef 定义与转换
    ├── get_web_search.go
    ├── read_local_file.go
    ├── open_browser.go
    └── ...
```

## 说明

- 项目不包含任何硬编码 API Key，全部通过环境变量读取
- 分析结果由模型生成，实际薪资与行情请以招聘平台原始信息为准
