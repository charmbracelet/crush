---
name: jq
description: Process, filter, transform, generate, and construct JSON data using the jq command-line processor. Crush includes a built-in jq implementation (powered by gojq) available in the bash tool. Use when the user needs to query, filter, reshape, extract, create, or construct JSON — including API responses, config files, log output, or any structured data. Also use when generating JSON from scratch with jq -n, or when helping users write or debug jq expressions.
---

# jq — Built-in JSON Processor

Crush ships a built-in `jq` command (via `github.com/itchyny/gojq`) available
in the bash tool. No external binary is required.

## Supported Flags

| Flag | Description |
|------|-------------|
| `-r`, `--raw-output` | Output strings without quotes |
| `-j`, `--join-output` | Like `-r` but no trailing newline |
| `-c`, `--compact-output` | One-line JSON output |
| `-s`, `--slurp` | Read all inputs into an array |
| `-n`, `--null-input` | Use `null` as input (ignore stdin) |
| `-e`, `--exit-status` | Exit 1 if last output is `false` or `null` |
| `-R`, `--raw-input` | Read each line as a string, not JSON |
| `--arg name value` | Bind `$name` to a string value |
| `--argjson name value` | Bind `$name` to a parsed JSON value |

File arguments after the filter are also supported: `jq '.foo' file.json`.

## Differences from Standard jq

The built-in uses gojq, which is a pure-Go jq implementation. Key
differences:

- **No object key ordering** — keys are sorted by default; `keys_unsorted`
  and `-S` are unavailable.
- **Arbitrary-precision integers** — large integers keep full precision
  (addition, subtraction, multiplication, modulo, division when divisible).
- **String indexing** — `"abcde"[2]` returns `"c"`.
- **Not supported** — `--ascii-output`, `--seq`, `--stream`,
  `--stream-errors`, `-f`/`--from-file`, `--slurpfile`, `--rawfile`,
  `--args`, `--jsonargs`, `input_line_number`, `$__loc__`, some regex
  features (backreferences, look-around).
- **YAML** — gojq supports `--yaml-input`/`--yaml-output` but the
  built-in does not currently expose these flags.

## Common Patterns

Extract a field:
```sh
echo '{"name":"crush"}' | jq '.name'
```

Filter an array:
```sh
echo '[1,2,3,4,5]' | jq '[.[] | select(. > 3)]'
```

Reshape objects:
```sh
echo '{"first":"Ada","last":"Lovelace"}' | jq '{full: (.first + " " + .last)}'
```

Use variables:
```sh
echo '{}' | jq --arg host localhost --argjson port 8080 '{host: $host, port: $port}'
```

Slurp multiple JSON values:
```sh
echo '{"a":1}{"b":2}' | jq -s '.'
```

Compact output for piping:
```sh
echo '{"a":1}' | jq -c '.a += 1'
```

Raw string output:
```sh
echo '["one","two","three"]' | jq -r '.[]'
```

Process a file:
```sh
jq '.dependencies | keys' package.json
```

Null input for constructing JSON:
```sh
jq -n --arg msg hello '{"message": $msg}'
```

## Tips

- Pipe jq output to other commands: `jq -r '.url' data.json | xargs curl`
- Chain filters with `|` inside the expression, not shell pipes.
- Use `try` to suppress errors on missing keys: `jq 'try .foo.bar'`
- Use `// "default"` for fallback values: `jq '.name // "unknown"'`
- Use `@csv`, `@tsv`, `@base64`, `@html`, `@uri` for format strings.
