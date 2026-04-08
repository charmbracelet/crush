// Copyright 2026 CrushCL. All rights reserved.
//
// TaskClassifier - 任務分類器
// 負責分析任務並分類到合適的執行者

package coordination

import (
	"regexp"
	"strings"
	"unicode"
)

// TaskClassifierImpl 任務分類器實現
type TaskClassifierImpl struct {
	patterns map[TaskType][]*regexp.Regexp
}

// NewTaskClassifierImpl 創建新的任務分類器
func NewTaskClassifierImpl() *TaskClassifierImpl {
	c := &TaskClassifierImpl{
		patterns: make(map[TaskType][]*regexp.Regexp),
	}

	c.addPattern(TaskQuickLookup, `(?i)^(what is|who is|define|explain quickly|look up|how to|how do|explain what|what does|what do)`)
	c.addPattern(TaskQuickLookup, `(?i)^(list files|get the|find the|check the) `)
	c.addPattern(TaskQuickLookup, `(?i)^what is (the )?\w+ (command|tool|software|program|application|command-line|cli)`)
	c.addPattern(TaskQuickLookup, `(?i)^what (is|does) \w+ (command|tool|software|program|application)`)
	c.addPattern(TaskQuickLookup, `(?i)^how to \w+ (a |an |the )?\w+`)
	c.addPattern(TaskQuickLookup, `(?i)^how do i \w+`)
	c.addPattern(TaskQuickLookup, `(?i)^explain what \w+ (is|means|does)`)

	// FileOperation patterns
	c.addPattern(TaskFileOperation, `(?i)(cat|head|tail|ls|dir|rm|mkdir|touch|chmod|chown|cd|pushd|popd)\s*`)
	c.addPattern(TaskFileOperation, `(?i)(show me|create|update|remove|delete|edit|read|write|modify|copy|move)\s+(a |the |an )?[^\s]+`)

	c.addPattern(TaskDataProcessing, `(?i)(parse|process|convert|transform|extract)`)
	c.addPattern(TaskDataProcessing, `(?i)(count|sum|average|filter|sort)`)

	c.addPattern(TaskGitHub, `(?i)(git |github|gitlab|commit|branch|push|pull request|pr |merge|checkout|stash)`)
	c.addPattern(TaskGitHub, `(?i)(check (the )?(git|github|gitlab|commit|branch) (status|log|history)|show (me )?my (git|github) (status|log))`)

	c.addPattern(TaskComplexRefactor, `(?i)(refactor|restructure|rewrite|rearchitect|migrate)`)
	c.addPattern(TaskComplexRefactor, `(?i)(redesign|optimize|performance|rest api|graphql)`)

	c.addPattern(TaskCodeReview, `(?i)(review|audit|check.*code|analyze.*code|code review)`)
	c.addPattern(TaskCodeReview, `(?i)(security.*scan|lint|static.*analysis|check code quality)`)

	c.addPattern(TaskBugHunt, `(?i)(find.*bug|fix.*bug|debug|error.*in|exception|null pointer)`)
	c.addPattern(TaskBugHunt, `(?i)(crash|panic|segmentation fault|sigsegv|bug in)`)

	c.addPattern(TaskCreative, `(?i)(write.*story|create.*design|generate.*idea|give me.*idea)`)
	c.addPattern(TaskCreative, `(?i)(brainstorm|design.*new|propose.*solution|creative)`)

	c.addPattern(TaskMCPTask, `(?i)(mcp|mcp\.|model context protocol|use.*tool|search.*web|query.*database|filesystem.*read)`)
	c.addPattern(TaskMCPTask, `(?i)(server|tool.*integration|connect.*to|mcp tool)`)

	return c
}

func (c *TaskClassifierImpl) addPattern(t TaskType, pattern string) {
	c.patterns[t] = append(c.patterns[t], regexp.MustCompile(pattern))
}

