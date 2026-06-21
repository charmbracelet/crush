ISSUES=$(gh issue list --state=all --limit=1000 --json "number" -t '{{range .}}{{printf "%.0f\n" .number}}{{end}}')
PRS=$(gh pr list --state=all --limit=1000 --json "number" -t '{{range .}}{{printf "%.0f\n" .number}}{{end}}')

for issue in $ISSUES; do
  echo "Dispatching labeler.yml for $issue"
  gh workflow run labeler.yml -f issue-number="$issue"
done

for pr in $PRS; do
  echo "Dispatching labeler.yml for $pr"
  gh workflow run labeler.yml -f issue-number="$pr"
done
