Save one durable fact to long-term memory, so it survives this session.

Memory already records the conversation on its own. This tool is for the rare
statement worth promoting above the transcript: something you would want to know
on day one of a new session six months from now.

<usage>
- `content`: a single complete sentence, standalone and self-explanatory. It will
  be read with no surrounding context, so "the user prefers that" is useless.
  Content longer than 600 characters is shortened.
</usage>

<worth_saving>
- Decisions with their reasoning: "Kieran chose SQLite over Postgres for this
  project because it ships as a single binary and the data is single-writer."
- Stable preferences: "Kieran wants table-driven tests with testify require, not
  bare if-err checks."
- Project conventions a newcomer would get wrong: "In this repo, tool
  descriptions live in sibling .md files and are embedded with go:embed."
- Gotchas learned the hard way: "gofumpt crashes on this repo's Go 1.27 generic
  methods; use gofmt instead."
- Constraints that outlive the task: "Never push or open PRs without asking."
</worth_saving>

<not_worth_saving>
- Transient state: what file is open, what test is failing right now, what you
  are about to do next
- Anything already in this conversation. The transcript is recorded anyway; do
  not duplicate it.
- Facts with a short shelf life: version numbers, branch names, open bug counts,
  "the build is broken"
- Guesses and inferences you have not confirmed. A wrong conclusion outlives the
  session that produced it and misleads every session after.
- Secrets, tokens, keys, or anything from a credentials file
</not_worth_saving>

<rule_of_thumb>
If it will probably be false next week, do not save it. If the user had to
explain it to you and would be annoyed explaining it again, save it.
</rule_of_thumb>