// Classify 分類任務
func (c *TaskClassifierImpl) Classify(task string) TaskClassification {
	scores := make(map[TaskType]int)
	totalMatches := 0

	// 評分
	for taskType, patterns := range c.patterns {
		for _, p := range patterns {
			if p.MatchString(task) {
				scores[taskType]++
				totalMatches++
			}
		}
	}

	// 找到最高分（確定性 tie-breaking：優先選擇較高 TaskType 值，確保更具體的分類優先）
	var bestType TaskType
	bestScore := 0
	for t, s := range scores {
		if s > bestScore || (s == bestScore && t > bestType) {
			bestScore = s
			bestType = t
		}
	}

	// 計算置信度
	confidence := 0.0
	if totalMatches > 0 {
		confidence = float64(bestScore) / float64(totalMatches)
	}

	// 決定執行者
	executor := c.decideExecutor(bestType, confidence)

	// 估算成本
	cost := c.estimateCost(bestType, confidence)

	// 確定工具
	tools := c.determineTools(bestType, task)

	return TaskClassification{
		TaskType:     bestType,
		Executor:     executor,
		Confidence:   confidence,
		CostEstimate: cost,
		Reason:       c.formatReason(bestType, bestScore, totalMatches),
		Tools:        tools,
	}
}

func (c *TaskClassifierImpl) decideExecutor(t TaskType, confidence float64) ExecutorType {
	switch {
	case t == TaskQuickLookup && confidence > 0.5:
		return ExecutorCL
	case t == TaskFileOperation && confidence > 0.5:
		return ExecutorCL
	case t == TaskDataProcessing && confidence > 0.5:
		return ExecutorCL
	case t == TaskGitHub && confidence > 0.6:
		return ExecutorClaudeCode
	case t == TaskComplexRefactor:
		return ExecutorClaudeCode
	case t == TaskCodeReview:
		return ExecutorClaudeCode
	case t == TaskBugHunt && confidence > 0.5:
		return ExecutorClaudeCode
	case t == TaskMCPTask:
		return ExecutorClaudeCode
	default:
		if confidence > 0.7 {
			return ExecutorCL
		}
		return ExecutorClaudeCode
	}
}

func (c *TaskClassifierImpl) estimateCost(t TaskType, confidence float64) float64 {
	// 基礎成本（美元）
	baseCosts := map[TaskType]float64{
		TaskUnknown:         0.001,
		TaskQuickLookup:     0.001,
		TaskFileOperation:   0.002,
		TaskDataProcessing:  0.003,
		TaskGitHub:          0.05,
		TaskComplexRefactor: 0.10,
		TaskCodeReview:      0.05,
		TaskBugHunt:         0.08,
		TaskCreative:        0.03,
		TaskMCPTask:         0.05,
	}

	base, ok := baseCosts[t]
	if !ok {
		base = 0.01
	}

	// 根據置信度調整
	return base * (0.5 + confidence*0.5)
}

func (c *TaskClassifierImpl) determineTools(t TaskType, task string) []string {
	defaultTools := []string{"Read", "Bash", "Grep", "Glob"}

	switch t {
	case TaskGitHub:
		return []string{"Bash", "Read", "Grep"}
	case TaskFileOperation:
		return []string{"Read", "Write", "Edit", "Bash"}
	case TaskDataProcessing:
		return []string{"Read", "Bash", "Write"}
	case TaskCodeReview:
		return []string{"Read", "Grep", "Glob"}
	case TaskBugHunt:
		return []string{"Read", "Grep", "Glob", "Bash"}
	default:
		return defaultTools
	}
}

func (c *TaskClassifierImpl) formatReason(t TaskType, score, total int) string {
	if score == 0 {
		return "No patterns matched"
	}
	s := t.String()
	if len(s) > 0 {
		s = string(unicode.ToUpper(rune(s[0]))) + s[1:]
	}
	return strings.TrimSpace(strings.ReplaceAll(s, "Task", "")) + " task"
}

// TaskTypeString returns the string representation of a TaskType
func (t TaskType) String() string {
	switch t {
	case TaskUnknown:
		return "unknown"
	case TaskQuickLookup:
		return "quick_lookup"
	case TaskFileOperation:
		return "file_operation"
	case TaskDataProcessing:
		return "data_processing"
	case TaskGitHub:
		return "github"
	case TaskComplexRefactor:
		return "complex_refactor"
	case TaskCodeReview:
		return "code_review"
	case TaskBugHunt:
		return "bug_hunt"
	case TaskCreative:
		return "creative"
	case TaskMCPTask:
		return "mcp_task"
	default:
		return "unknown"
	}
}
