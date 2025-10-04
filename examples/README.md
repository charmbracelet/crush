# Cliffy Examples

This directory contains example files demonstrating various cliffy features.

## Task Files

### tasks-simple.txt
Line-separated tasks with comments:
```bash
cliffy --tasks-file examples/tasks-simple.txt
```

### tasks.json
JSON array format tasks:
```bash
cliffy --json --tasks-file examples/tasks.json
```

## STDIN Examples

### Line-separated from stdin:
```bash
cat examples/tasks-simple.txt | cliffy -
```

### JSON from stdin:
```bash
cat examples/tasks.json | cliffy --json -
```

### Shell pipeline:
```bash
# Generate tasks dynamically
find . -name "*.go" -type f | head -5 | \
  xargs -I {} echo "count the lines in {}" | \
  cliffy -
```

### From echo:
```bash
echo -e "task1\ntask2\ntask3" | cliffy -
```

## Advanced Usage

### With shared context:
```bash
cliffy --context "You are a math expert" --tasks-file examples/tasks-simple.txt
```

### With verbose output:
```bash
cliffy --verbose --tasks-file examples/tasks-simple.txt
```

### JSON output for automation:
```bash
cliffy --output-format json --tasks-file examples/tasks.json | jq .
```
