package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// —— 消息类型 ——
// statusMsg 表示一条搜索状态，会追加到 history
type statusMsg string

// answerMsg 表示最终返回的 JSON 报告
type answerMsg string

// —— 样式 ——
var (
	appTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	historyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true)
	hintStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	queryTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("250"))
	loadingStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))

	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)
	cursorStyle = lipgloss.NewStyle().Background(lipgloss.Color("212"))

	borderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	labelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	iconSalary   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	iconSkill    = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	iconTrend    = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	iconAdvice   = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	iconConflict = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

// entry 表示一次已完成的查询记录
type entry struct {
	input   string   // 用户输入的岗位名
	history []string // 这次的搜索步骤
	report  string   // 这次的报告
}

// model 是 Bubble Tea 的状态
type model struct {
	input     string
	history   []string // 当前这一轮的搜索步骤
	report    string   // 当前这一轮的报告
	loading   bool
	program   *tea.Program
	entries   []entry // 已归档的历史查询
	lastInput string  // 当前这一轮的岗位名

	viewport viewport.Model // 中间可滚动区域
	ready    bool           // 是否已收到窗口尺寸
	width    int            // 终端宽
	height   int            // 终端高
}

// 固定区域占用行数
const (
	headerHeight = 2 // 标题 + 提示
	footerHeight = 4 // 输入框(3) + 提示(1)
)

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.viewport = viewport.New(msg.Width, 0)
			m.viewport.YPosition = headerHeight
			m.ready = true
		}
		// 尺寸变化会改变 viewport 可用高度
		m.resizeViewport()
		m.viewport.SetContent(m.renderContent())
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd:
			// 滚动键交给 viewport
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.input != "" && !m.loading {
				// 上一轮报告已完成：先归档，再开始新一轮
				if m.report != "" {
					m.entries = append(m.entries, entry{
						input:   m.lastInput,
						history: m.history,
						report:  m.report,
					})
				}
				m.lastInput = m.input
				userInput := m.input
				m.input = ""
				m.report = ""
				m.history = nil
				m.loading = true
				m.resizeViewport()
				m.refreshContent()
				m.viewport.GotoBottom()
				return m, runAgentCmd(userInput, m.program)
			}
		case tea.KeyBackspace:
			if !m.loading {
				r := []rune(m.input)
				if len(r) > 0 {
					m.input = string(r[:len(r)-1])
				}
			}
		case tea.KeyRunes, tea.KeySpace:
			if !m.loading {
				m.input += msg.String()
			}
		}

	case tea.MouseMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case statusMsg:
		m.history = append(m.history, string(msg))
		m.refreshContent()
		m.viewport.GotoBottom()

	case answerMsg:
		m.report = string(msg)
		m.loading = false
		m.resizeViewport()
		m.refreshContent()
		m.viewport.GotoBottom()
	}

	return m, nil
}

func (m *model) View() string {
	if !m.ready {
		return "\n  初始化中..."
	}

	var b strings.Builder

	// 顶部固定：标题 + 提示
	b.WriteString(appTitleStyle.Render("🤖 岗位分析 Agent") + "\n")
	b.WriteString(hintStyle.Render("输入岗位名称，回车分析 · ↑↓/PgUp/PgDn/滚轮 翻看历史") + "\n")

	// 中间滚动区：viewport
	b.WriteString(m.viewport.View() + "\n")

	// 底部固定：输入框 + 提示
	if m.loading {
		b.WriteString(loadingStyle.Render("⏳ 正在分析中…") + "\n")
	}
	prompt := "› " + m.input
	if !m.loading {
		prompt += cursorStyle.Render(" ")
	}
	b.WriteString(inputBoxStyle.Render(prompt) + "\n")
	b.WriteString(hintStyle.Render("回车发送 · Ctrl+C 退出"))

	return b.String()
}

// renderContent 拼出 viewport 里显示的全部内容：归档记录 + 当前轮
func (m *model) renderContent() string {
	var b strings.Builder

	// 已归档的历史查询
	for _, e := range m.entries {
		b.WriteString(queryTitleStyle.Render("── 查询：" + e.input + " ──"))
		b.WriteString("\n")
		for _, h := range e.history {
			b.WriteString(historyStyle.Render("  " + h))
			b.WriteString("\n")
		}
		if e.report != "" {
			b.WriteString(renderReport(e.report))
			b.WriteString("\n\n")
		}
	}

	// 当前这一轮：搜索步骤与报告
	if m.lastInput != "" {
		b.WriteString(queryTitleStyle.Render("── 查询：" + m.lastInput + " ──"))
		b.WriteString("\n")
	}
	for _, h := range m.history {
		b.WriteString(historyStyle.Render("  " + h))
		b.WriteString("\n")
	}
	if len(m.history) > 0 {
		b.WriteString("\n")
	}
	if m.report != "" {
		b.WriteString(renderReport(m.report))
		b.WriteString("\n")
	}

	return b.String()
}

