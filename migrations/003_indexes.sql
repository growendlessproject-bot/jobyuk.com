-- Full-text search index (GIN)
CREATE INDEX IF NOT EXISTS idx_jobs_search ON jobs USING GIN(search_vector);

-- Job listing queries
CREATE INDEX IF NOT EXISTS idx_jobs_status_date ON jobs(status, date_posted DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_location ON jobs(location_id, status);
CREATE INDEX IF NOT EXISTS idx_jobs_salary ON jobs(yearly_min, yearly_max) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_jobs_expiry ON jobs(expiration_date) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_jobs_slug ON jobs(slug);
CREATE INDEX IF NOT EXISTS idx_jobs_reed_id ON jobs(reed_job_id);

-- Taxonomy lookups
CREATE INDEX IF NOT EXISTS idx_roles_slug ON job_roles(slug);
CREATE INDEX IF NOT EXISTS idx_industries_slug ON industries(slug);
CREATE INDEX IF NOT EXISTS idx_locations_slug ON locations(slug);
CREATE INDEX IF NOT EXISTS idx_locations_parent ON locations(parent_id);
CREATE INDEX IF NOT EXISTS idx_companies_slug ON companies(slug);

-- Analytics
CREATE INDEX IF NOT EXISTS idx_pageviews_date ON page_views(created_at);
