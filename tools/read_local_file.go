package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/openai/openai-go/v3"
)

func NewReadLocalFile() *ToolDef {
	return &ToolDef{
		Name:        "read_local_file",
		Description: "读取本地电脑上的文件内容。当用户要求你分析本地简历、文档或代码时使用此工具。",
		Params: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"filepath": map[string]string{
					"type":        "string",
					"description": "要读取的文件路径，绝对路径或相对路径，例如 'resume.txt'",
				},
			},
			"required": []string{"filepath"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			filepath, _ := args["filepath"].(string)
			filepath = strings.TrimSpace(filepath)
			if filepath == "" {
				return "", fmt.Errorf("filepath 参数为空")
			}

			content, err := os.ReadFile(filepath)
			if err != nil {
				return "", fmt.Errorf("文件不存在或无法读取：%s", filepath)
			}

			text := string(content)
			runes := []rune(text)
			if len(runes) > 4000 {
				text = string(runes[:4000]) + "(内容过长已截断)"
			}
			return text, nil
		},
	}
}
