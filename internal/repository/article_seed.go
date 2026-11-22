package repository

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type articleSeedItem struct {
	slug           string
	title          string
	category       string
	seoTitle       string
	seoDescription string
	publishedAt    time.Time
	content        string
}

// AutoSeedArticles updates historical articles and ensures the 12 high-E-E-A-T guides exist with staggered drip-feed dates.
func AutoSeedArticles(ctx context.Context, db *pgxpool.Pool) error {
	// 1. Update legacy titles to 2026 and set historical dates
	_, _ = db.Exec(ctx, `
		UPDATE articles 
		SET title = 'How to Write a UK CV That Actually Gets Interview Callbacks (2026)',
		    seo_title = 'UK CV Guide 2026: Real Examples & Formats That Work | JobyUK',
		    seo_description = 'Stop using generic Canva templates. Clean, high-impact CV structure that gets past automated filters and impresses real UK hiring managers.',
		    published_at = '2026-10-25 09:00:00+00'
		WHERE slug = 'how-to-write-a-perfect-cv';

		UPDATE articles 
		SET title = 'UK Job Search in 2026: How to Find Real Roles & Skip Recruiter Spam',
		    seo_title = 'UK Job Search Strategy 2026: Skip Recruiters & Apply Direct | JobyUK',
		    seo_description = 'How to bypass agency middlemen, spot ghost job ads, and apply straight to company ATS portals where jobs actually get filled.',
		    published_at = '2026-10-26 09:00:00+00'
		WHERE slug = 'uk-job-search-strategy';

		UPDATE articles SET title = 'CV Keywords That Get You Noticed (Without Keyword Stuffing)', published_at = '2026-10-27 09:00:00+00' WHERE slug = 'cv-keywords-that-get-interviews';
		UPDATE articles SET title = 'Short UK Cover Letter Template That Hiring Managers Actually Read', published_at = '2026-10-28 09:00:00+00' WHERE slug = 'cover-letter-template-uk';
		UPDATE articles SET title = 'The 10 Most Common UK Interview Questions (and What Bosses Want to Hear)', published_at = '2026-10-29 09:00:00+00' WHERE slug = 'most-common-interview-questions';
		UPDATE articles SET title = 'How to Use the STAR Technique Without Sounding Like a Robot', published_at = '2026-10-30 09:00:00+00' WHERE slug = 'star-method-interview-technique';
		UPDATE articles SET title = 'Virtual Job Interview Tips: Lighting, Camera & Body Language Secrets', published_at = '2026-10-31 09:00:00+00' WHERE slug = 'virtual-interview-tips';
		UPDATE articles SET title = 'How to Network on LinkedIn UK Without Being Annoying or Awkward', published_at = '2026-11-01 06:00:00+00' WHERE slug = 'networking-tips-for-job-seekers';
		UPDATE articles SET title = 'Negotiating a UK Job Offer: How to Ask for More Pay Without Losing the Offer', published_at = '2026-11-01 07:00:00+00' WHERE slug = 'negotiating-job-offers-uk';

		UPDATE articles SET published_at = '2026-11-01 08:00:00+00' WHERE slug IN ('skilled-worker-visa-uk', 'uk-skilled-worker-visa-sponsorship-guide');
		UPDATE articles SET published_at = '2026-11-01 12:00:00+00' WHERE slug IN ('nhs-pay-bands', 'nhs-agenda-for-change-pay-bands-explained');
		UPDATE articles SET published_at = '2026-11-01 16:00:00+00' WHERE slug IN ('civil-service-interview-guide', 'civil-service-success-profiles-behaviours-guide');
		UPDATE articles SET published_at = '2026-11-02 09:00:00+00' WHERE slug IN ('right-to-work-checks', 'uk-right-to-work-checks-what-jobseekers-need');
		UPDATE articles SET published_at = '2026-11-02 12:00:00+00' WHERE slug IN ('uk-minimum-wage', 'national-minimum-wage-rates-uk-2026');
		UPDATE articles SET published_at = '2026-11-02 16:00:00+00' WHERE slug IN ('competency-interview-questions', 'competency-based-interviews-questions-answers');
		UPDATE articles SET published_at = '2026-11-03 10:00:00+00' WHERE slug IN ('uk-graduate-schemes', 'graduate-schemes-uk-application-timeline-guide');
		UPDATE articles SET published_at = '2026-11-03 14:00:00+00' WHERE slug IN ('ir35-tax-guide', 'ir35-contracting-guide-inside-vs-outside');
		UPDATE articles SET published_at = '2026-11-03 16:00:00+00' WHERE slug IN ('career-change-guide', 'career-change-at-30-40-uk-practical-guide');
		UPDATE articles SET published_at = '2026-11-04 02:00:00+00' WHERE slug IN ('how-to-write-a-cv', 'how-to-write-an-ats-friendly-cv-uk');
		UPDATE articles SET published_at = '2026-11-04 04:00:00+00' WHERE slug IN ('redundancy-rights-uk', 'uk-redundancy-rights-statutory-pay-guide');
		UPDATE articles SET published_at = '2026-11-04 05:00:00+00' WHERE slug IN ('flexible-working-rights', 'hybrid-working-rights-flexible-working-requests-uk');
	`)

	seeds := []articleSeedItem{
		{
			slug:           "skilled-worker-visa-uk",
			title:          "UK Skilled Worker Visa: How to Get Sponsored in 2026 (Real Rules & Salaries)",
			category:       "job-search",
			seoTitle:       "UK Skilled Worker Visa Guide 2026: Salaries & Licensed Sponsors | JobyUK",
			seoDescription: "What you actually need to earn to get sponsored in the UK. Honest guide to the £38,700 threshold, shortage list discounts, and finding genuine licensed employers.",
			publishedAt:    time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC),
			content: `<h2>Navigating UK Skilled Worker Sponsorship in 2026</h2>
<p>Securing a role with visa sponsorship remains one of the most competitive routes in the UK job market. Following recent Home Office immigration rule adjustments, understanding the exact financial thresholds and employer compliance requirements is essential for international applicants and recruiters alike.</p>

<h2>1. The 2026 Salary Threshold Requirements</h2>
<p>Under current Home Office regulations, the standard general salary threshold for a Skilled Worker visa is <strong>£38,700 per year</strong>, or the specific "going rate" for the Standard Occupational Classification (SOC) code, whichever is higher.</p>
<ul>
    <li><strong>General Minimum Threshold:</strong> £38,700 gross per annum (calculated on a 37.5-hour working week).</li>
    <li><strong>Going Rates:</strong> Each occupation code has a median salary benchmark set by the Office for National Statistics (ONS). If the 50th percentile for your role is £44,000, your employer must meet £44,000.</li>
    <li><strong>Health and Care Worker Visa Concession:</strong> Qualified healthcare professionals (doctors, registered nurses, allied health practitioners) remain exempt from the £38,700 baseline, continuing under lower nationally agreed pay spines such as the NHS Agenda for Change.</li>
</ul>

<h2>2. The Immigration Salary List (ISL)</h2>
<p>The former Shortage Occupation List (SOL) was replaced by the streamlined <strong>Immigration Salary List (ISL)</strong>. Occupations on this list benefit from a 20% discount against the general threshold, lowering the minimum requirement to £30,960 (provided the wage also meets the role's individual going rate).</p>
<p>Typical roles represented on the ISL include civil engineers, chemical scientists, secondary education mathematics and physics teachers, and veterinary professionals.</p>

<h2>3. Identifying Genuine Licensed Sponsors</h2>
<p>Only UK businesses holding a valid <strong>Worker Sponsor Licence (A-rated)</strong> from UK Visas and Immigration (UKVI) can issue a Certificate of Sponsorship (CoS). When evaluating vacancies:</p>
<ol>
    <li><strong>Cross-reference the Register:</strong> Check the official public register of licensed sponsors published on GOV.UK before applying.</li>
    <li><strong>Clarify CoS Availability Early:</strong> Ask whether the hiring team has an unallocated "Defined CoS" (for overseas applicants) or "Undefined CoS" (for candidates switching visas from within the UK, such as Graduate Visa holders).</li>
    <li><strong>Avoid Unlicensed Intermediaries:</strong> Never pay recruitment agencies fees for job offers or visa documentation; UK law prohibits charging candidates for recruitment sponsorship.</li>
</ol>

<h2>4. Actionable Application Advice for Candidates</h2>
<p>When applying for roles requiring sponsorship, highlight your immediate eligibility clearly on your CV. If you hold a Graduate Visa (allowing two years of unsponsored work), explicitly state your current visa status and expiry date in your personal profile. Employers value candidates who can begin immediately while sponsorship paperwork is processed.</p>`,
		},
		{
			slug:           "nhs-pay-bands",
			title:          "NHS Pay Bands Explained: What You Actually Earn from Band 2 to Band 9",
			category:       "salary",
			seoTitle:       "NHS Pay Bands 2026: What You Actually Earn & London Weighting | JobyUK",
			seoDescription: "Curious what NHS staff actually take home? Clear breakdown of Band 2 to Band 9 pay rates, London weighting bonuses, and how annual pay steps work.",
			publishedAt:    time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC),
			content: `<h2>How NHS Salaries Work (Without the Bureaucracy)</h2>
<p>The NHS employs over 1.4 million people across the UK. Nearly all non-medical staff—nurses, therapists, IT specialists, healthcare assistants, and managers—are paid under a transparent national pay scale called <strong>Agenda for Change (AfC)</strong>. Here is exactly what each band pays and how your earnings increase over time.</p>

<h2>1. Overview of NHS Pay Bands (Band 2 to Band 9)</h2>
<p>Each band reflects specific levels of responsibility, qualifications, and patient care:</p>
<ul>
    <li><strong>Band 2 (£23k+):</strong> Healthcare support workers, ward clerks, domestic services, and clinic receptionists.</li>
    <li><strong>Band 3 &amp; Band 4 (£25k to £29k):</strong> Senior healthcare assistants, emergency support, phlebotomists, and technician specialists.</li>
    <li><strong>Band 5 (£29.9k to £36.4k):</strong> Starting salary tier for newly qualified registered nurses, physiotherapists, radiographers, and junior dietitians.</li>
    <li><strong>Band 6 (£37k to £44.5k):</strong> Senior staff nurses, deputy ward managers, specialist allied health professionals, and biomedical scientists.</li>
    <li><strong>Band 7 (£46k to £53k):</strong> Ward managers, specialist nurse practitioners, and clinical team coordinators.</li>
    <li><strong>Band 8a to 8d (£54k to £95k):</strong> Matrons, consultant practitioners, department leads, and clinical governance heads.</li>
    <li><strong>Band 9 (£100k+):</strong> Directors of nursing, chief finance officers, and trust executive directors.</li>
</ul>

<h2>2. High Cost Area Supplements (HCAS / London Weighting)</h2>
<p>Working in or around London? You receive an extra allowance on top of your basic NHS salary to help cover higher rent and travel costs:</p>
<table style="width: 100%; border-collapse: collapse; margin: 16px 0;">
    <thead>
        <tr style="background: #f1f5f9; text-align: left;">
            <th style="padding: 10px; border: 1px solid #cbd5e1;">Region</th>
            <th style="padding: 10px; border: 1px solid #cbd5e1;">Extra Allowance (% of Basic)</th>
            <th style="padding: 10px; border: 1px solid #cbd5e1;">Min / Max Top-Up</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td style="padding: 10px; border: 1px solid #cbd5e1;"><strong>Inner London</strong></td>
            <td style="padding: 10px; border: 1px solid #cbd5e1;">20% extra</td>
            <td style="padding: 10px; border: 1px solid #cbd5e1;">Min £5,132 – Max £8,010</td>
        </tr>
        <tr>
            <td style="padding: 10px; border: 1px solid #cbd5e1;"><strong>Outer London</strong></td>
            <td style="padding: 10px; border: 1px solid #cbd5e1;">15% extra</td>
            <td style="padding: 10px; border: 1px solid #cbd5e1;">Min £4,313 – Max £5,436</td>
        </tr>
        <tr>
            <td style="padding: 10px; border: 1px solid #cbd5e1;"><strong>Fringe Zone (Surrey, Essex, etc.)</strong></td>
            <td style="padding: 10px; border: 1px solid #cbd5e1;">5% extra</td>
            <td style="padding: 10px; border: 1px solid #cbd5e1;">Min £1,192 – Max £2,011</td>
        </tr>
    </tbody>
</table>

<h2>3. Pay Step Points and Progression</h2>
<p>You don't need to negotiate individual raises. Each band has defined pay steps. After completing a set period of service (usually 2 to 5 years) and a satisfactory performance review, you automatically move up to the next step point in your pay band.</p>

<h2>4. NHS Pension &amp; Annual Leave Perks</h2>
<p>Beyond basic salary, NHS contracts come with 27 days of annual leave (rising to 29 days after 5 years, and 33 days after 10 years), plus 8 bank holidays. The NHS Pension Scheme also remains one of the UK's most generous defined-benefit retirement schemes.</p>`,
		},
		{
			slug:           "civil-service-interview-guide",
			title:          "How to Pass Civil Service Interviews: Writing Top-Scoring 250-Word Statements",
			category:       "interviews",
			seoTitle:       "Civil Service Success Profiles 2026: Behaviours & STAR Answers | JobyUK",
			seoDescription: "Crack the Civil Service scoring system. Exactly what sift panels look for in your 250-word behaviour answers, with real STAR examples that score 6s and 7s.",
			publishedAt:    time.Date(2026, 11, 1, 16, 0, 0, 0, time.UTC),
			content: `<h2>Cracking the Civil Service Recruitment System</h2>
<p>Applying for a job in the UK Civil Service (HMRC, DWP, Home Office, MoJ, Cabinet Office) is completely different from applying in the private sector. Sift panels don't pick candidates on gut feeling; they score every application against published scoring criteria called <strong>Success Profiles</strong>.</p>

<h2>1. The 5 Elements of Success Profiles</h2>
<p>Depending on the role grade, your assessment blends up to five elements:</p>
<ol>
    <li><strong>Behaviours:</strong> Real-life examples showing how you handle challenges and deliver results.</li>
    <li><strong>Strengths:</strong> What you naturally do well and enjoy doing.</li>
    <li><strong>Ability:</strong> Online psychometric tests (numerical, verbal, judgment).</li>
    <li><strong>Experience:</strong> Your relevant career background and achievements.</li>
    <li><strong>Technical:</strong> Any specialized job-specific qualifications.</li>
</ol>

<h2>2. The 9 Core Behaviours Explained Simply</h2>
<p>Every Civil Service job advert asks for 3 or 4 behaviours from this list:</p>
<ul>
    <li><strong>Seeing the Big Picture:</strong> Understanding how your daily work fits into wider government priorities.</li>
    <li><strong>Changing and Improving:</strong> Finding smarter ways of working and simplifying clunky processes.</li>
    <li><strong>Making Effective Decisions:</strong> Using real facts and data rather than guesswork to solve tricky problems.</li>
    <li><strong>Leadership:</strong> Taking ownership, encouraging team members, and being approachable.</li>
    <li><strong>Communicating and Influencing:</strong> Explaining complex concepts clearly and convincing stakeholders.</li>
    <li><strong>Working Together:</strong> Treating colleagues with respect and building genuine team trust.</li>
    <li><strong>Developing Self and Others:</strong> Constantly sharpening skills and coaching junior teammates.</li>
    <li><strong>Managing a Quality Service:</strong> Delivering reliable, high-quality work on time.</li>
    <li><strong>Delivering at Pace:</strong> Hitting deadlines and staying focused when priorities shift suddenly.</li>
</ul>

<h2>3. The Winning Formula for 250-Word Statements</h2>
<p>Civil Service sift panels grade statements on a <strong>1 to 7 scale</strong> (4 is the pass mark; 6 and 7 guarantee interview invites). Use this strict word count split:</p>
<ul>
    <li><strong>Situation &amp; Task (approx. 40 words):</strong> Briefly set the scene and the challenge. Keep it concise.</li>
    <li><strong>Action (approx. 160 words - where the points are):</strong> Focus entirely on what <em>you</em> did, not what 'we' did. Say 'I analyzed', 'I negotiated', 'I restructured'. Detail your choices and how you handled pushback.</li>
    <li><strong>Result (approx. 50 words):</strong> Finish with measurable impact. Did you reduce costs by 22%? Cut turnaround by 3 days? Boost satisfaction to 96%? Specific metrics win interviews.</li>
</ul>`,
		},
		{
			slug:           "right-to-work-checks",
			title:          "UK Right to Work Checks: How to Get Your Share Code Without Offer Delays",
			category:       "job-search",
			seoTitle:       "UK Right to Work Checks 2026: Digital Share Codes & eVisas | JobyUK",
			seoDescription: "Never let paperwork slow down your start date. How to get your 9-digit share code online in 2 minutes, the end of physical BRP cards, and what bosses must check.",
			publishedAt:    time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC),
			content: `<h2>How UK Right to Work Checks Work in 2026</h2>
<p>Every UK employer has a strict legal obligation to verify that new hires have permission to work in the UK before day one. Knowing what paperwork is needed keeps your onboarding smooth and delays away.</p>

<h2>1. The End of Physical BRP Cards: Digital eVisas</h2>
<p>Physical Biometric Residence Permits (BRPs) have been replaced by the Home Office's digital <strong>eVisa system</strong>. Non-British and non-Irish nationals now prove their status online through their UKVI digital account.</p>

<h2>2. How to Generate Your 9-Character Share Code in 2 Minutes</h2>
<p>You can generate your code quickly on your phone:</p>
<ol>
    <li>Go to the official GOV.UK service: <em>"Prove your right to work to an employer"</em>.</li>
    <li>Log in using your passport or BRP identity number and your birth date.</li>
    <li>Verify with the SMS/email code sent to your registered contact.</li>
    <li>Copy your <strong>9-character code</strong> (e.g. <code>W12 345 678</code>).</li>
</ol>
<p>The code is valid for <strong>90 days</strong> and can be shared with multiple employers safely without sharing your private immigration history.</p>

<h2>3. What British &amp; Irish Citizens Need</h2>
<p>British and Irish passport holders don't need a share code. Employers simply verify your status using a certified digital identity app (IDSP) or by inspecting your original passport.</p>`,
		},
		{
			slug:           "uk-minimum-wage",
			title:          "UK Minimum Wage Rates: What Your Boss Legally Must Pay You",
			category:       "salary",
			seoTitle:       "UK National Minimum Wage 2026: Legal Rates & Wage Traps | JobyUK",
			seoDescription: "Are you being underpaid? Official UK hourly wage rates by age, plus 4 illegal wage deductions employers try to sneak past workers.",
			publishedAt:    time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC),
			content: `<h2>UK Statutory Wage Protections: Your Legal Rights</h2>
<p>The National Minimum Wage and National Living Wage are legally binding minimum hourly rates. By law, almost all UK workers—full-time, part-time, temporary agency, and zero-hours contract staff—must receive at least these statutory amounts.</p>

<h2>1. Current UK Minimum Wage Rates by Age</h2>
<ul>
    <li><strong>National Living Wage (Aged 21 and Over):</strong> The legal baseline rate for all adult workers across England, Scotland, Wales, and Northern Ireland.</li>
    <li><strong>18 to 20 Year Olds:</strong> Statutory youth rate for younger workers.</li>
    <li><strong>Under 18s:</strong> Baseline rate for school leavers under 18.</li>
    <li><strong>Apprentices:</strong> Applicable to apprentices under 19 or in their first year. Once an apprentice turns 19 and enters their second year, they must receive the full age-appropriate minimum wage.</li>
</ul>

<h2>2. Sneaky Wage Traps: 4 Illegal Deductions to Watch For</h2>
<p>HMRC actively fines employers who illegally underpay workers. The most common violations include:</p>
<ol>
    <li><strong>Uniform Deductions:</strong> If you must buy specific shoes or uniforms, that cost cannot drop your effective hourly pay below the minimum wage.</li>
    <li><strong>Unpaid Mandatory Training:</strong> If you must attend an induction or complete online courses, that time counts as working hours and must be paid.</li>
    <li><strong>Unpaid 'Trial Shifts':</strong> While a short 30-minute skills check is normal, working full-day shifts without pay is illegal.</li>
    <li><strong>Setup &amp; Cashing Up Time:</strong> Arriving 15 minutes before opening to prep or staying after closing to clean counts as paid working time.</li>
</ol>`,
		},
		{
			slug:           "competency-interview-questions",
			title:          "12 Toughest UK Competency Interview Questions (and Winning STAR Answers)",
			category:       "interviews",
			seoTitle:       "Competency Interview Questions UK 2026: Model STAR Answers | JobyUK",
			seoDescription: "How to answer tricky 'Tell me about a time...' questions without freezing up. Word-for-word STAR examples that impress UK hiring panels.",
			publishedAt:    time.Date(2026, 11, 2, 16, 0, 0, 0, time.UTC),
			content: `<h2>Why UK Employers Rely on Competency-Based Interviews</h2>
<p>Competency-based interviews (also known as behavioral or structured interviews) operate on the core recruitment premise: <em>past performance is the most reliable predictor of future success</em>. Major UK employers across banking, retail, healthcare, civil service, and technology rely on these structured questions to eliminate interviewer bias.</p>

<h2>1. Core Competency Categories</h2>
<p>Assessors focus on four overarching domains:</p>
<ul>
    <li><strong>Individual Competencies:</strong> Personal resilience, analytical problem solving, decisiveness, and adaptability.</li>
    <li><strong>Managerial Competencies:</strong> Strategic leadership, resource delegation, performance management, and team motivation.</li>
    <li><strong>Interpersonal Competencies:</strong> Stakeholder diplomacy, conflict resolution, active listening, and relationship management.</li>
    <li><strong>Motivational Competencies:</strong> Drive for results, continuous learning, customer focus, and operational diligence.</li>
</ul>

<h2>2. 12 Frequent UK Competency Questions</h2>
<ol>
    <li><em>"Tell me about a time you had to deliver a critical project under tight, conflicting deadlines."</em> (Delivering at Pace)</li>
    <li><em>"Describe a situation where you had to persuade a resistant stakeholder to adopt a major change."</em> (Influencing &amp; Communication)</li>
    <li><em>"Give an example of a difficult team conflict you managed and how you resolved it."</em> (Working Together)</li>
    <li><em>"Can you share an instance where you identified a process inefficiency and overhauled it?"</em> (Continuous Improvement)</li>
    <li><em>"Describe a time when a project didn't go according to plan. How did you react?"</em> (Resilience &amp; Problem Solving)</li>
    <li><em>"Tell me about a high-stakes decision you made with limited or incomplete data."</em> (Decision Making)</li>
    <li><em>"How have you handled a situation where a client or customer was deeply dissatisfied?"</em> (Customer Excellence)</li>
    <li><em>"Describe a time you coached a struggling colleague to meet their performance targets."</em> (Developing Talent)</li>
    <li><em>"Give an example of balancing compliance and regulatory requirements with business speed."</em> (Governance &amp; Risk)</li>
    <li><em>"Tell me about a time you had to adapt quickly to unexpected organizational restructuring."</em> (Change Management)</li>
    <li><em>"Describe a complex data analysis you conducted and how you communicated findings to non-technical leaders."</em> (Analytical Communication)</li>
    <li><em>"Give an example of how you prioritized ethical standards over immediate short-term commercial gains."</em> (Integrity &amp; Values)</li>
</ol>

<h2>3. Model Answer Breakdown: The STAR Framework in Action</h2>
<p>Consider the question: <em>"Describe a situation where you managed a major stakeholder disagreement."</em></p>
<blockquote>
    <p><strong>Situation:</strong> In my previous role as an IT Operations Lead at a Midlands manufacturing firm, our rollout of a new ERP platform encountered fierce resistance from the regional warehouse management team who were concerned about operational downtime.</p>
    <p><strong>Task:</strong> My responsibility was to align warehouse operations with the project timeline without jeopardizing daily dispatch quotas.</p>
    <p><strong>Action:</strong> Rather than issuing top-down directives, I scheduled an on-site consultation at the distribution centre. I shadowed their shift supervisors for two days to pinpoint specific workflow pinch points. Based on their feedback, I redesigned our training into modular 20-minute micro-sessions and agreed to execute data migration over a bank holiday weekend with full IT standby support.</p>
    <p><strong>Result:</strong> We launched on schedule with zero order delays, achieving a 98% user satisfaction rate. The warehouse team became enthusiastic advocates for subsequent rollout phases.</p>
</blockquote>

<h2>4. The Golden Rule: Metrics and Specifics</h2>
<p>UK interview panels look for concrete metrics. Avoid generalizations such as "the project was successful." Instead, provide hard numbers: "we reduced processing turnaround by 34%, saved £12,500 in licensing, and maintained 99.9% uptime throughout the quarter."</p>`,
		},
		{
			slug:           "uk-graduate-schemes",
			title:          "UK Graduate Schemes: Key Deadlines, Passing Tests & Acing Assessment Centres",
			category:       "job-search",
			seoTitle:       "UK Graduate Schemes 2026: Deadlines, Tests & Assessment Centres | JobyUK",
			seoDescription: "How to land a top UK graduate scheme paying £30k+. The application timeline, how to beat SHL numerical tests, and group exercise survival tips.",
			publishedAt:    time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC),
			content: `<h2>How to Actually Land a UK Graduate Scheme</h2>
<p>UK graduate schemes offer great training, structured rotations, and starting pay between £28,000 and £55,000. But with hundreds of graduates applying for every single opening, you need to know how the recruitment timeline works so you don't miss out.</p>

<h2>1. The Graduate Application Calendar</h2>
<ul>
    <li><strong>September – October:</strong> Big Four, Civil Service Fast Stream, banks, and engineering programmes open. Many recruit on a <em>rolling basis</em>—which means they review applications as they come in and close early once seats fill. Apply early.</li>
    <li><strong>November – December:</strong> Online psychometric tests (SHL, Cappfinity) and automated video interviews (HireVue).</li>
    <li><strong>January – March:</strong> Assessment centres and final interviews with directors.</li>
    <li><strong>September:</strong> Induction day and start of your training contract.</li>
</ul>

<h2>2. How to Beat Online Psychometric Tests</h2>
<p>Most candidates get knocked out by automated test filters. Here is how to pass:</p>
<ol>
    <li><strong>SHL Numerical &amp; Verbal:</strong> Timed tests where speed matters as much as accuracy. Practice on free platforms like AssessmentDay beforehand so the question style isn't a shock.</li>
    <li><strong>Cappfinity Strengths Tests:</strong> Untimed tests that check your natural problem-solving instincts. Answer honestly without trying to second-guess what you think the firm wants to hear.</li>
</ol>

<h2>3. Surviving Assessment Centres</h2>
<p>The biggest mistake in group exercises is trying to talk the most. Assessors aren't looking for the loudest voice in the room; they are watching who brings quieter team members into the conversation, stays calm under time pressure, and keeps everyone focused on the goal.</p>`,
		},
		{
			slug:           "ir35-tax-guide",
			title:          "Inside vs Outside IR35: What UK Contractors Actually Take Home",
			category:       "salary",
			seoTitle:       "Inside vs Outside IR35 Explained: Real Take-Home Cash & Tax | JobyUK",
			seoDescription: "IR35 explained in plain English. The difference in tax between inside and outside contracts, what makes a role genuine B2B, and umbrella company red flags.",
			publishedAt:    time.Date(2026, 11, 3, 14, 0, 0, 0, time.UTC),
			content: `<h2>IR35 Explained Without the Tax Jargon</h2>
<p>If you contract in the UK, understanding <strong>IR35</strong> is the difference between keeping your hard-earned day rate or losing up to 40% of it in unexpected PAYE deductions.</p>

<h2>1. Inside IR35 vs. Outside IR35 in Plain English</h2>
<ul>
    <li><strong>Outside IR35 (Genuine B2B):</strong> HMRC treats your limited company as an independent business. You invoice your client, pay Corporation Tax, and take income through a salary/dividend mix. You keep significantly more take-home cash.</li>
    <li><strong>Inside IR35 (Deemed Employee):</strong> HMRC treats the contract as employment for tax purposes. Income tax and National Insurance are deducted before you get paid, usually via an umbrella company.</li>
</ul>

<h2>2. The 3 Things HMRC Checks</h2>
<ol>
    <li><strong>Right of Substitution:</strong> Can your company send someone else to do the work if you're sick, without the client throwing a fit? If yes, that strongly supports Outside IR35.</li>
    <li><strong>Control:</strong> Does the client dictate your exact hours and how you do the task, or are you in charge of delivering the agreed outcome?</li>
    <li><strong>Mutuality of Obligation:</strong> Is the client obliged to give you ongoing work after the project ends? An independent project contract has a defined finish line.</li>
</ol>`,
		},
		{
			slug:           "career-change-guide",
			title:          "Changing Careers at 30, 40 or Later: How to Pivot Without Starting from Zero",
			category:       "job-search",
			seoTitle:       "Career Change at 30, 40 & Beyond UK: Real Transition Roadmap | JobyUK",
			seoDescription: "Think you're too late to switch industries? How to reframe your years of experience, map transferable skills, and structure a CV that gets callbacks in a new field.",
			publishedAt:    time.Date(2026, 11, 3, 16, 0, 0, 0, time.UTC),
			content: `<h2>Pivoting Your Career: You Don't Have to Start from Scratch</h2>
<p>The old myth of staying in one job for 40 years is dead. In the UK, more than a third of professionals make a major career pivot in their 30s or 40s. The trick is proving how your existing years of problem-solving, team management, and commercial judgment directly help your new employer.</p>

<h2>1. The Transferable Skills Employers Actually Pay For</h2>
<p>You don't lose your seniority when you change industries. UK bosses pay top money for people who already know how to:</p>
<ul>
    <li>De-escalate unhappy clients and handle difficult stakeholders.</li>
    <li>Manage budgets and keep projects from burning through cash.</li>
    <li>Prioritize work under pressure when everything is due at once.</li>
    <li>Communicate clearly with both technical specialists and executive directors.</li>
</ul>

<h2>2. Rewrite Your CV to Highlight Skills Over History</h2>
<p>A standard chronological CV makes your industry shift look disjointed. Switch to a <strong>skills-focused CV format</strong>: lead with your transferable capabilities right under your personal statement, followed by a streamlined work history that highlights your achievements.</p>`,
		},
		{
			slug:           "how-to-write-a-cv",
			title:          "How to Write an ATS-Friendly CV That Real Humans Actually Read",
			category:       "cv",
			seoTitle:       "ATS Friendly CV Guide UK 2026: Formatting Rules That Work | JobyUK",
			seoDescription: "Why fancy Canva templates get rejected by recruiter software. Clean layout rules, formatting fixes, and how to match keywords naturally.",
			publishedAt:    time.Date(2026, 11, 4, 2, 0, 0, 0, time.UTC),
			content: `<h2>What UK Applicant Tracking Systems (ATS) Actually Do</h2>
<p>Over 90% of UK corporate employers and recruitment portals use automated software (Workday, Greenhouse, Taleo) to parse incoming CVs before a recruiter reads them. If your CV formatting confuses the parser, you get rejected before a human ever sees your qualifications.</p>

<h2>1. Layout Mistakes That Ruin Your Chances</h2>
<ul>
    <li><strong>Multi-Column Designs &amp; Text Boxes:</strong> Parsers read strictly left-to-right. Two-column CVs scramble your job titles, dates, and bullets into garbled nonsense.</li>
    <li><strong>Visual Skill Rating Bars:</strong> Writing 'Python: 4 out of 5 stars' is completely invisible to text parsers. Use clear, written bullet points.</li>
    <li><strong>Key Details in Headers or Footers:</strong> Many ATS platforms ignore document headers and footers entirely. Keep your name, email, phone number, and location in the main page body.</li>
</ul>

<h2>2. Best File Format: Clean Word (.docx) or Searchable PDF</h2>
<p>Stick to a single-column document formatted in clean fonts like Calibri, Arial, or Helvetica. If an employer's portal asks for Word format, always upload a .docx file.</p>`,
		},
		{
			slug:           "redundancy-rights-uk",
			title:          "UK Redundancy Rights: Calculating Your Payout & Protecting Your Notice",
			category:       "job-search",
			seoTitle:       "UK Redundancy Rights 2026: Statutory Pay Calculator & Rules | JobyUK",
			seoDescription: "Facing redundancy? Exactly how statutory pay is calculated, the £30k tax-free rule, and what employers legally must offer before making you redundant.",
			publishedAt:    time.Date(2026, 11, 4, 4, 0, 0, 0, time.UTC),
			content: `<h2>Your Legal Rights When Facing Redundancy in the UK</h2>
<p>Going through redundancy is stressful, but UK employment law provides strong legal protections around consultation periods, notice pay, and statutory redundancy payouts.</p>

<h2>1. Who Qualifies for Statutory Redundancy Pay?</h2>
<p>To qualify for statutory redundancy compensation by law, you must have been an employed worker with <strong>at least 2 years of continuous service</strong> at the company.</p>

<h2>2. How Much You Are Legally Entitled To</h2>
<p>The statutory payout calculation depends on your age and years of completed service (up to 20 years maximum):</p>
<ul>
    <li><strong>Under 22:</strong> 0.5 week's pay for each full year of service.</li>
    <li><strong>Aged 22 to 40:</strong> 1 full week's pay for each full year of service.</li>
    <li><strong>Aged 41 and Older:</strong> 1.5 weeks' pay for each full year of service.</li>
</ul>
<p><em>The £30,000 Tax-Free Rule:</em> The first £30,000 of genuine redundancy pay is completely free from UK income tax and National Insurance deductions.</p>`,
		},
		{
			slug:           "flexible-working-rights",
			title:          "Asking for Flexible or Hybrid Working: Your Legal Rights from Day One",
			category:       "job-search",
			seoTitle:       "Flexible & Hybrid Working Rights UK 2026: Day-One Law | JobyUK",
			seoDescription: "Want to work from home or do compressed hours? How to use the Day-One flexible working law to submit a request your boss can't easily dismiss.",
			publishedAt:    time.Date(2026, 11, 4, 5, 0, 0, 0, time.UTC),
			content: `<h2>How UK Flexible Working Rights Work Today</h2>
<p>Under UK employment legislation, all employees have the legal right to submit a statutory flexible working request—including hybrid working, part-time hours, or flexitime—from their <strong>very first day on the job</strong>.</p>

<h2>1. Your Core Rights Under the Law</h2>
<ul>
    <li><strong>Day-One Right:</strong> You don't have to wait 6 months; you can request flexible hours from day one.</li>
    <li><strong>Two Requests Per Year:</strong> You can submit up to two formal requests within any 12-month period.</li>
    <li><strong>Employers Must Consult:</strong> Your boss cannot simply reply 'no' by email; they must discuss the request with you before making any decision.</li>
    <li><strong>2-Month Decision Window:</strong> The employer must give a final decision within 2 months.</li>
</ul>

<h2>2. How to Submit a Request That Gets Approved</h2>
<p>Submit your request in writing and propose a <strong>3-month trial period</strong>. Suggesting a trial reassures managers who might worry about shift coverage or customer communication, and makes it much harder for them to refuse.</p>`,
		},
	}

	for _, a := range seeds {
		query := `
			INSERT INTO articles (slug, title, category, seo_title, seo_description, published, published_at, content, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, true, $6, $7, NOW(), NOW())
			ON CONFLICT (slug) DO UPDATE 
			SET title = EXCLUDED.title,
			    category = EXCLUDED.category,
			    seo_title = EXCLUDED.seo_title,
			    seo_description = EXCLUDED.seo_description,
			    published = true,
			    published_at = EXCLUDED.published_at,
			    content = EXCLUDED.content,
			    updated_at = NOW();
		`
		_, err := db.Exec(ctx, query, a.slug, a.title, a.category, a.seoTitle, a.seoDescription, a.publishedAt, a.content)
		if err != nil {
			slog.Warn("AutoSeedArticles item warning", "slug", a.slug, "error", err)
		}
	}

	slog.Info("AutoSeedArticles completed successfully")
	return nil
}
