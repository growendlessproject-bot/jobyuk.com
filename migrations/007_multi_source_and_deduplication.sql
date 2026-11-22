-- Migration: 007_multi_source_and_deduplication.sql
-- Enables multi-source job ingestion (Reed, FreeHire, Direct ATS) and high-speed deduplication.

-- 1. Make reed_job_id nullable for non-Reed sources (e.g., FreeHire)
ALTER TABLE jobs ALTER COLUMN reed_job_id DROP NOT NULL;

-- 2. Add source tracking column to distinguish job origin
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS source VARCHAR(50) DEFAULT 'reed';

-- 3. Add external_slug column to store upstream unique identifier (e.g. FreeHire slug)
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS external_slug TEXT;

-- 4. Create performance indexes for deduplication lookups
CREATE INDEX IF NOT EXISTS idx_jobs_external_url_active ON jobs(external_url) WHERE external_url IS NOT NULL AND external_url != '';
CREATE INDEX IF NOT EXISTS idx_jobs_title_employer ON jobs(LOWER(title), employer_id);
CREATE INDEX IF NOT EXISTS idx_jobs_source ON jobs(source);
