package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"tui/agentOpenai/core"
	"tui/agentOpenai/tools"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

const MaxLoop = 10

// getLlmTools 获取工具
func getLlmTools() map[string]*tools.ToolDef {
	llmTools := make(map[string]*tools.ToolDef)
	//weatherTool := tools.NewGetRealWeather()
	//wardrobeTool := tools.NewGetWardrobe()
	//currentTimeTool := tools.NewGetCurrentTime()
	//llmTools[weatherTool.Name] = weatherTool
	webSearchTool := tools.NewGetWebSearch()
	shell := tools.NewExecShell()
	readFileTool := tools.NewReadLocalFile()
	openBrowserTool := tools.NewOpenBrowser()
	llmTools[webSearchTool.Name] = webSearchTool
	//llmTools[wardrobeTool.Name] = wardrobeTool
	//llmTools[currentTimeTool.Name] = currentTimeTool
	llmTools[shell.Name] = shell
	llmTools[readFileTool.Name] = readFileTool
	llmTools[openBrowserTool.Name] = openBrowserTool

	// 加载 mcp.json 中配置的 MCP 服务器工具（未配置或单个服务器失败都不影响内置工具）
	mcpTools, err := tools.LoadMCPTools(context.Background(), "mcp.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "MCP 加载失败：%v\n", err)
	}
	for name, def := range mcpTools {
		llmTools[name] = def
	}
	return llmTools
}

// buildOpenaiTools 生成openai格式工具
func buildOpenaiTools(tools map[string]*tools.ToolDef) []responses.ToolUnionParam {
	openaiTools := make([]responses.ToolUnionParam, 0)
	for _, tool := range tools {
		openaiTools = append(openaiTools, tool.ToParam())
	}
	return openaiTools
}

// systemPrompt 岗位分析助手 / 对话助手的系统提示词
const systemPrompt = `你是一个岗位分析助手，同时也能进行友好对话。

判断规则：
- 如果用户输入的是岗位名称（如 "Go 后端开发"、"产品经理"、"厨师"）→ 走分析模式
- 如果用户输入的是闲聊、打招呼、提问（如 "你好"、"你是谁"、"谢谢"）→ 走对话模式

分析模式：
1. 自己决定搜索什么关键词，获取岗位市场信息
2. 至少搜索 2 次，覆盖：薪资、技能、趋势
3. 最后只输出一个 JSON 对象，格式如下：
{
  "job": "岗位名称",
  "salary": "薪资范围",
  "skills": ["技能1", "技能2"],
  "trend": "行业趋势",
  "advice": "求职建议",
  "conflict": "风险提示"
}

对话模式：
- 直接输出纯文本回复，不要输出 JSON
- 简洁友好，不要长篇大论

本地文件：
- 如果用户要求你分析他的个人简历（如 resume.txt）或本地文档，请调用 read_local_file 工具。

浏览器：
- 你有能力直接打开用户的浏览器。如果用户说“打开网页”，请调用 open_browser 工具。
- 如果你决定调用 open_browser 工具，绝对不要只打开网站首页。你必须根据用户的需求，生成带有搜索参数的 URL。例如：如果要查 BOSS 直聘的 Go 岗位，应该生成 https://www.zhipin.com/web/geek/job?query=Go后端 并传入。打开后，请告诉用户：已为您打开具体的搜索页面。

多工具协同：
- 如果你决定调用 open_browser 或 browser_open_url 工具打开网页，你不能就此停止任务。打开网页只是为了让用户直观查看。在打开网页后，你必须紧接着调用搜索工具（web_search），去获取用户真正需要的核心信息（如天气数据、薪资数据），并将获取到的结果在终端里以纯文本或 JSON 的形式完整汇报给用户。绝对不要只说一句"网页已打开"就结束。

要求：
- skills 数组 5-8 项
- 其他字段每项不超过 60 字
- 不要编造数据
- 分析模式下不要输出 JSON 以外的任何内容
- 只采用 2024 年及以后发布的信息；无法判断时间的数据标注"时间未知"`

// globalSession 全局会话，跨轮保留对话记忆
var globalSession = core.NewSession()

