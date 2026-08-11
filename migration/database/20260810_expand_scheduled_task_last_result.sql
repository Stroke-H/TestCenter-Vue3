-- Prevent verbose retry diagnostics from rolling back the scheduler's
-- status/next-run checkpoint.
ALTER TABLE `scheduled_tasks`
  MODIFY COLUMN `last_result` TEXT NULL;
