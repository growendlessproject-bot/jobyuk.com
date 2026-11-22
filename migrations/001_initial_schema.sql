-- Core Jobs Table
CREATE TABLE IF NOT EXISTS companies (
    id              BIGSERIAL PRIMARY KEY,
    reed_employer_id BIGINT UNIQUE,
    slug            TEXT UNIQUE NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT,
    logo_url        TEXT,
    website_url     TEXT,
    job_count       INT DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS locations (
    id              SERIAL PRIMARY KEY,
    slug            TEXT UNIQUE NOT NULL,
    name            TEXT NOT NULL,
    parent_id       INT REFERENCES locations(id),
    level           TEXT NOT NULL,  -- country, region, city
    latitude        DECIMAL(9,6),
    longitude       DECIMAL(9,6),
    job_count       INT DEFAULT 0
);

CREATE TABLE IF NOT EXISTS industries (
    id              SERIAL PRIMARY KEY,
    slug            TEXT UNIQUE NOT NULL,
    name            TEXT NOT NULL,
    job_count       INT DEFAULT 0,
    seo_title       TEXT,
    seo_description TEXT
);

CREATE TABLE IF NOT EXISTS job_roles (
    id              SERIAL PRIMARY KEY,
    slug            TEXT UNIQUE NOT NULL,
    name            TEXT NOT NULL,
    parent_id       INT REFERENCES job_roles(id),
    industry_id     INT REFERENCES industries(id),
    job_count       INT DEFAULT 0,
    seo_title       TEXT,
    seo_description TEXT
);

CREATE TABLE IF NOT EXISTS jobs (
    id              BIGSERIAL PRIMARY KEY,
    reed_job_id     BIGINT UNIQUE NOT NULL,
    slug            TEXT UNIQUE NOT NULL,
    title           TEXT NOT NULL,
    description     TEXT NOT NULL,
    employer_id     BIGINT REFERENCES companies(id),
    location_id     INT REFERENCES locations(id),
    location_name   TEXT,
    min_salary      DECIMAL(12,2),
    max_salary      DECIMAL(12,2),
    yearly_min      DECIMAL(12,2),
    yearly_max      DECIMAL(12,2),
    currency        TEXT DEFAULT 'GBP',
    salary_type     TEXT,          -- per_hour, per_day, per_annum
    contract_type   TEXT,          -- permanent, contract, temporary
    job_type        TEXT,          -- full_time, part_time
    is_graduate     BOOLEAN DEFAULT FALSE,
    is_remote       BOOLEAN DEFAULT FALSE,
    external_url    TEXT,
    reed_url        TEXT,
    date_posted     TIMESTAMPTZ NOT NULL,
    expiration_date TIMESTAMPTZ,
    status          TEXT DEFAULT 'active', -- active, expired, removed
    search_vector   TSVECTOR,      -- Full-text search
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Job ↔ Role many-to-many
CREATE TABLE IF NOT EXISTS job_roles_mapping (
    job_id          BIGINT REFERENCES jobs(id) ON DELETE CASCADE,
    role_id         INT REFERENCES job_roles(id),
    PRIMARY KEY (job_id, role_id)
);

-- Job ↔ Industry many-to-many
CREATE TABLE IF NOT EXISTS job_industries_mapping (
    job_id          BIGINT REFERENCES jobs(id) ON DELETE CASCADE,
    industry_id     INT REFERENCES industries(id),
    PRIMARY KEY (job_id, industry_id)
);

-- Career Advice Articles (CMS)
CREATE TABLE IF NOT EXISTS articles (
    id              SERIAL PRIMARY KEY,
    slug            TEXT UNIQUE NOT NULL,
    title           TEXT NOT NULL,
    content         TEXT NOT NULL,
    category        TEXT NOT NULL,  -- cv, interviews, job-search, etc.
    seo_title       TEXT,
    seo_description TEXT,
    published       BOOLEAN DEFAULT FALSE,
    published_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Salary Guides
CREATE TABLE IF NOT EXISTS salary_guides (
    id              SERIAL PRIMARY KEY,
    role_slug       TEXT NOT NULL,
    role_name       TEXT NOT NULL,
    location_slug   TEXT,
    location_name   TEXT,
    min_salary      DECIMAL(12,2),
    max_salary      DECIMAL(12,2),
    avg_salary      DECIMAL(12,2),
    sample_size     INT,
    data_source     TEXT,
    seo_title       TEXT,
    seo_description TEXT,
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Admin Users
CREATE TABLE IF NOT EXISTS admin_users (
    id              SERIAL PRIMARY KEY,
    username        TEXT UNIQUE NOT NULL,
    password_hash   TEXT NOT NULL,  -- Argon2id
    role            TEXT DEFAULT 'admin',
    last_login      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- SEO Redirects (for expired job management)
CREATE TABLE IF NOT EXISTS redirects (
    id              SERIAL PRIMARY KEY,
    from_path       TEXT UNIQUE NOT NULL,
    to_path         TEXT NOT NULL,
    status_code     INT DEFAULT 301,  -- 301 or 410
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Analytics Events
CREATE TABLE IF NOT EXISTS page_views (
    id              BIGSERIAL PRIMARY KEY,
    path            TEXT NOT NULL,
    referrer        TEXT,
    user_agent      TEXT,
    ip_hash         TEXT,           -- Hashed IP for privacy
    country         TEXT,
    device_type     TEXT,           -- mobile, tablet, desktop
    created_at      TIMESTAMPTZ DEFAULT NOW()
);
