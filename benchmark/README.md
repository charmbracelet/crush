# Cliffy vs Crush Benchmarks

Performance comparison between Cliffy and Crush using identical configurations.

## Quick Start

```bash
cd benchmark
./bench.sh
```

This runs all tests 5 times each and saves results to `results/bench_TIMESTAMP.json`.

Generate a markdown report:

```bash
./report.sh results/bench_TIMESTAMP.json > results/report.md
```

## Test Categories

**Cold Start** (< 5 seconds)
- File operations, simple searches
- Shows Cliffy's startup advantage most clearly

**Medium Tasks** (5-15 seconds)
- Code generation, translations, testing
- Cold start matters but becomes smaller percentage

**Long Running** (30+ seconds)
- Full implementations, refactoring, API design
- Tests hypothesis: cold start overhead becomes negligible

## Configuration

Both tools use identical settings from `config/`:
- Same model: x-ai/grok-4-fast:free via OpenRouter
- Same max_tokens: 16384 (model supports 2M context)
- No LSP or MCP servers (apples to apples)

Crush runs with: `--data-dir config -q -y`
Cliffy runs with: `--quiet` (uses config/ automatically)

## What We Measure

- **Cold start**: Time from command to first token
- **Total time**: Full execution duration
- **Speedup**: Crush time / Cliffy time

Each test runs 5 times. Results include mean and standard deviation.

## Requirements

- `jq` for JSON processing
- `bc` for calculations
- `crush` installed and in PATH
- `cliffy` binary at `../bin/cliffy`
- `CLIFFY_OPENROUTER_API_KEY` environment variable set

## Results

Check `results/` for timestamped benchmark runs and generated reports.

ᕕ( ᐛ )ᕗ  Ready when you are.
