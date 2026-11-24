{{- if .JobReports -}}
Get stdout/stderr from a background shell by ID; returns whatever the job has printed so far. A finished job reports back by itself with a `<job-report>`, so use this to check on a job that is still running, such as a server's startup log, not to collect its final result. Set wait=true only when blocking on the job is what you want; a wait returns after at most 2 minutes, and ending your turn and letting the report wake you is usually better.
{{- else -}}
Get stdout/stderr from a background shell by ID; returns whatever the job has printed so far. Call it when you need the job's result, not right after starting the job, and set wait=true only when blocking on the job is what you want. A wait returns after at most 2 minutes, with status running if the job is still going; keep working on other things and wait again only if nothing else is left to do.
{{- end }}
