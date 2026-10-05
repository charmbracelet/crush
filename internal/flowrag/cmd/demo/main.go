package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/crush/internal/flowrag"
)

func main() {
	ctx := context.Background()

	home, _ := os.UserHomeDir()
	storePath := filepath.Join(home, ".crush", "flowrag", "demo_workflows.json")
	os.MkdirAll(filepath.Dir(storePath), 0755)

	embClient := flowrag.NewHashEmbeddingClient(256)
	store, err := flowrag.NewFileVectorStore(storePath, embClient)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create store: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║       FlowRAG 交互式 Demo                    ║")
	fmt.Println("║   基于 trigram 哈希的语义向量检索              ║")
	fmt.Println("╠══════════════════════════════════════════════╣")
	fmt.Println("║  命令:                                       ║")
	fmt.Println("║    add <描述>    — 添加一个工作流             ║")
	fmt.Println("║    search <查询> — 语义搜索相似工作流          ║")
	fmt.Println("║    list          — 列出所有已保存工作流       ║")
	fmt.Println("║    demo          — 加载示例数据               ║")
	fmt.Println("║    help          — 显示帮助                   ║")
	fmt.Println("║    quit          — 退出                       ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("flowrag> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		cmd := strings.ToLower(parts[0])
		arg := ""
		if len(parts) > 1 {
			arg = parts[1]
		}

		switch cmd {
		case "help":
			printHelp()

		case "demo":
			loadDemo(ctx, store)

		case "list":
			listWorkflows(ctx, store)

		case "add":
			if arg == "" {
				fmt.Println("用法: add <描述文本>")
				fmt.Println("例如: add 修复了用户登录认证的密码比对bug")
				continue
			}
			addWorkflow(ctx, store, arg)

		case "search":
			if arg == "" {
				fmt.Println("用法: search <查询文本>")
				fmt.Println("例如: search 怎么修登录bug")
				continue
			}
			searchWorkflows(ctx, store, arg)

		case "quit", "exit", "q":
			fmt.Println("再见!")
			return

		default:
			fmt.Printf("未知命令: %s (输入 help 查看帮助)\n", cmd)
		}
		fmt.Println()
	}
}

func printHelp() {
	fmt.Println()
	fmt.Println("FlowRAG 通过 trigram 哈希将文本转为向量，存入向量库。")
	fmt.Println("相似语义的文本会产生相似的向量 → 余弦相似度检索。")
	fmt.Println()
	fmt.Println("试试这些例子:")
	fmt.Println("  demo                    — 加载6条不同领域的示例工作流")
	fmt.Println("  add 写了个Python脚本解析CSV生成报表")
	fmt.Println("  search 怎么用python处理csv数据    — 语义搜索")
	fmt.Println("  search 数据库迁移怎么做           — 看看能不能找到迁移工作流")
	fmt.Println("  search docker部署postgres         — 找到Docker相关工作流")
}

