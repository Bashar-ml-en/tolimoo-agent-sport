-- A new process cannot resume old in-memory jobs. Clear any historical active
-- rows before adding the active-run invariant, so existing databases migrate.
UPDATE runs
SET status = 'failed', ended_at = started_at, error = 'Interrupted by a previous process shutdown.'
WHERE status IN ('queued', 'running');

-- At most one queued or running research job may exist for an assignment.
CREATE UNIQUE INDEX IF NOT EXISTS idx_runs_one_active_per_agent
ON runs(agent_id)
WHERE status IN ('queued', 'running');
