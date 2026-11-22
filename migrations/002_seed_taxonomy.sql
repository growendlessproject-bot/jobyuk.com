-- Seed Industries
INSERT INTO industries (slug, name, seo_title, seo_description) VALUES
('technology', 'Technology', 'Technology Jobs | JobyUK', 'Browse the latest technology and IT jobs in the UK.'),
('finance', 'Finance & Accounting', 'Finance Jobs | JobyUK', 'Browse finance, accounting, and banking jobs in the UK.'),
('healthcare', 'Healthcare & Medical', 'Healthcare Jobs | JobyUK', 'Find nursing, medical, and healthcare jobs across the UK.'),
('education', 'Education & Teaching', 'Education Jobs | JobyUK', 'Find teaching, tutoring, and education jobs in the UK.'),
('engineering', 'Engineering', 'Engineering Jobs | JobyUK', 'Explore mechanical, electrical, and civil engineering jobs.'),
('construction', 'Construction', 'Construction Jobs | JobyUK', 'Find construction, building, and trade jobs in the UK.'),
('retail', 'Retail & Consumer Goods', 'Retail Jobs | JobyUK', 'Explore retail, sales assistant, and store manager jobs.'),
('hospitality', 'Hospitality & Catering', 'Hospitality Jobs | JobyUK', 'Browse restaurant, hotel, and catering jobs in the UK.'),
('logistics', 'Logistics & Transport', 'Logistics Jobs | JobyUK', 'Explore logistics, warehouse, driving, and supply chain jobs.')
ON CONFLICT (slug) DO NOTHING;

-- Seed Job Roles
INSERT INTO job_roles (slug, name, industry_id, seo_title, seo_description) VALUES
('software-developer', 'Software Developer', (SELECT id FROM industries WHERE slug = 'technology'), 'Software Developer Jobs | JobyUK', 'Find Software Developer roles in the UK.'),
('software-engineer', 'Software Engineer', (SELECT id FROM industries WHERE slug = 'technology'), 'Software Engineer Jobs | JobyUK', 'Find Software Engineer roles in the UK.'),
('web-developer', 'Web Developer', (SELECT id FROM industries WHERE slug = 'technology'), 'Web Developer Jobs | JobyUK', 'Find Web Developer roles in the UK.'),
('data-analyst', 'Data Analyst', (SELECT id FROM industries WHERE slug = 'technology'), 'Data Analyst Jobs | JobyUK', 'Find Data Analyst roles in the UK.'),
('project-manager', 'Project Manager', (SELECT id FROM industries WHERE slug = 'technology'), 'Project Manager Jobs | JobyUK', 'Find Project Manager roles in the UK.'),
('accountant', 'Accountant', (SELECT id FROM industries WHERE slug = 'finance'), 'Accountant Jobs | JobyUK', 'Find Accountant roles in the UK.'),
('nurse', 'Nurse', (SELECT id FROM industries WHERE slug = 'healthcare'), 'Nursing Jobs | JobyUK', 'Find Nursing roles in the UK.'),
('teacher', 'Teacher', (SELECT id FROM industries WHERE slug = 'education'), 'Teaching Jobs | JobyUK', 'Find Teaching roles in the UK.')
ON CONFLICT (slug) DO NOTHING;

-- Seed Location Hierarchy
-- 1. Countries
INSERT INTO locations (slug, name, level) VALUES
('england', 'England', 'country'),
('scotland', 'Scotland', 'country'),
('wales', 'Wales', 'country'),
('northern-ireland', 'Northern Ireland', 'country')
ON CONFLICT (slug) DO NOTHING;

-- 2. Major Cities
INSERT INTO locations (slug, name, parent_id, level) VALUES
('london', 'London', (SELECT id FROM locations WHERE slug = 'england'), 'city'),
('manchester', 'Manchester', (SELECT id FROM locations WHERE slug = 'england'), 'city'),
('birmingham', 'Birmingham', (SELECT id FROM locations WHERE slug = 'england'), 'city'),
('leeds', 'Leeds', (SELECT id FROM locations WHERE slug = 'england'), 'city'),
('liverpool', 'Liverpool', (SELECT id FROM locations WHERE slug = 'england'), 'city'),
('bristol', 'Bristol', (SELECT id FROM locations WHERE slug = 'england'), 'city'),
('glasgow', 'Glasgow', (SELECT id FROM locations WHERE slug = 'scotland'), 'city'),
('edinburgh', 'Edinburgh', (SELECT id FROM locations WHERE slug = 'scotland'), 'city'),
('cardiff', 'Cardiff', (SELECT id FROM locations WHERE slug = 'wales'), 'city'),
('belfast', 'Belfast', (SELECT id FROM locations WHERE slug = 'northern-ireland'), 'city')
ON CONFLICT (slug) DO NOTHING;
