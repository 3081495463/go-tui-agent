// browser-mcp 是一个独立的 MCP 服务器（stdio），把「打开浏览器」暴露为 MCP 工具。
// 用法：先 go build -o bin/browser-mcp.exe ./cmd/browser-mcp，再由主程序的 mcp.json 拉起。
package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// openURLInput open_url 工具的入参
type openURLInput struct {
	URL string `json:"url" jsonschema:"要打开的网页地址，必须以 http:// 或 https:// 开头"`
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "browser-mcp",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "open_url",
		Description: "在你的电脑屏幕上用默认浏览器打开指定网址。当用户想看具体网页时使用。",
	}, openURL)

	// stdio 传输：stdout 只能输出协议报文，日志一律走 stderr
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func openURL(ctx context.Context, _ *mcp.CallToolRequest, in openURLInput) (*mcp.CallToolResult, any, error) {
	url := strings.TrimSpace(in.URL)
	if url == "" {
		return nil, nil, fmt.Errorf("url 参数为空")
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, nil, fmt.Errorf("url 必须以 http:// 或 https:// 开头")
	}

	if err := exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url).Run(); err != nil {
		return nil, nil, fmt.Errorf("打开浏览器失败：%w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "已成功打开网页"}},
	}, nil, nil
}
