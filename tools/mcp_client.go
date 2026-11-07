package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go/v3"
)

// mcpMaxContent 单个 MCP 工具返回内容的字符上限
const mcpMaxContent = 4000

// MCPServerConfig 一个 MCP 服务器的启动配置（stdio 方式）
type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

type mcpConfig struct {
	Servers map[string]MCPServerConfig `json:"mcpServers"`
}

var (
	mcpOnce  sync.Once
	mcpTools map[string]*ToolDef
	mcpErr   error
)

// LoadMCPTools 读取 mcp.json，启动其中所有 MCP 服务器，把它们的工具包装成 ToolDef。
// 结果在进程内只初始化一次；配置文件不存在时返回空集合，不影响原有工具。
func LoadMCPTools(ctx context.Context, configPath string) (map[string]*ToolDef, error) {
	mcpOnce.Do(func() {
		mcpTools, mcpErr = startMCPServers(ctx, configPath)
	})
	return mcpTools, mcpErr
}

func startMCPServers(ctx context.Context, configPath string) (map[string]*ToolDef, error) {
	out := make(map[string]*ToolDef)
	raw, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("读取 %s 失败：%w", configPath, err)
	}

	var cfg mcpConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("解析 %s 失败：%w", configPath, err)
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "go-tui-agent",
		Version: "1.0.0",
	}, nil)

	var failed []string
	for serverName, sc := range cfg.Servers {
		if sc.Command == "" {
			failed = append(failed, serverName+"(未配置 command)")
			continue
		}
		defs, err := connectMCPServer(ctx, client, serverName, sc)
		if err != nil {
			// 单个服务器失败不影响其他服务器
			failed = append(failed, fmt.Sprintf("%s(%v)", serverName, err))
			continue
		}
		for name, def := range defs {
			if _, exists := out[name]; exists {
				failed = append(failed, serverName+"(工具名冲突:"+name+")")
				continue
			}
			out[name] = def
		}
	}

	if len(failed) > 0 {
		return out, fmt.Errorf("部分 MCP 服务器不可用：%s", strings.Join(failed, "; "))
	}
	return out, nil
}

func connectMCPServer(ctx context.Context, client *mcp.Client, serverName string, sc MCPServerConfig) (map[string]*ToolDef, error) {
	cmd := exec.Command(sc.Command, sc.Args...)
	if len(sc.Env) > 0 {
		env := os.Environ()
		for k, v := range sc.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, fmt.Errorf("连接失败：%w", err)
	}

	res, err := session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("获取工具列表失败：%w", err)
	}

	defs := make(map[string]*ToolDef, len(res.Tools))
	for _, t := range res.Tools {
		llmName := mcpToolName(serverName, t.Name)
		toolName := t.Name
		defs[llmName] = &ToolDef{
			Name:        llmName,
			Description: fmt.Sprintf("[MCP:%s] %s", serverName, t.Description),
			Params:      schemaToParams(t.InputSchema),
			Fn: func(ctx context.Context, args map[string]any) (string, error) {
				callRes, err := session.CallTool(ctx, &mcp.CallToolParams{
					Name:      toolName,
					Arguments: args,
				})
				if err != nil {
					return "", fmt.Errorf("调用 %s 失败：%w", toolName, err)
				}
				text := mcpResultText(callRes)
				if callRes.IsError {
					return "工具执行失败：" + text, nil
				}
				return text, nil
			},
		}
	}
	return defs, nil
}

// mcpToolName 生成符合 OpenAI 函数名规范的工具名：server_tool，仅保留字母数字下划线和短横线，最长 64
func mcpToolName(server, tool string) string {
	raw := server + "_" + tool
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	name := b.String()
	if len(name) > 64 {
		name = name[:64]
	}
	return name
}

// schemaToParams 把 MCP 的 inputSchema 转成 OpenAI 的 FunctionParameters
func schemaToParams(schema any) openai.FunctionParameters {
	if m, ok := schema.(map[string]any); ok {
		return openai.FunctionParameters(m)
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return openai.FunctionParameters(m)
}

// mcpResultText 拼接 MCP 工具返回的文本内容，过长时截断
func mcpResultText(res *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
			sb.WriteString("\n")
		}
	}
	if res.StructuredContent != nil {
		if raw, err := json.Marshal(res.StructuredContent); err == nil {
			sb.WriteString(string(raw))
		}
	}

	text := strings.TrimSpace(sb.String())
	if len([]rune(text)) > mcpMaxContent {
		text = string([]rune(text)[:mcpMaxContent]) + "(内容过长已截断)"
	}
	return text
}
