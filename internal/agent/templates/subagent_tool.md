Invoke a user-defined subagent with specialized capabilities. Subagents are custom agents defined in markdown files with YAML frontmatter that specify their system prompt, tool access, and permission controls.

<usage>
- Use this tool when you need to invoke a specialized agent for a specific task
- Each subagent has its own system prompt optimized for particular use cases (e.g., code review, test generation, documentation)
- Subagents have full access to all tools by default, unless explicitly restricted in their definition
- Subagents can have pre-approved tools (no permission prompts) or run in yolo mode
</usage>

<usage_notes>
1. The `subagent` parameter is required - you must specify which subagent to invoke
2. Launch multiple subagents concurrently when tasks are independent
3. Each subagent invocation is stateless - include all necessary context in the prompt
4. The subagent's response is returned to you, not directly visible to the user
5. Subagents inherit the parent session's model configuration
</usage_notes>

