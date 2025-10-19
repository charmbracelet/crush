# Claude Automanage - Crush Mk2 Scaffold

This file helps maintain consistency across the Crush Mk2 (Crush → Toke) integration scaffold located in `/plan/`.

## Project Context

This is the Crush Mk2 project - integrating Charmbracelet Crush with LM Studio and Qwen3-8B for local AI-powered development workflows.

**Key Components:**
- **Crush**: Terminal-based AI coding assistant (base project)
- **LM Studio**: Local LLM inference server
- **Qwen3-8B**: Local language model (4/5-bit quantized for RTX 4080)
- **Reasoning Layer**: Task management + AI integration scripts

## Directory Structure

```
/plan/
├── plan.md                           # Main architecture document
├── lm_studio_config/
│   ├── README.md                     # LM Studio setup guide
│   └── crush_config.json             # Crush config for LM Studio
├── powershell_wrappers/
│   ├── send_prompt.ps1               # Send prompts to LM Studio
│   ├── list_models.ps1               # List available models
│   └── README.md                     # PowerShell wrapper docs
└── crush_scripts/
    ├── reasoning_layer.ps1           # Task management + AI reasoning
    └── README.md                     # Reasoning layer docs
```

## Scaffold Principles

### 1. Everything in /plan/ is experimental
- Keep all integration code under `/plan/` until stable
- Don't modify core Crush files during experimentation
- Document findings and learnings in plan.md

### 2. Local-first approach
- No external API calls for AI features
- All inference runs through LM Studio locally
- Optimize for RTX 4080 (16 GB VRAM) constraints

### 3. PowerShell as the glue
- Windows-first development environment
- PowerShell scripts for API integration
- Easy to test and modify without rebuilding

### 4. Incremental integration
- Phase 1: Setup and structure ✅
- Phase 2: LM Studio + Qwen validation
- Phase 3: Test reasoning layer
- Phase 4: Refine and merge stable pieces

## Maintenance Guidelines

### When updating /plan/ contents:

1. **Update plan.md** if architecture changes
2. **Update READMEs** if commands/usage changes
3. **Test scripts** before committing
4. **Document LM Studio configs** when adding new models
5. **Keep .claude/automanage.md** updated with new patterns

### File consistency checks:

- All PowerShell scripts have proper error handling
- All scripts use consistent LM Studio URL (`http://localhost:1234/v1`)
- All scripts default to `qwen3-8b` model ID
- All README files include troubleshooting sections

### When adding new features:

1. Create under `/plan/` first
2. Document in appropriate README
3. Test with LM Studio running
4. Add to plan.md checklist
5. Update this file if it introduces new patterns

## Common Patterns

### LM Studio API Call Pattern

```powershell
$body = @{
    model = "qwen3-8b"
    messages = @(@{ role = "user"; content = $prompt })
    temperature = 0.7
    max_tokens = 2048
} | ConvertTo-Json -Depth 10

$response = Invoke-RestMethod -Uri "http://localhost:1234/v1/chat/completions" `
    -Method Post `
    -Body $body `
    -ContentType "application/json"

$content = $response.choices[0].message.content
```

### Error Handling Pattern

```powershell
try {
    # Operation
} catch {
    Write-Host "Error: " -ForegroundColor Red
    Write-Host $_.Exception.Message -ForegroundColor Red
    exit 1
}
```

### Task Storage Pattern

- JSON file in project root: `.crush_tasks.json`
- Auto-initialize if doesn't exist
- Include created/updated timestamps
- Support for tags and priorities

## Configuration Management

### LM Studio Settings (Centralized)

**Base URL:** `http://localhost:1234/v1`
**Default Model:** `qwen3-8b`
**Default Temperature:** 0.3 (code) / 0.7 (general)
**Max Tokens:** 2048 (default) / 4096 (code gen)

### Crush Integration

Place `crush_config.json` in one of these locations:
1. `.crush.json` (project root)
2. `crush.json` (project root)
3. `~/.config/crush/crush.json` (global)

Use the config from `/plan/lm_studio_config/crush_config.json` as a template.

## Testing Checklist

Before considering /plan/ code stable:

- [ ] LM Studio is running and accessible
- [ ] PowerShell wrappers execute without errors
- [ ] `list_models.ps1` returns available models
- [ ] `send_prompt.ps1` generates responses
- [ ] Reasoning layer loads without errors
- [ ] Task management functions work (add/list/update/remove)
- [ ] AI reasoning produces useful breakdowns
- [ ] Code generation produces valid code
- [ ] All scripts handle errors gracefully
- [ ] Documentation is complete and accurate

## Known Limitations

1. **Windows-only**: PowerShell scripts designed for Windows
   - Future: Consider cross-platform alternatives (Python, Go)

2. **LM Studio dependency**: Requires LM Studio running
   - Could abstract to support other local inference servers

3. **Manual model management**: User must download/configure models
   - Future: Automate model recommendations

4. **No persistent context**: Each AI call is independent
   - Future: Implement conversation history

5. **Limited error recovery**: Scripts fail fast on errors
   - Future: Add retry logic and graceful degradation

## Future Enhancements

### Short-term (in /plan/)
- Add conversation history to reasoning layer
- Support for multiple models
- Better task subtask management
- Export tasks to external formats (Markdown, Jira, etc.)

### Medium-term (merge to main)
- Integrate stable pieces into Crush codebase
- Add Crush commands for task management
- LSP integration for code context

### Long-term (Toke evolution)
- Multi-agent workflows
- Visual task board (TUI)
- Project templates
- Team collaboration features

## References

- **Crush Repo**: https://github.com/charmbracelet/crush
- **LM Studio**: https://lmstudio.ai/
- **Qwen Models**: https://huggingface.co/Qwen
- **Plan Document**: `/plan/plan.md`

## Notes

- This file should be updated as the scaffold evolves
- Keep it focused on maintaining consistency
- Document patterns, not implementation details
- Update when adding new scripts or changing architecture

---

**Last Updated**: 2025-01-15
**Scaffold Version**: 1.0
**Status**: Initial setup complete, ready for testing
