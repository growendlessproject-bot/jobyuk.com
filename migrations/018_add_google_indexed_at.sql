-- Track Google Indexing API submissions for real-time fast-track indexing
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS google_indexed_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_jobs_google_indexed_at ON jobs (google_indexed_at) WHERE status = 'active';

