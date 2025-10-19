# Crush Mk2 Plan (Crush → Toke)

## LM Studio + Crush + Qwen Integration Scaffold

### Summary

This proposal adds a self-contained plan scaffold to integrate Charmbracelet Crush with LM Studio using Qwen3-8B locally (optimized for RTX 4080 / 16 GB via 4/5-bit quantization). It also introduces a minimal reasoning/task-list layer so CLI tasks can call the local LLM for next-step planning and code generation.

## What's Included (under /plan/)

- **plan.md** - This file (architecture + scaffold description)
- **lm_studio_config/** - Model download + config for Qwen3-8B
- **powershell_wrappers/** - Windows helpers: send_prompt.ps1, list_models.ps1
- **crush_scripts/** - reasoning_layer.crush for TODOs + AI integration
- **.claude/** - automanage.md to keep scaffolds consistent

## Why

- **LM Studio** gives a local, API-compatible LLM endpoint (no cloud calls)
- **Qwen3-8B** fits on RTX 4080 (16 GB) with 4/5-bit quant, leaving headroom
- **Crush** becomes a task orchestrator while Claude (or local Qwen) suggests code/steps

## Highlights

- No external APIs for code gen—everything runs locally (LM Studio + Qwen)
- Scripts remain in /plan for easy experimentation before merging upstream
- Skeleton reasoning_layer.crush + PowerShell helpers to get started quickly

## Phase Overview

### Phase 1 - Clone & Structure

Use Claude Code terminal (VSCode/Cursor) to create a clean workspace and clone upstream crush:

```bash
mkdir ~/workspace/crush-mk2
cd ~/workspace/crush-mk2
git clone https://github.com/charmbracelet/crush.git
cd crush
```

Keep /plan in this repo for docs + scaffolding only.

### Phase 2 - LM Studio + Qwen (on RTX 4080)

Download LM Studio and install Qwen3-8B with 4-bit or 5-bit quantization
Start local server on http://localhost:1234/v1/chat/completions

**PowerShell Wrapper (stub):**

```powershell
$prompt = "Generate a ROS2 launch file for dual CSI cameras"
$body = @{
    model = "qwen3-8b"
    messages = @(@{ role = "user"; content = $prompt })
    temperature = 0.7
} | ConvertTo-Json

Invoke-RestMethod -Uri http://localhost:1234/v1/chat/completions `
    -Method Post -Body $body -ContentType "application/json"
```

### Phase 3 - Reasoning / Task-List Layer (Crush)

**/plan/crush_scripts/reasoning_layer.crush** provides:

Commands: task:add, task:list, task:remove, and task:ai:reason

It forwards collated TODOs to LM Studio for step breakdowns / code suggestions

.claude/automanage.md keeps the scaffold consistent over time

## How to Test (Quick)

1. Run Crush: `crush reasoning_layer.crush`
2. Add task: `task:add "Generate ROS2 camera launch"` → task-list
3. AI reasoning: `task:ai:reason` → expect a stepwise plan via LM Studio (local)
4. Adjust send_prompt.ps1 to your port/model name if needed

## Checklist

- [x] Document /plan structure and content
- [x] Validate LM Studio running at http://localhost:1234
- [ ] Test reasoning_layer.crush basic commands
- [ ] Refine prompts for code gen quality
- [ ] Merge stable pieces from /plan into the main tree later

## Next Steps

1. **Validate LM Studio** - Ensure it's running and accessible
2. **Test PowerShell wrappers** - Verify API communication
3. **Implement reasoning_layer.crush** - Create task management commands
4. **Integrate with Crush** - Test the full workflow
5. **Refine and iterate** - Improve based on testing results

---

**Note:** This is a development scaffold. Production integration will require additional security, error handling, and configuration management.
