# ADR-001: GitHub as Backbone

## Status

Proposed

## Context

Orchestra needs a persistence and collaboration layer. Options include:

1. **GitHub** - Issues, PRs, Milestones, Discussions, Projects
2. **GitLab** - Similar to GitHub but self-hostable
3. **Local-only** - File-based state in `.orchestra/`
4. **External tools** - Linear, Jira, Notion integrations

We need visibility into agent work, human oversight, and team collaboration.

## Decision

**Use GitHub as the backbone for Orchestra.**

Entity mapping:
- Specs → YAML files in `specs/`
- Phases → GitHub Milestones
- Tasks → GitHub Issues
- Agent work → Branches and PRs
- Communication → Issue/PR comments
- Discussions → GitHub Discussions

## Consequences

### Positive

- Native git integration (branches, PRs, reviews)
- Built-in visibility (Issues, Projects, activity feeds)
- Familiar to developers
- Webhooks for real-time updates
- Mobile app for monitoring

### Negative

- Locks users into GitHub
- Requires GitHub API access
- Rate limits may apply
- Not usable offline

### Mitigations

- Cache state locally in `.orchestra/`
- Sync when connection restored
- Abstract GitHub client for future GitLab support