func loadDemo(ctx context.Context, store *flowrag.VectorStore) {
	demos := []struct {
		id     string
		prompt string
		steps  []flowrag.WorkflowStep
	}{
		{
			id:     "auth-login-fix",
			prompt: "修复了用户登录认证模块中密码比对逻辑颠倒的bug",
			steps: []flowrag.WorkflowStep{
				{Role: "tool_call", Tool: "read", Input: `auth.go`},
				{Role: "tool_result", Tool: "read", Output: "发现密码比较用了!=应该是=="},
				{Role: "tool_call", Tool: "edit", Input: `auth.go - 修复比较运算符`},
				{Role: "tool_result", Tool: "edit", Output: "已修复，密码验证恢复正常"},
			},
		},
		{
			id:     "rest-api-register",
			prompt: "创建了用户注册的REST API接口，包含邮箱验证和路由注册",
			steps: []flowrag.WorkflowStep{
				{Role: "tool_call", Tool: "read", Input: `routes.go`},
				{Role: "tool_result", Tool: "read", Output: "找到现有路由结构"},
				{Role: "tool_call", Tool: "write", Input: `handler_register.go - POST /api/users`},
				{Role: "tool_result", Tool: "write", Output: "创建注册接口成功"},
				{Role: "tool_call", Tool: "edit", Input: `routes.go - 注册新路由`},
				{Role: "tool_result", Tool: "edit", Output: "路由注册完成"},
			},
		},
		{
			id:     "python-csv-report",
			prompt: "用Python写了一个CSV数据解析脚本，能自动生成PDF报表",
			steps: []flowrag.WorkflowStep{
				{Role: "tool_call", Tool: "write", Input: `report_parser.py`},
				{Role: "tool_result", Tool: "write", Output: "基于pandas的CSV解析器"},
				{Role: "tool_call", Tool: "write", Input: `requirements.txt`},
				{Role: "tool_result", Tool: "write", Output: "添加pandas、reportlab依赖"},
				{Role: "tool_call", Tool: "bash", Input: `python report_parser.py`},
				{Role: "tool_result", Tool: "bash", Output: "报表生成成功，150条记录"},
			},
		},
		{
			id:     "db-migration-users",
			prompt: "为users表新增了数据库迁移脚本，添加了email和password_hash字段",
			steps: []flowrag.WorkflowStep{
				{Role: "tool_call", Tool: "read", Input: `migrations/目录`},
				{Role: "tool_result", Tool: "read", Output: "已有2个迁移文件"},
				{Role: "tool_call", Tool: "write", Input: `003_add_users_table.sql`},
				{Role: "tool_result", Tool: "write", Output: "CREATE TABLE users迁移已创建"},
				{Role: "tool_call", Tool: "bash", Input: `执行数据库迁移`},
				{Role: "tool_result", Tool: "bash", Output: "迁移执行成功"},
			},
		},
		{
			id:     "oauth-token-fix",
			prompt: "修复了OAuth中间件中token过期后不自动刷新的问题",
			steps: []flowrag.WorkflowStep{
				{Role: "tool_call", Tool: "read", Input: `middleware/auth.go`},
				{Role: "tool_result", Tool: "read", Output: "发现过期token检测逻辑"},
				{Role: "tool_call", Tool: "edit", Input: `middleware/auth.go`},
				{Role: "tool_result", Tool: "edit", Output: "添加了过期前5分钟主动刷新"},
			},
		},
		{
			id:     "docker-postgres-redis",
			prompt: "搭建了本地开发的Docker Compose环境，包含PostgreSQL和Redis",
			steps: []flowrag.WorkflowStep{
				{Role: "tool_call", Tool: "write", Input: `docker-compose.yml`},
				{Role: "tool_result", Tool: "write", Output: "创建compose文件：postgres:16 + redis:7"},
				{Role: "tool_call", Tool: "write", Input: `.env.example`},
				{Role: "tool_result", Tool: "write", Output: "环境变量配置完成"},
				{Role: "tool_call", Tool: "bash", Input: `docker compose up -d`},
				{Role: "tool_result", Tool: "bash", Output: "服务启动成功:5432, :6379"},
			},
		},
	}

	for _, d := range demos {
		record := flowrag.WorkflowRecord{
			ID:         d.id,
			UserPrompt: d.prompt,
			Steps:      d.steps,
			StepsText:  buildStepsText(d.prompt, d.steps),
			SessionID:  "demo",
		}
		if err := store.Insert(ctx, record); err != nil {
			fmt.Printf("  ✗ 插入失败 %s: %v\n", d.id, err)
			return
		}
		fmt.Printf("  ✓ 已加载: %s\n", d.prompt)
	}
	fmt.Printf("\n  共加载 %d 条工作流，现在可以用 search 搜索了！\n", len(demos))
}

func buildStepsText(prompt string, steps []flowrag.WorkflowStep) string {
	var sb strings.Builder
	sb.WriteString("任务: ")
	sb.WriteString(prompt)
	for _, s := range steps {
		sb.WriteString("\n")
		sb.WriteString(s.Role)
		sb.WriteString(": ")
		sb.WriteString(s.Tool)
		if s.Output != "" {
			sb.WriteString(" -> ")
			sb.WriteString(s.Output)
		}
	}
	return sb.String()
}

func addWorkflow(ctx context.Context, store *flowrag.VectorStore, desc string) {
	record := flowrag.WorkflowRecord{
		ID:         fmt.Sprintf("manual-%d", store.Count()+1),
		UserPrompt: desc,
		StepsText:  "任务: " + desc,
		SessionID:  "manual",
	}
	if err := store.Insert(ctx, record); err != nil {
		fmt.Printf("  ✗ 添加失败: %v\n", err)
		return
	}
	fmt.Printf("  ✓ 已添加: %s\n", desc)
	fmt.Printf("  当前共 %d 条工作流\n", store.Count())
}

func searchWorkflows(ctx context.Context, store *flowrag.VectorStore, query string) {
	fmt.Printf("\n  搜索: \"%s\"\n\n", query)

	results, err := store.Search(ctx, query, 5)
	if err != nil {
		fmt.Printf("  ✗ 搜索失败: %v\n", err)
		return
	}

	if len(results) == 0 {
		fmt.Println("  (没有找到相似工作流，先 demo 加载示例数据吧)")
		return
	}

	for i, r := range results {
		marker := "  "
		if i == 0 {
			marker = "→ "
		}
		fmt.Printf("  %s[%d] %s\n", marker, i+1, r.UserPrompt)
		if len(r.Steps) > 0 {
			stepCount := 0
			for _, s := range r.Steps {
				if s.Role == "tool_call" {
					if stepCount == 0 {
						fmt.Printf("      工具链: ")
					} else {
						fmt.Print(" → ")
					}
					fmt.Print(s.Tool)
					stepCount++
				}
			}
			if stepCount > 0 {
				fmt.Println()
			}
		}
	}
}

func listWorkflows(ctx context.Context, store *flowrag.VectorStore) {
	count := store.Count()
	if count == 0 {
		fmt.Println("  (没有工作流，输入 demo 加载示例数据)")
		return
	}
	fmt.Printf("\n  已保存 %d 条工作流:\n\n", count)

	results, _ := store.Search(ctx, "list all", count)
	for i, r := range results {
		fmt.Printf("  [%d] %s\n", i+1, r.UserPrompt)
	}

	fmt.Printf("\n  试试: search <关键词>  来语义搜索\n")
	fmt.Println("  例如: search 登录认证bug")
}
