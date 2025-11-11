# Crush Usage Examples & Cookbook

**Comprehensive guide to using Crush in various scenarios**

---

## Table of Contents

1. [Interactive Mode](#interactive-mode)
2. [Programmatic Mode](#programmatic-mode)
3. [Git Integration](#git-integration)
4. [Automation & CI/CD](#automation--cicd)
5. [Code Analysis](#code-analysis)
6. [Testing Workflows](#testing-workflows)
7. [Best Practices](#best-practices)

---

## Interactive Mode

### Basic Usage

```bash
# Launch Crush in interactive mode
crush

# Start in a specific directory
crush -c /path/to/project

# Enable debug mode
crush -d

# Auto-approve all permissions (dangerous)
crush -y
```

### In-Session Commands

Once in interactive mode, you can:

```
# Ask questions
> How do I implement a binary search in Go?

# Request code analysis
> Review the code in main.go and suggest improvements

# Generate code
> Create a new REST API endpoint for user registration

# Refactor code
> Refactor the getUserById function to use dependency injection

# Switch models mid-conversation
Ctrl+M  # Opens model selector

# Switch sessions
Ctrl+R  # Opens session browser

# Access command palette
Ctrl+K  # Opens command palette
```

---

## Programmatic Mode

### One-Shot Prompts

```bash
# Simple prompt
crush -p "Explain how goroutines work"

# Code generation
crush -p "Generate a Dockerfile for this Node.js application"

# Code review
crush -p "Review main.go and identify potential bugs"

# Quiet mode (no spinner)
crush -p -q "Analyze test coverage"
```

### Piping Input

```bash
# Analyze code from stdin
cat main.go | crush -p "Review this code for security issues"

# Process multiple files
cat *.go | crush -p "Find all TODO comments and create GitHub issues"

# Combine with other tools
git diff | crush -p "Summarize these changes in plain English"

# Error analysis
npm test 2>&1 | crush -p "Analyze these test failures and suggest fixes"
```

### Scripting Examples

```bash
#!/bin/bash
# Code review script

echo "Running AI code review..."

for file in $(git diff --name-only main); do
  echo "Reviewing $file..."
  cat "$file" | crush -p -q "Review this file and rate from 1-10"
done
```

---

## Git Integration

### AI-Powered Commits

```bash
# Stage changes and generate commit message
git add .
crush git commit

# Stage all and commit in one command
crush git commit --all

# Commit with custom message
crush git commit "Add user authentication feature"

# Review staged changes before committing
crush git diff --staged --analyze
```

### Code Review Workflow

```bash
# Review your changes
crush git diff --analyze

# Check what's changed
crush git status

# See commit history with AI insights
crush git log -n 20 --analyze

# Compare branches
git diff main..feature-branch | crush -p "Summarize the differences"
```

### Undo & Fix Mistakes

```bash
# Undo last commit (keep changes staged)
crush git undo

# Undo and unstage changes
crush git undo --unstage

# Fix commit message
crush git undo
crush git commit "Better commit message"
```

### Remote Operations

```bash
# Push to remote
crush git push origin main

# Force push (use with caution)
crush git push --force

# Pull latest changes
crush git pull origin main
```

---

## Automation & CI/CD

### GitHub Actions Example

```yaml
name: AI Code Review
on: [pull_request]

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Install Crush
        run: |
          npm install -g @charmland/crush

      - name: Run AI Review
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
        run: |
          git diff origin/main...HEAD | \
            crush -p -q "Review these changes and identify issues" > review.md

      - name: Post Review
        run: gh pr comment --body-file review.md
```

### GitLab CI Example

```yaml
ai-review:
  stage: test
  script:
    - npm install -g @charmland/crush
    - git diff origin/main...HEAD |
        crush -p -q "Analyze code quality" > report.txt
  artifacts:
    paths:
      - report.txt
```

### Pre-Commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit

echo "Running AI pre-commit checks..."

# Get staged changes
CHANGES=$(git diff --cached)

if [ -n "$CHANGES" ]; then
  # Check for security issues
  echo "$CHANGES" | crush -p -q "Check for security vulnerabilities" > /tmp/security-check.txt

  if grep -q "CRITICAL" /tmp/security-check.txt; then
    echo "Critical security issues found!"
    cat /tmp/security-check.txt
    exit 1
  fi
fi
```

### Automated Testing & Fixing

```bash
#!/bin/bash
# Run tests and auto-fix failures

MAX_ITERATIONS=3
ITERATION=0

while [ $ITERATION -lt $MAX_ITERATIONS ]; do
  echo "Test iteration $((ITERATION + 1))..."

  # Run tests
  TEST_OUTPUT=$(npm test 2>&1)

  if [ $? -eq 0 ]; then
    echo "All tests passed!"
    exit 0
  fi

  # Ask AI to fix failures
  echo "$TEST_OUTPUT" | crush -p --yolo "Fix these test failures" > /dev/null

  ITERATION=$((ITERATION + 1))
done

echo "Max iterations reached. Manual intervention needed."
exit 1
```

---

## Code Analysis

### Security Audit

```bash
# Analyze entire codebase
crush -p "Perform a security audit of this codebase and identify OWASP top 10 vulnerabilities"

# Check specific file
cat auth.js | crush -p "Check for authentication vulnerabilities"

# Analyze dependencies
cat package.json | crush -p "Identify outdated or vulnerable dependencies"
```

### Performance Analysis

```bash
# Profile code
cat slow-function.go | crush -p "Identify performance bottlenecks"

# Database query optimization
cat queries.sql | crush -p "Optimize these SQL queries for PostgreSQL"

# Memory leak detection
crush -p "Analyze memory usage patterns in the application"
```

### Code Quality

```bash
# Check code smells
crush -p "Review the codebase and identify code smells"

# Complexity analysis
cat complex-module.js | crush -p "Calculate cyclomatic complexity and suggest refactoring"

# Documentation review
crush -p "Identify undocumented functions and generate JSDoc comments"
```

---

## Testing Workflows

### Generate Tests

```bash
# Generate unit tests
cat src/utils.js | crush -p "Generate comprehensive unit tests using Jest"

# Generate integration tests
crush -p "Create integration tests for the user authentication API"

# Generate E2E tests
crush -p "Create Cypress E2E tests for the checkout flow"
```

### Test Analysis

```bash
# Analyze test coverage
npm test -- --coverage | crush -p "Analyze test coverage and suggest improvements"

# Fix failing tests
npm test 2>&1 | crush -p "Fix these failing tests"

# Generate test data
crush -p "Generate realistic test data for user model"
```

### TDD Workflow

```bash
# 1. Write failing test
crush -p "Write a failing test for getUserById function"

# 2. Implement feature
crush -p "Implement getUserById to make the test pass"

# 3. Refactor
crush -p "Refactor getUserById while keeping tests green"
```

---

## Best Practices

### Effective Prompts

```bash
# ❌ Vague prompt
crush -p "fix my code"

# ✅ Specific prompt
crush -p "Fix the null pointer exception in line 42 of UserService.java"

# ❌ Too broad
crush -p "make it better"

# ✅ Clear objective
crush -p "Refactor the authentication module to follow SOLID principles"
```

### Context Matters

```bash
# Provide context via stdin
cat README.md architecture.md | crush -p "Based on this architecture, design a new payment module"

# Use project-specific context
crush -c ./microservices/auth -p "Add rate limiting to the login endpoint"
```

### Safety First

```bash
# Review changes before applying
crush git diff --analyze  # Review first
crush git commit           # Then commit

# Use quiet mode in scripts
crush -p -q "..."  # Easier to parse output

# Avoid --yolo in production
crush --yolo  # ❌ Dangerous in production
crush         # ✅ Review each action
```

### Session Management

```bash
# Use separate sessions for different tasks
# Session 1: Feature development
# Session 2: Bug fixing
# Session 3: Code review

# Switch between sessions with Ctrl+R

# Keep sessions focused
# Don't mix unrelated work in one session
```

### Performance Tips

```bash
# Use appropriate models
# Large model for complex tasks
# Small model for simple queries

# Limit context size
# Include only relevant files
# Use .crushignore for large files

# Enable debug when troubleshooting
crush -d  # Debug mode
crush logs --follow  # Watch logs
```

---

## Common Workflows

### Feature Development

```bash
# 1. Create feature branch
git checkout -b feature/user-notifications

# 2. Develop with Crush
crush -p "Implement email notification system"

# 3. Generate tests
crush -p "Generate tests for notification system"

# 4. Review changes
crush git diff --analyze

# 5. Commit with AI message
crush git commit --all

# 6. Push
crush git push origin feature/user-notifications
```

### Bug Fixing

```bash
# 1. Reproduce bug
npm test  # Run failing test

# 2. Analyze failure
npm test 2>&1 | crush -p "Why is this test failing?"

# 3. Fix with AI assistance
crush -p "Fix the bug in calculateTax function that causes negative values"

# 4. Verify fix
npm test

# 5. Commit
crush git commit "Fix tax calculation bug"
```

### Code Review

```bash
# Review PR changes
git fetch origin pull/123/head:pr-123
git checkout pr-123

# AI review
crush git diff main..pr-123 --analyze > review.md

# Leave comments
cat review.md  # Review AI feedback
```

### Refactoring

```bash
# 1. Identify refactoring opportunities
crush -p "Analyze this module and suggest refactoring opportunities"

# 2. Plan refactoring
crush -p "Create a step-by-step refactoring plan for the UserService"

# 3. Execute refactoring
crush -p "Refactor UserService to separate concerns"

# 4. Verify tests still pass
npm test

# 5. Commit
crush git commit "Refactor UserService to improve maintainability"
```

---

## Advanced Tips

### Multi-File Operations

```bash
# Refactor across multiple files
find src -name "*.js" | xargs cat | \
  crush -p "Rename all instances of 'getUserData' to 'fetchUserData'"
```

### Integration with Other Tools

```bash
# Combine with grep
grep -r "TODO" . | crush -p "Convert these TODOs into GitHub issues with descriptions"

# Combine with find
find . -name "*.test.js" -mtime +90 | \
  crush -p "These tests haven't been updated in 90 days. Analyze if they're still relevant"

# Combine with awk
git log --oneline -n 100 | awk '{print $2}' | \
  crush -p "Analyze these commit messages and suggest improvements to our commit message style"
```

### Custom Scripts

```bash
#!/bin/bash
# Daily code health check

echo "=== Daily Code Health Report ===" > report.md
echo "" >> report.md

# Security check
echo "## Security" >> report.md
crush -p -q "Quick security scan of modified files in the last 24 hours" >> report.md

# Code quality
echo "## Code Quality" >> report.md
git diff HEAD~1 | crush -p -q "Assess code quality of recent changes" >> report.md

# Test coverage
echo "## Test Coverage" >> report.md
npm test -- --coverage 2>&1 | crush -p -q "Summarize test coverage" >> report.md

# Email report
mail -s "Daily Code Health Report" dev-team@example.com < report.md
```

---

## Troubleshooting

### Common Issues

```bash
# Issue: "No providers configured"
# Solution: Run interactive mode first to configure
crush  # Configure provider interactively

# Issue: Permission denied errors
# Solution: Check file permissions or use --yolo (carefully)
crush --yolo -p "..."  # Use with caution

# Issue: Context too large
# Solution: Use .crushignore or limit input
echo "node_modules/" >> .crushignore

# Issue: Unexpected output format
# Solution: Use -q flag for cleaner output
crush -p -q "..." | grep "ERROR"
```

### Debug Mode

```bash
# Enable debug logging
crush -d

# View logs in real-time
crush logs --follow

# View last 1000 log lines
crush logs --tail 1000

# Check data directories
crush dirs
```

---

## Resources

- [README](README.md) - Main documentation
- [Feature Comparison](FEATURE_COMPARISON_2025.md) - Competitive analysis
- [Configuration Guide](README.md#configuration) - Detailed config options
- [GitHub Issues](https://github.com/charmbracelet/crush/issues) - Report bugs
- [Discord](https://charm.land/discord) - Community support

---

**Happy Crushing! 💘**
