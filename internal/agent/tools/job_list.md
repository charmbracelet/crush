List background jobs started by this session, running and finished.

<usage>
- `running_only`: set true to hide jobs that have already finished. Defaults
  to false.
</usage>

<features>
- Each row is the job ID, its status (running with elapsed time, completed, or
  failed with an exit code), and its description.
- Metadata carries the full command and the transcript path for each job.
</features>

<tips>
- Use this to recover a job ID you no longer have in view, then read it with
  `job_output` or stop it with `job_kill`.
- Worth a look before starting a long command, in case an equivalent job is
  already running.
</tips>
