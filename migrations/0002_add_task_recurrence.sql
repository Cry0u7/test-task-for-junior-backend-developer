ALTER TABLE tasks
ADD COLUMN IF NOT EXISTS recurrence_type TEXT,
ADD COLUMN IF NOT EXISTS recurrence_every_n_days INT,
ADD COLUMN IF NOT EXISTS recurrence_month_days INT[],
ADD COLUMN IF NOT EXISTS recurrence_dates DATE[],
ADD COLUMN IF NOT EXISTS recurrence_parity TEXT,
ADD COLUMN IF NOT EXISTS recurrence_start_date DATE;

CREATE INDEX IF NOT EXISTS idx_tasks_recurrence_type
    ON tasks (recurrence_type);

CREATE INDEX IF NOT EXISTS idx_tasks_recurrence_start_date
    ON tasks (recurrence_start_date);

ALTER TABLE tasks
ADD CONSTRAINT tasks_recurrence_type_chk
CHECK (
    recurrence_type IS NULL
    OR recurrence_type IN ('daily', 'monthly_days', 'dates', 'parity')
) NOT VALID;

ALTER TABLE tasks
ADD CONSTRAINT tasks_recurrence_parity_chk
CHECK (
    recurrence_parity IS NULL
    OR recurrence_parity IN ('odd', 'even')
) NOT VALID;

ALTER TABLE tasks
ADD CONSTRAINT tasks_recurrence_start_date_required_chk
CHECK (
    recurrence_type IS NULL
    OR recurrence_start_date IS NOT NULL
) NOT VALID;

ALTER TABLE tasks
ADD CONSTRAINT tasks_recurrence_daily_chk
CHECK (
    recurrence_type IS DISTINCT FROM 'daily'
    OR recurrence_every_n_days IS NOT NULL
) NOT VALID;

ALTER TABLE tasks
ADD CONSTRAINT tasks_recurrence_monthly_days_chk
CHECK (
    recurrence_type IS DISTINCT FROM 'monthly_days'
    OR recurrence_month_days IS NOT NULL
) NOT VALID;

ALTER TABLE tasks
ADD CONSTRAINT tasks_recurrence_dates_chk
CHECK (
    recurrence_type IS DISTINCT FROM 'dates'
    OR recurrence_dates IS NOT NULL
) NOT VALID;

ALTER TABLE tasks
ADD CONSTRAINT tasks_recurrence_parity_required_chk
CHECK (
    recurrence_type IS DISTINCT FROM 'parity'
    OR recurrence_parity IS NOT NULL
) NOT VALID;