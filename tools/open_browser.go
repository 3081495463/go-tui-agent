package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/openai/openai-go/v3"
)

func NewOpenBrowser() *ToolDef {
	return &ToolDef{
		Name:        "open_browser",
		Description: "在你的电脑屏幕上打开默认浏览器并跳转指定网址。当用户想看具体招聘网页或你搜到好链接时使用。",
		Params: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]string{
					"type":        "string",
					"description": "要打开的网页地址，必须以 http:// 或 https:// 开头",
				},
			},
			"required": []string{"url"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			url, _ := args["url"].(string)
			url = strings.TrimSpace(url)
			if url == "" {
				return "", fmt.Errorf("url 参数为空")
			}

			if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Run(); err != nil {
				return "", fmt.Errorf("打开浏览器失败：%v", err)
			}
			return "已成功打开网页", nil
		},
	}
}