// trimSession 限制会话长度：永远保留第一条（system prompt），其余只留最后 maxMessages-1 条
func trimSession(maxMessages int) {
	msgs := globalSession.Messages
	if len(msgs) <= maxMessages {
		return
	}
	kept := make(responses.ResponseInputParam, 0, maxMessages)
	kept = append(kept, msgs[0])
	kept = append(kept, msgs[len(msgs)-(maxMessages-1):]...)
	globalSession.Messages = kept
}

// RunAgent 运行 Agent 主循环：userInput 为用户输入，onStatus 在每次搜索前回调状态，返回最终输出文本
func RunAgent(userInput string, onStatus func(string)) string {
	// 首轮写入 system prompt，之后复用已有记忆
	if len(globalSession.Messages) == 0 {
		globalSession.AddMessage(
			responses.ResponseInputItemParamOfMessage(systemPrompt, responses.EasyInputMessageRoleSystem),
		)
	}
	globalSession.AddMessage(
		responses.ResponseInputItemParamOfMessage(userInput, responses.EasyInputMessageRoleUser),
	)

	// 无论成功失败，返回前都裁剪会话长度
	defer trimSession(40)

	llmTools := getLlmTools()
	openaiTools := buildOpenaiTools(llmTools)

	ctx := context.Background()

	client := openai.NewClient(
		option.WithAPIKey(os.Getenv("ARK_API_KEY")), // defaults to os.LookupEnv("OPENAI_API_KEY")
		option.WithBaseURL("https://ark.cn-beijing.volces.com/api/v3"),
		option.WithRequestTimeout(30*time.Second),
		//option.WithJSONSet("thinking.type", "disabled"),
	)

	for realLoop := 0; realLoop <= MaxLoop; realLoop++ {
		resp, err := client.Responses.New(ctx, responses.ResponseNewParams{
			Input: responses.ResponseNewParamsInputUnion{
				OfInputItemList: globalSession.GetMessages(),
			},
			Tools: openaiTools,
			Model: "ep-20261022110104-2wf6f",
			//Temperature: openai.Float(0.1),
		})

		if err != nil {
			var apiErr *openai.Error
			if errors.As(err, &apiErr) {
				return fmt.Sprintf("请求失败：HTTP %d %s\n%s", apiErr.StatusCode, apiErr.Type, apiErr.RawJSON())
			}
			return fmt.Sprintf("请求失败：%v", err)
		}

		hasToolCall := false
		for _, item := range resp.Output {
			switch item.Type {
			case "function_call":
				hasToolCall = true
				//火山引擎 处理
				globalSession.AddMessage(responses.ResponseInputItemUnionParam{
					OfFunctionCall: &responses.ResponseFunctionToolCallParam{
						Arguments: item.AsFunctionCall().Arguments,
						CallID:    item.AsFunctionCall().CallID,
						Name:      item.AsFunctionCall().Name,
					},
				})

				if v, ok := llmTools[item.AsFunctionCall().Name]; ok {
					// 每次调用 web_search 前回调搜索状态
					if item.AsFunctionCall().Name == "web_search" && onStatus != nil {
						var a map[string]any
						if json.Unmarshal([]byte(item.AsFunctionCall().Arguments), &a) == nil {
							if q, ok := a["query"].(string); ok {
								onStatus("🔍 正在搜索: " + q)
							}
						}
					}

					if (item.AsFunctionCall().Name == "open_browser" || item.AsFunctionCall().Name == "browser_open_url") && onStatus != nil {
						onStatus("🌐 正在为你打开浏览器...")
					}
					if item.AsFunctionCall().Name == "read_local_file" && onStatus != nil {
						onStatus("📄 正在读取本地文件...")
					}

					var toolRe string
					var err error
					if toolRe, err = v.Call(ctx, item.AsFunctionCall().Arguments); err != nil {
						toolRe = fmt.Sprintf("%s%v", item.AsFunctionCall().Name, err)
					}

					globalSession.AddMessage(responses.ResponseInputItemUnionParam{
						OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
							Output: responses.ResponseInputItemFunctionCallOutputOutputUnionParam{
								OfString: openai.String(toolRe),
							},
							CallID: openai.String(item.AsFunctionCall().CallID),
						},
					})
				}
			}
		}

		if !hasToolCall {
			return resp.OutputText()
		}
	}

	return ""
}
