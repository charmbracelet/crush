# Crush Branding Audit

ᕕ( ᐛ )ᕗ  Ensuring complete Cliffy identity while honoring our roots

## Our Relationship with Crush

**Cliffy adores Crush.** This fork exists because Crush is excellent at what it does - interactive, session-based AI coding with a beautiful TUI. Cliffy is optimized for a different pattern: fast one-off tasks, automation, and CLI-first workflows.

We're not competing with Crush. We're a specialized variant, like a sprint runner vs a marathon runner. Both athletes, both excellent, different events.

**Key principle:** When we remove Crush branding, we do it to give Cliffy its own identity, not to distance ourselves from an amazing project we respect deeply.

## Status

Initial branding updates complete (HTTP headers, git attribution, User-Agent strings). This document tracks remaining references to audit and clean up.

## Completed Changes

- ✓ HTTP headers: `X-Title: Cliffy`, `HTTP-Referer: https://cliffy.ettio.com`
- ✓ Git commit attribution: `ᕕ( ᐛ )ᕗ  Generated with Cliffy`, `Co-Authored-By: Cliffy <cliffy@ettio.com>`
- ✓ User-Agent strings: Changed from `crush/1.0` to `cliffy/1.0`
- ✓ ASCII art library created for terminal-safe branding
- ✓ README updated with Cliffy personality

## Areas to Audit

### 1. Code Comments and Documentation

**Search pattern:**
```bash
grep -ri "crush" internal/ cmd/ --include="*.go" | grep -i "\/\/"
```

**Action items:**
- [ ] Review inline comments that reference "Crush" conceptually
- [ ] Update package documentation strings
- [ ] Check for Crush-specific behavior explanations
- [ ] Update copyright headers if present

**Guideline:** It's fine to say "forked from Crush" or "inspired by Crush's approach." We just don't want users confused about which tool they're using.

### 2. Configuration Files

**Files to check:**
- [ ] `.cliffy.json` - Already uses Cliffy naming ✓
- [ ] Sample configs in documentation
- [ ] Test fixtures and mock configs

**Search command:**
```bash
find . -name "*.json" -o -name "*.yaml" -o -name "*.toml" | xargs grep -i "crush"
```

### 3. Error Messages and User-Facing Strings

**Search pattern:**
```bash
grep -r "fmt\\..*crush" internal/ cmd/ --include="*.go" -i
grep -r "errors\\..*crush" internal/ cmd/ --include="*.go" -i
```

**Priority:** High - Users should never see "Crush failed to..." in Cliffy

**Check for:**
- [ ] Error messages that say "Crush failed to..."
- [ ] Help text that references Crush
- [ ] Log messages with Crush branding

### 4. Internal Package Names

**Current state:** Using `internal/` structure inherited from Crush

**Assessment:** Package names are generic (`llm`, `config`, `tools`, `lsp`) - no Crush-specific names found. This is good! It means the architecture is clean and reusable.

**No action needed** unless we discover Crush-specific packages.

### 5. Git History and Attribution

**Current approach:** Keeping full git history from Crush fork

**This is the right call.** It:
- Shows provenance and respects original authors
- Keeps context for why decisions were made
- Makes it easier to pull improvements back
- Demonstrates we're building on excellent foundations

**Recommendation:**
- Keep history intact
- Add prominent ATTRIBUTION.md celebrating the Crush codebase
- Mention in README that Cliffy is a specialized fork of Crush

### 6. Dependencies and Module Names

**Current module:** Need to verify go.mod has correct module path

**Check:**
- [ ] Verify go.mod module path is cliffy-specific
- [ ] Update any hardcoded import paths
- [ ] Ensure references to `charmbracelet/crush` are appropriate (like in comments explaining architecture)

### 7. External References

**Where Cliffy identifies itself:**
- OpenRouter API calls ✓
- Anthropic API calls (via catwalk) - uses catwalk's headers
- Other provider calls - should identify as Cliffy

**Note:** It's totally fine if internal dependencies use Crush code. We're just ensuring the public-facing identity is clear.

## Systematic Audit Commands

Run these to find remaining references:

```bash
# Case-insensitive search for "crush" in Go files
grep -ri "crush" cmd/ internal/ --include="*.go" | grep -v "go.sum"

# Check for "charm" references (original org - totally fine to keep in comments!)
grep -ri "charm" cmd/ internal/ --include="*.go" | grep -v "go.mod" | grep -v "go.sum"

# Find Crush in markdown docs
find . -name "*.md" -exec grep -l -i "crush" {} \;

# Check test files specifically
find . -name "*_test.go" -exec grep -l -i "crush" {} \;
```

## Branding Consistency Checklist

Ensure these user-facing touchpoints are all "Cliffy" not "Crush":

- [x] Binary name: `cliffy`
- [x] Config directory: `~/.config/cliffy/`
- [x] Config file: `cliffy.json`
- [x] Environment variables: `CLIFFY_*`
- [x] HTTP headers: Cliffy
- [x] Git attribution: Cliffy
- [x] User-Agent: cliffy/1.0
- [x] ASCII art: ᕕ( ᐛ )ᕗ
- [x] Email domain: cliffy@ettio.com
- [x] Website: cliffy.ettio.com
- [ ] Log messages: (audit needed)
- [ ] Error messages: (audit needed)
- [ ] Package comments: (audit needed)

## Philosophy Questions

### 1. Attribution in User Docs

**Question:** Should README mention Crush?

**Recommendation:** Yes! Brief, celebratory mention:
```markdown
## Credits

Cliffy is a specialized fork of [Crush](https://github.com/charmbracelet/crush),
the excellent interactive AI coding assistant. Crush excels at session-based
work with a beautiful TUI. Cliffy optimizes for fast one-off tasks and automation.
```

### 2. Config Compatibility

**Question:** Maintain config compatibility with Crush?

**Current:** Yes - identical structure, can share configs

**Recommendation:** Keep compatible unless we have a compelling reason to diverge. This lets users easily switch between tools for different workflows.

### 3. Issue/PR References

**Question:** How do we handle "this works in Crush but not Cliffy"?

**Approach:**
- Acknowledge gratefully
- Evaluate if it fits Cliffy's one-off, fast-execution use case
- If yes: implement and consider contributing back to Crush if applicable
- If no: explain the design difference respectfully

### 4. Credit in Code

**Question:** Keep Crush references in code comments?

**Recommendation:** Absolutely! Comments like "uses Crush's tool architecture" or "adapted from Crush's agent system" are perfect. They:
- Give credit where it's due
- Help future developers understand design decisions
- Show we're building on proven patterns

## Next Actions

**Phase 1 - Immediate (This Week)**
1. Run audit commands and document findings
2. Create ATTRIBUTION.md celebrating Crush
3. Update README with friendly fork mention
4. Fix any user-facing error messages that say "Crush"

**Phase 2 - Short Term (This Month)**
1. Ensure all HTTP headers and external identifiers are Cliffy
2. Update any user-facing strings in tools
3. Add section to docs explaining Crush relationship
4. Set up process for tracking Crush updates we want to pull

**Phase 3 - Ongoing**
1. Update internal comments as we touch files (low priority)
2. Keep celebrating Crush when we implement their ideas
3. Maintain config compatibility unless there's a reason to break it

**Phase 4 - Before 1.0**
1. Complete audit of all user-facing strings
2. Ensure documentation clearly explains when to use Cliffy vs Crush
3. Consider contributing performance improvements back to Crush

## Success Criteria

✓ Users never confused about which tool they're using
✓ Cliffy has clear identity in API calls and attribution
✓ We openly celebrate building on Crush's excellent foundation
✓ Code comments properly credit Crush architecture and patterns
✓ Config remains compatible for easy workflow switching

## The Bottom Line

We're not trying to hide that Cliffy is built on Crush. We're trying to give Cliffy its own identity while being incredibly grateful for the amazing codebase we forked from.

Think of it like this: Crush built an incredible race car. Cliffy is taking that engine and putting it in a different chassis optimized for sprint races. We're not saying our chassis is better - it's just built for a different track.

ᕕ( ᐛ )ᕗ  Built on Crush's excellence, optimized for speed