// resizeViewport 按终端尺寸与底部固定区高度算出 viewport 可用高度
// loading 时底部会多出一行“正在分析中”，需一并扣除，避免整屏溢出
func (m *model) resizeViewport() {
	if !m.ready {
		return
	}
	extra := 0
	if m.loading {
		extra = 1
	}
	h := m.height - headerHeight - footerHeight - extra
	if h < 1 {
		h = 1
	}
	m.viewport.Width = m.width
	m.viewport.Height = h
}

// refreshContent 把最新内容同步到 viewport
func (m *model) refreshContent() {
	if m.ready {
		m.viewport.SetContent(m.renderContent())
	}
}

// runAgentCmd 在后台运行 Agent，状态通过 program 推给 UI
func runAgentCmd(userInput string, p *tea.Program) tea.Cmd {
	return func() tea.Msg {
		report := RunAgent(userInput, func(s string) {
			if p != nil {
				p.Send(statusMsg(s))
			}
		})
		return answerMsg(report)
	}
}

func main() {
	m := &model{}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.program = p
	if _, err := p.Run(); err != nil {
		fmt.Println("启动失败:", err)
	}
}

// —— 报告渲染 ——

// jobReport 对应 system prompt 里约定的 JSON 结构
type jobReport struct {
	Job      string   `json:"job"`
	Salary   string   `json:"salary"`
	Skills   []string `json:"skills"`
	Trend    string   `json:"trend"`
	Advice   string   `json:"advice"`
	Conflict string   `json:"conflict"`
}

type row struct {
	icon  string
	label string
	value string
}

// 卡片尺寸：边框之间总宽 56，卡片总宽 58 列
const (
	boxInner   = 56
	contentW   = boxInner - 2          // 正文可用宽 54
	rowPrefixW = 9                     // emoji(2) + 空格(1) + 标签(4) + 两个空格(2)
	rowValueW  = contentW - rowPrefixW // 值宽度 45
)

func renderReport(jsonStr string) string {
	// 容错：模型可能输出 markdown 代码块或额外解释文字
	cleaned := strings.TrimSpace(jsonStr)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```JSON")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	// 兜底：从第一个 { 截到最后一个 }，剥离围栏与前后解释文字
	if i := strings.Index(cleaned, "{"); i >= 0 {
		if j := strings.LastIndex(cleaned, "}"); j > i {
			cleaned = cleaned[i : j+1]
		}
	}
	cleaned = strings.TrimSpace(cleaned)

	var r jobReport
	if err := json.Unmarshal([]byte(cleaned), &r); err != nil {
		// 对话模式的纯文本回复：按普通样式折行渲染，不当成错误
		return strings.Join(wrapText(cleaned, 46), "\n")
	}

	rows := []row{
		{iconSalary.Render("💰"), "薪资", r.Salary},
		{iconSkill.Render("🛠"), "技能", strings.Join(r.Skills, " · ")},
		{iconTrend.Render("📈"), "趋势", r.Trend},
		{iconAdvice.Render("🎯"), "建议", r.Advice},
	}
	if r.Conflict != "" {
		rows = append(rows, row{iconConflict.Render("⚠️"), "风险", r.Conflict})
	}

	lines := make([]string, 0, len(rows))
	indent := strings.Repeat(" ", rowPrefixW) // 续行缩进，和值列对齐
	for _, rw := range rows {
		for i, v := range wrapText(rw.value, rowValueW) {
			var text string
			if i == 0 {
				// 第一行：emoji + 标签 + 值首行
				text = fmt.Sprintf("%s %s  %s", rw.icon, padRight(labelStyle.Render(rw.label), 4), v)
			} else {
				// 续行：缩进对齐到值列
				text = indent + v
			}
			lines = append(lines, borderStyle.Render("│")+" "+padRight(text, contentW)+" "+borderStyle.Render("│"))
		}
	}

	// 标题嵌进上边框
	titlePlain := "📊 " + r.Job
	dashes := boxInner - 2 - 1 - lipgloss.Width(titlePlain)
	if dashes < 0 {
		dashes = 0
	}
	top := borderStyle.Render("┌─ ") + titleStyle.Render(titlePlain) + " " +
		borderStyle.Render(strings.Repeat("─", dashes)+"┐")
	bottom := borderStyle.Render("└" + strings.Repeat("─", boxInner) + "┘")

	return top + "\n" + strings.Join(lines, "\n") + "\n" + bottom
}

// padRight 用空格把 s 补到可见宽度 w（ANSI 安全）
func padRight(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// wrapText 按可见宽度把文本折成多行：中文按宽度 2、ASCII 按 1，超过 maxW 即换行
func wrapText(s string, maxW int) []string {
	if s == "" {
		return []string{""}
	}

	var lines []string
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > maxW && w > 0 {
			lines = append(lines, b.String())
			b.Reset()
			w = 0
		}
		b.WriteRune(r)
		w += rw
	}
	lines = append(lines, b.String())

	return lines
}
