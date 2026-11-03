package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
)

func NewGetWebSearch() *ToolDef {
	return &ToolDef{
		Name:        "web_search",
		Description: "搜索互联网获取实时信息。适用于查询岗位薪资、技能要求、行业趋势、公司信息、招聘要求等。",
		Params: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]string{
					"type":        "string",
					"description": "搜索关键词，尽量具体，例如 'Go后端开发 薪资 2026'",
				},
			},
			"required": []string{"query"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			query, _ := args["query"].(string)
			query = strings.TrimSpace(query)

			apiKey := os.Getenv("TAVILY_API_KEY")
			if apiKey == "" {
				return "", fmt.Errorf("环境变量 TAVILY_API_KEY 未设置")
			}

			body, err := json.Marshal(map[string]any{
				"api_key":      apiKey,
				"query":        query,
				"max_results":  5,
				"search_depth": "basic",
			})
			if err != nil {
				return "", err
			}

			req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewReader(body))
			if err != nil {
				return "", err
			}
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 20 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return "", fmt.Errorf("tavily HTTP %d", resp.StatusCode)
			}

			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return "", err
			}

			var result struct {
				Results []struct {
					Title   string `json:"title"`
					URL     string `json:"url"`
					Content string `json:"content"`
				} `json:"results"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				return "", err
			}

			if len(result.Results) == 0 {
				return fmt.Sprintf("没有找到关于 %s 的搜索结果", query), nil
			}

			var sb strings.Builder
			for i, r := range result.Results {
				fmt.Fprintf(&sb, "【%d】\n标题：%s\n内容：%s\n来源：%s\n", i+1, r.Title, r.Content, r.URL)
			}

			text := sb.String()
			runes := []rune(text)
			if len(runes) > 3000 {
				text = string(runes[:3000]) + "...(已截断)"
			}
			return text, nil
		},
	}
}
