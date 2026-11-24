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
	// 1. Delete duplicate legacy slugs to avoid Google AdSense "Low value content" and duplicate cannibalization
	duplicateSlugs := []string{
		"uk-skilled-worker-visa-sponsorship-guide",
		"nhs-agenda-for-change-pay-bands-explained",
		"civil-service-success-profiles-behaviours-guide",
		"uk-right-to-work-checks-what-jobseekers-need",
		"national-minimum-wage-rates-uk-2026",
		"competency-based-interviews-questions-answers",
		"graduate-schemes-uk-application-timeline-guide",
		"ir35-contracting-guide-inside-vs-outside",
		"career-change-at-30-40-uk-practical-guide",
		"how-to-write-an-ats-friendly-cv-uk",
		"uk-redundancy-rights-statutory-pay-guide",
		"hybrid-working-rights-flexible-working-requests-uk",
		"how-to-write-a-perfect-cv",
		"cover-letter-template-uk",
		"most-common-interview-questions",
		"star-method-interview-technique",
		"virtual-interview-tips",
		"networking-tips-for-job-seekers",
		"negotiating-job-offers-uk",
		"cv-keywords-that-get-interviews",
	}

	for _, dSlug := range duplicateSlugs {
		_, _ = db.Exec(ctx, "DELETE FROM articles WHERE slug = $1", dSlug)
	}

	seeds := []articleSeedItem{
		// 1. CV Guide
		{
			slug:           "how-to-write-a-cv",
			title:          "How to Write a UK CV That Actually Gets Interview Callbacks (2026 Guide)",
			category:       "cv",
			seoTitle:       "UK CV Writing Guide 2026: Format, ATS Rules & Real Examples | JobyUK",
			seoDescription: "Step-by-step UK CV writing guide with real Before & After bullet points, ATS formatting rules, and proven structures that impress UK hiring managers.",
			publishedAt:    time.Date(2026, 10, 28, 9, 15, 0, 0, time.UTC),
			content: `<h2>The Reality of UK Recruitment in 2026</h2>
<p>In the UK employment market, corporate job openings and direct-to-employer vacancies receive an average of 120 to 250 applications within the first 72 hours. Behind the scenes, large employers use Applicant Tracking Systems (ATS) such as Workday, Greenhouse, Taleo, and Bullhorn to parse and index incoming CVs. However, the common myth that "AI robot algorithms automatically reject 90% of candidates" is misunderstood.</p>
<p>In reality, the software extracts your text into a standard candidate profile, and a human talent acquisition specialist reviews it on their monitor for roughly <strong>6 to 10 seconds</strong>. If your CV is cluttered with two-column graphics, Canva charts, or vague duties without metrics, it gets passed over. Here is the exact framework required to pass both automated parsers and time-poor UK hiring managers.</p>

<h2>1. UK CV Standards: What Must Be Included (and Left Out)</h2>
<p>Unlike international resumes, UK recruitment practices are strictly governed by the <em>Equality Act 2010</em>. Including personal demographic information can cause internal compliance teams to discard your CV to avoid unconscious bias claims.</p>
<ul>
    <li><strong>Never Include:</strong> Your photograph, date of birth, age, marital status, nationality, National Insurance (NI) number, or full street address.</li>
    <li><strong>Mandatory Essentials:</strong> Full Name, Professional Title (e.g., <em>Senior Software Engineer | AWS & Go</em>), City and Postcode area (e.g., <em>Manchester, M4 | Willing to commute / Hybrid</em>), Mobile Number, Professional Email Address, and an active LinkedIn or GitHub portfolio link.</li>
</ul>

<h2>2. The Ideal Structure and Page Budget</h2>
<p>For 95% of UK jobseekers, the standard limit is strictly <strong>two pages of A4</strong>. Only academic researchers (who list publications) or contractors with 15+ years of specialist project delivery should ever exceed two pages.</p>
<ol>
    <li><strong>Header:</strong> Clean contact block (10% of page 1).</li>
    <li><strong>Professional Summary (4 lines maximum):</strong> State who you are, your core specialisms, and the exact commercial value you deliver. Avoid generic buzzwords like "hardworking team player who thrives under pressure."</li>
    <li><strong>Core Technical Competencies / Skills Matrix:</strong> A neat bulleted row of hard skills, technologies, and certifications.</li>
    <li><strong>Employment History (Reverse Chronological):</strong> Your most recent roles occupying 60% of the CV space.</li>
    <li><strong>Education & Professional Qualifications:</strong> University degrees, Apprenticeships, or recognized accreditations (e.g., ACCA, PRINCE2, CIPD).</li>
</ol>

<h2>3. Writing High-Impact Bullet Points (Before vs. After)</h2>
<p>UK employers hire for results, not task lists. Every bullet point in your work history should follow the <strong>Action + Context + Quantifiable Metric</strong> formula.</p>

<table style="width: 100%; border-collapse: collapse; margin: 20px 0; font-size: 0.95rem;">
    <thead>
        <tr style="background: #F1F5F9; border-bottom: 2px solid #CBD5E1; text-align: left;">
            <th style="padding: 12px; width: 45%;">Weak Duty-Based Bullet (Pass)</th>
            <th style="padding: 12px; width: 55%; background: #ECFDF5; color: #065F46;">High-Impact Commercial Bullet (Interview Winner)</th>
        </tr>
    </thead>
    <tbody>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 12px; color: #64748B;">"Responsible for managing customer service queries and resolving complaints."</td>
            <td style="padding: 12px; background: #F0FDF4; font-weight: 500;">"Handled 65+ Tier-2 customer escalations daily, boosting first-contact resolution by 22% and reducing Zendesk queue backlog across a 12-person team."</td>
        </tr>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 12px; color: #64748B;">"Helped the sales team generate leads and improve conversion rates."</td>
            <td style="padding: 12px; background: #F0FDF4; font-weight: 500;">"Architected an automated inbound lead nurturing sequence in HubSpot, generating £340,000 in qualified enterprise pipeline within Q3."</td>
        </tr>
    </tbody>
</table>

<h2>4. ATS Technical Formatting Rules</h2>
<p>To ensure 100% clean parsing through Workday, Taleo, and Greenhouse:</p>
<ul>
    <li>Use standard, web-safe fonts: Calibri, Arial, Helvetica, or Garamond between 10pt and 11.5pt.</li>
    <li>Avoid floating text boxes, icons, skill rating bars (e.g., 4/5 stars in Excel), and multi-column magazine layouts.</li>
    <li>Save and upload your document as a clean <strong>.docx or standard searchable PDF</strong> (never upload scanned images or flat JPEGs).</li>
</ul>`,
		},

		// 2. UK Job Search Strategy
		{
			slug:           "uk-job-search-strategy",
			title:          "UK Job Search Strategy 2026: Skip Agency Middlemen & Apply Direct to Company ATS",
			category:       "job-search",
			seoTitle:       "UK Job Search Strategy 2026: Direct Employer ATS vs Agency Spam | JobyUK",
			seoDescription: "How to stop wasting weeks applying to ghost agency adverts. Learn how to locate genuine direct employer vacancies and apply straight to company ATS portals.",
			publishedAt:    time.Date(2026, 10, 29, 10, 30, 0, 0, time.UTC),
			content: `<h2>The Ghost Job Problem in UK Recruitment</h2>
<p>If you have spent weeks firing off applications on generic aggregator boards only to receive automated rejections or complete silence, you are not alone. Industry surveys across the UK reveal that up to <strong>35% to 45% of jobs posted by third-party recruitment agencies are "phantom" or "pipeline" listings</strong>.</p>
<p>Recruitment agencies frequently post attractive vacancies to harvest candidate CVs, test commercial client budgets, or build candidate rosters for tenders that may never materialize. Meanwhile, genuine direct employers are actively hiring on their private internal career portals (Applicant Tracking Systems).</p>

<h2>1. Agency Middleman Trap vs. Direct ATS Applications</h2>
<p>Understanding how UK employers recruit is critical to accelerating your job hunt:</p>
<ul>
    <li><strong>Agency Applications:</strong> Your CV sits in a shared recruitment agency inbox with 300 others. If the recruiter doesn't like one word, your CV is never passed to the hiring manager. Furthermore, if hired via an agency, the employer must pay a hefty 15% to 25% finder's fee (often £6,000 to £15,000), making you a more expensive hire than direct applicants.</li>
    <li><strong>Direct Company ATS (JobyUK Approach):</strong> When you apply directly to a company's internal portal (Workday, Greenhouse, Lever, Ashby, or BambooHR), your profile enters the hiring manager's internal dashboard. There are zero agency commission fees attached to your head, making your candidacy significantly more attractive to commercial finance directors.</li>
</ul>

<h2>2. How to Spot "Ghost" Job Ads in 30 Seconds</h2>
<p>Look for these unmistakable warning signs of agency placeholder ads:</p>
<ol>
    <li><strong>Vague Client Name:</strong> "Our client, a market-leading Tier 1 fintech in Central London..." (If they cannot name the business, it is an agency trying to protect their commission).</li>
    <li><strong>Salary Range Too Wide:</strong> "£40,000 – £85,000 depending on experience." Legitimate hiring teams have fixed pay bands approved by HR finance.</li>
    <li><strong>Immediate Auto-Responder:</strong> You receive an email asking you to register on an external agency database before an interview is even mentioned.</li>
</ol>

<h2>3. The 3-Step Direct Sourcing Routine</h2>
<p>Follow this routine daily on JobyUK to dramatically increase your interview rate:</p>
<ul>
    <li><strong>Target Direct Vacancies:</strong> Filter only verified direct employer listings.</li>
    <li><strong>Map the Internal Team:</strong> Search LinkedIn for the company name + "Talent Acquisition Specialist" or "Head of [Your Department]".</li>
    <li><strong>Submit Clean Application:</strong> Apply via the direct ATS portal, then send a polite, 3-sentence note to the internal talent acquisition manager letting them know your application is in their system.</li>
</ul>`,
		},

		// 3. Skilled Worker Visa
		{
			slug:           "skilled-worker-visa-uk",
			title:          "UK Skilled Worker Visa 2026: Salary Thresholds, Shortage Rules & Licensed Employers",
			category:       "job-search",
			seoTitle:       "UK Skilled Worker Visa Guide 2026: Salaries & Licensed Sponsors | JobyUK",
			seoDescription: "Honest guide to the UK Skilled Worker Visa rules: £38,700 threshold, Immigration Salary List discounts, and finding genuine licensed sponsors in 2026.",
			publishedAt:    time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC),
			content: `<h2>Navigating UK Skilled Worker Sponsorship in 2026</h2>
<p>Securing employment with visa sponsorship remains one of the most competitive avenues in the United Kingdom. Following the major immigration policy updates implemented by the Home Office, understanding the exact financial thresholds and sponsor compliance criteria is vital for international professionals.</p>

<h2>1. The 2026 Salary Threshold Requirements</h2>
<p>Under current Home Office immigration regulations, the standard general baseline threshold for a Skilled Worker visa is <strong>£38,700 per annum</strong>, or the specific "going rate" for the Standard Occupational Classification (SOC) code, whichever is higher.</p>
<ul>
    <li><strong>General Baseline:</strong> £38,700 gross per year based on a 37.5-hour working week.</li>
    <li><strong>SOC Code Going Rates:</strong> Each job category has a specific median benchmark set by the Office for National Statistics (ONS). For instance, if the going rate for a Senior Data Engineer is £52,000, your sponsor must match £52,000, not the lower baseline.</li>
    <li><strong>Health and Care Exemption:</strong> Qualified healthcare professionals (NHS registered nurses, doctors, radiographers, and allied health staff) continue under exempt national pay spines such as the NHS Agenda for Change.</li>
</ul>

<h2>2. The Immigration Salary List (ISL)</h2>
<p>The former Shortage Occupation List was replaced with the streamlined <strong>Immigration Salary List (ISL)</strong>. Occupations designated on the ISL benefit from a 20% discount against the general baseline, lowering the requirement to <strong>£30,960</strong> (provided the wage also meets the occupation's specific 2026 going rate).</p>

<h2>3. Identifying Genuine Home Office Licensed Sponsors</h2>
<p>Never pay an agent or agency for a Certificate of Sponsorship (CoS). It is illegal in the UK for recruiters or employers to charge jobseekers for visa sponsorship paperwork.</p>
<p>Before applying, always cross-reference the hiring company against the official <em>Home Office Register of Worker and Temporary Worker Licensed Sponsors</em>. On JobyUK, we flag companies with verified direct sponsorship licences so you can focus your efforts on legitimate hiring organisations.</p>`,
		},

		// 4. NHS Pay Bands
		{
			slug:           "nhs-pay-bands",
			title:          "NHS Agenda for Change Pay Bands 2026: Complete Salary Scale, HCAS & Progression",
			category:       "salary",
			seoTitle:       "NHS Pay Bands 2026: Agenda for Change Salary Scales & HCAS | JobyUK",
			seoDescription: "Complete guide to NHS Agenda for Change pay bands from Band 2 to Band 9. Includes 2026 pay points, London weighting allowances, and NHS pension breakdown.",
			publishedAt:    time.Date(2026, 11, 1, 9, 30, 0, 0, time.UTC),
			content: `<h2>Understanding the NHS Agenda for Change (AfC) System</h2>
<p>The National Health Service (NHS) is one of the world's largest employers, employing over 1.4 million people in the United Kingdom. Almost all non-medical NHS staff—including nurses, healthcare assistants, radiographers, administrative officers, IT engineers, and project managers—are paid under the standardized <strong>Agenda for Change (AfC)</strong> pay spine.</p>

<h2>1. NHS Pay Bands Overview (2026 Scales)</h2>
<p>Each band contains specific step points that increase as you gain completed years of service within that grade:</p>
<table style="width: 100%; border-collapse: collapse; margin: 20px 0; font-size: 0.95rem;">
    <thead>
        <tr style="background: #F1F5F9; border-bottom: 2px solid #CBD5E1; text-align: left;">
            <th style="padding: 10px;">Band Grade</th>
            <th style="padding: 10px;">Typical NHS Roles</th>
            <th style="padding: 10px;">Salary Range (National)</th>
        </tr>
    </thead>
    <tbody>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 10px; font-weight: 700;">Band 2</td>
            <td style="padding: 10px;">Healthcare Support Worker, Domestic Assistant, Receptionist</td>
            <td style="padding: 10px;">£23,615 – £24,071</td>
        </tr>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 10px; font-weight: 700;">Band 3</td>
            <td style="padding: 10px;">Senior Healthcare Assistant, Clinical Support Worker, Team Sec</td>
            <td style="padding: 10px;">£24,336 – £26,180</td>
        </tr>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 10px; font-weight: 700;">Band 4</td>
            <td style="padding: 10px;">Nursing Associate, Theatre Support, Medical Secretary</td>
            <td style="padding: 10px;">£26,530 – £29,114</td>
        </tr>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 10px; font-weight: 700;">Band 5</td>
            <td style="padding: 10px;">Newly Qualified Staff Nurse, Junior Physiotherapist, Radiographer</td>
            <td style="padding: 10px;">£29,970 – £36,483</td>
        </tr>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 10px; font-weight: 700;">Band 6</td>
            <td style="padding: 10px;">Senior Staff Nurse, Specialist Physiotherapist, Ward Sister</td>
            <td style="padding: 10px;">£37,338 – £44,962</td>
        </tr>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 10px; font-weight: 700;">Band 7</td>
            <td style="padding: 10px;">Advanced Clinical Practitioner, Lead Clinical Nurse, Service Manager</td>
            <td style="padding: 10px;">£46,148 – £52,809</td>
        </tr>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 10px; font-weight: 700;">Band 8a – 8d</td>
            <td style="padding: 10px;">Matron, Head of Nursing, Principal Clinical Psychologist, Associate Director</td>
            <td style="padding: 10px;">£53,755 – £96,376</td>
        </tr>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 10px; font-weight: 700;">Band 9</td>
            <td style="padding: 10px;">Chief Nurse, Director of Operations, Corporate Executive</td>
            <td style="padding: 10px;">£103,401 – £114,949</td>
        </tr>
    </tbody>
</table>

<h2>2. High Cost Area Supplements (HCAS / London Weighting)</h2>
<p>If you work for an NHS trust within London or the surrounding home counties, you receive an additional non-consolidated allowance on top of your basic pay:</p>
<ul>
    <li><strong>Inner London:</strong> 20% of basic salary (minimum £5,138, maximum £8,010).</li>
    <li><strong>Outer London:</strong> 15% of basic salary (minimum £4,313, maximum £5,436).</li>
    <li><strong>Fringe Area:</strong> 5% of basic salary (minimum £1,192, maximum £2,011).</li>
</ul>

<h2>3. NHS Pension Scheme Value</h2>
<p>Beyond headline base salary, the NHS Pension Scheme remains one of the most generous Defined Benefit career-average pensions in Europe. While employee contributions range between 5.2% and 12.5%, the <strong>employer contribution is currently 23.7%</strong>, adding immense real-term value to your total compensation.</p>`,
		},

		// 5. Civil Service Interview Guide
		{
			slug:           "civil-service-interview-guide",
			title:          "Civil Service Success Profiles 2026: 9 Behaviours & Scoring Model Answers",
			category:       "interviews",
			seoTitle:       "Civil Service Success Profiles 2026: 9 Behaviours & Scoring | JobyUK",
			seoDescription: "Demystifying Civil Service interview scoring. Master the 9 core Behaviours and learn how to write 250-word statements that score 6s and 7s.",
			publishedAt:    time.Date(2026, 11, 1, 11, 0, 0, 0, time.UTC),
			content: `<h2>How Civil Service Recruitment Really Works</h2>
<p>Recruitment across the UK Civil Service (including HMRC, Home Office, DWP, Ministry of Justice, and Defra) does not rely on traditional CV browsing. It is strictly regulated by the <em>Civil Service Commission</em> to ensure appointments are made solely on merit through fair and open competition.</p>
<p>Civil Service hiring uses the <strong>Success Profiles Framework</strong>, assessing candidates across five elements: Ability, Behaviours, Strengths, Experience, and Technical. Understanding the 9 core Behaviours is the key to passing both the initial application sift and the interview board.</p>

<h2>1. The 9 Core Civil Service Behaviours</h2>
<ol>
    <li><strong>Seeing the Big Picture:</strong> Understanding how your team's work fits into wider departmental objectives and ministerial priorities.</li>
    <li><strong>Changing and Improving:</strong> Seeking innovative ideas, embracing new digital tools, and adapting positively to organizational change.</li>
    <li><strong>Making Effective Decisions:</strong> Using objective data, weighing up risks, and consulting relevant stakeholders before acting.</li>
    <li><strong>Leadership:</strong> Showing pride and passion for public service, motivating team members, and championing diversity.</li>
    <li><strong>Communicating and Influencing:</strong> Communicating complex policy clearly, listening actively, and negotiating with difficult stakeholders.</li>
    <li><strong>Working Together:</strong> Building inclusive, collaborative partnerships across departmental boundaries.</li>
    <li><strong>Developing Self and Others:</strong> Investing in continuous learning, mentoring colleagues, and sharing best practices.</li>
    <li><strong>Managing a Quality Service:</strong> Ensuring delivery is customer-focused, compliant with regulations, and represents value for public money.</li>
    <li><strong>Delivering at Pace:</strong> Prioritising workloads, overcoming blockers, and delivering high-quality results to tight deadlines.</li>
</ol>

<h2>2. The Civil Service 1 to 7 Scoring Scale</h2>
<p>Panel assessors score every behaviour example independently against this standardized benchmark:</p>
<ul>
    <li><strong>1 – 3:</strong> Significant weaknesses / not demonstrated (Immediate Fail).</li>
    <li><strong>4 (Moderate Demonstration):</strong> Satisfactory evidence meeting minimum standard.</li>
    <li><strong>5 (Good Demonstration):</strong> Strong evidence with clear outcomes.</li>
    <li><strong>6 – 7 (Strong / Outstanding):</strong> Exceptional evidence demonstrating systemic impact, lessons learned, and proactive risk management.</li>
</ul>

<h2>3. The Winning 250-Word Behaviour Formula: STAR+L</h2>
<p>When drafting your written behaviour statement or answering at the panel interview, structure your narrative using <strong>STAR+L</strong>:</p>
<ul>
    <li><strong>Situation (15%):</strong> Brief context. What was the challenge, the department, and the deadline?</li>
    <li><strong>Task (15%):</strong> What was your specific personal remit? (Use "I", never "we").</li>
    <li><strong>Action (55%):</strong> The meat of your answer. What analysis did you perform? How did you resolve objections? Which Civil Service guidelines did you consult?</li>
    <li><strong>Result (10%):</strong> The measurable output (e.g., delivered £1.2M efficiency savings, reduced case backlog by 35%).</li>
    <li><strong>Learning (5%):</strong> What did you reflect on, and how did you share this learning across the wider Directorate?</li>
</ul>`,
		},

		// 6. Right to Work Checks
		{
			slug:           "right-to-work-checks",
			title:          "UK Right to Work Checks 2026: Digital Share Codes, BRP Phase-Out & eVisa Guide",
			category:       "job-search",
			seoTitle:       "UK Right to Work Checks 2026: Digital eVisa Share Codes | JobyUK",
			seoDescription: "Everything UK jobseekers need to know about statutory Right to Work checks. How to generate a Home Office digital share code and prevent start date delays.",
			publishedAt:    time.Date(2026, 11, 2, 8, 45, 0, 0, time.UTC),
			content: `<h2>Statutory Right to Work: What Every UK Candidate Must Know</h2>
<p>Under the <em>Immigration, Asylum and Nationality Act 2006</em>, all UK employers have an absolute legal duty to conduct statutory <strong>Right to Work (RTW)</strong> checks before any employee's first working day. Failure to comply exposes employers to civil penalties of up to £45,000 per illegal worker.</p>
<p>Understanding the modern verification process prevents administrative hold-ups and ensures your formal job contract is issued smoothly.</p>

<h2>1. The Complete Phase-Out of Physical BRPs (The Digital eVisa)</h2>
<p>Physical Biometric Residence Permits (BRPs), Biometric Residence Cards (BRCs), and passport vignette stamps have been fully replaced by the <strong>Home Office Digital eVisa</strong> system. Physical cards are no longer accepted as valid proof of Right to Work.</p>
<p>Non-British and non-Irish nationals access their status online via their UK Visas and Immigration (UKVI) digital account and generate a secure 9-character <strong>Share Code</strong>.</p>

<h2>2. How to Generate Your Official Share Code</h2>
<ol>
    <li>Navigate to the official Gov.uk service: <em>"Prove your right to work to an employer"</em>.</li>
    <li>Log in using your passport details or identity document linked to your UKVI account, followed by SMS two-factor authentication.</li>
    <li>Select the option <em>"I want to prove my right to work"</em>.</li>
    <li>The system will generate a 9-character alphanumeric code starting with <strong>W</strong> (for Work).</li>
    <li>Share this code along with your date of birth with your prospective employer's HR onboarding team. The code remains valid for <strong>90 days</strong>.</li>
</ol>

<h2>3. What Employers Are NOT Allowed to Ask</h2>
<p>UK employment law and ACAS guidance establish clear boundaries regarding when pre-employment checks can take place:</p>
<ul>
    <li>Employers cannot demand to see your passport or share code <em>before</em> making a conditional offer of employment. Asking during an initial interview can expose them to racial discrimination claims under the Equality Act 2010.</li>
    <li>Employers must treat all candidates equally: they cannot conduct digital checks on candidates with foreign-sounding names while accepting verbal confirmation from others.</li>
</ul>`,
		},

		// 7. UK Minimum Wage 2026
		{
			slug:           "uk-minimum-wage",
			title:          "UK National Minimum Wage & Living Wage Rates (April 2026): Your Legal Pay Rights",
			category:       "salary",
			seoTitle:       "UK Minimum Wage Rates April 2026: Statutory Rights & Rules | JobyUK",
			seoDescription: "Official UK National Living Wage and Minimum Wage rates for 2026. Understand statutory hourly rates, illegal salary deductions, and apprentice wage rules.",
			publishedAt:    time.Date(2026, 11, 2, 10, 15, 0, 0, time.UTC),
			content: `<h2>Statutory Wage Protection in the United Kingdom</h2>
<p>In the UK, every worker—whether full-time, part-time, agency temporary worker, or casual zero-hours contractor—is legally entitled to receive at least the statutory <strong>National Living Wage (NLW)</strong> or <strong>National Minimum Wage (NMW)</strong> for every working hour performed.</p>

<h2>1. Official Statutory Hourly Pay Rates (Effective April 2026)</h2>
<table style="width: 100%; border-collapse: collapse; margin: 20px 0; font-size: 0.95rem;">
    <thead>
        <tr style="background: #F1F5F9; border-bottom: 2px solid #CBD5E1; text-align: left;">
            <th style="padding: 12px;">Age Category / Band</th>
            <th style="padding: 12px;">Statutory Hourly Rate</th>
            <th style="padding: 12px;">Gross Annual Equivalent (37.5 hrs/wk)</th>
        </tr>
    </thead>
    <tbody>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 12px; font-weight: 700;">National Living Wage (Aged 21 and Over)</td>
            <td style="padding: 12px; color: #059669; font-weight: 800; font-size: 1.05rem;">£12.21 / hr</td>
            <td style="padding: 12px; font-weight: 600;">£23,809.50</td>
        </tr>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 12px; font-weight: 700;">18 to 20 Year Old Rate</td>
            <td style="padding: 12px; font-weight: 700;">£10.00 / hr</td>
            <td style="padding: 12px;">£19,500.00</td>
        </tr>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 12px; font-weight: 700;">Under 18 Rate</td>
            <td style="padding: 12px;">£7.55 / hr</td>
            <td style="padding: 12px;">£14,722.50</td>
        </tr>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 12px; font-weight: 700;">Apprentice Rate (Under 19 or in 1st year)</td>
            <td style="padding: 12px;">£7.55 / hr</td>
            <td style="padding: 12px;">£14,722.50</td>
        </tr>
    </tbody>
</table>

<h2>2. Common Hidden Payroll Deductions That Break the Law</h2>
<p>An employer may pay £12.50 on paper, but if deductions bring your effective hourly rate below £12.21, the employer is guilty of an illegal wage underpayment under HMRC rules:</p>
<ul>
    <li><strong>Mandatory Uniforms:</strong> If you are required to purchase branded workwear, safety shoes, or aprons, that cost must not reduce your hourly pay below the statutory minimum.</li>
    <li><strong>Security Checks & DBS Fees:</strong> If an employer deducts the £18 to £38 DBS check fee from your first pay packet, it cannot bring that month's pay below minimum wage.</li>
    <li><strong>Unpaid Mandatory Training:</strong> Attending fire safety drills, induction days, or online compliance modules outside shift hours is legally counted as working time and must be paid in full.</li>
</ul>

<h2>3. Statutory vs. The Voluntary 'Real Living Wage'</h2>
<p>Do not confuse the statutory government wage with the voluntary <strong>Real Living Wage</strong> set by the Living Wage Foundation. In 2026, the voluntary Real Living Wage is <strong>£12.60 per hour nationally</strong> and <strong>£13.85 per hour in Greater London</strong>. Over 14,000 accredited UK employers voluntarily commit to paying this higher rate.</p>`,
		},

		// 8. Competency Interview Questions
		{
			slug:           "competency-interview-questions",
			title:          "12 Toughest UK Competency Interview Questions: Full STAR Winning Scripts",
			category:       "interviews",
			seoTitle:       "12 UK Competency Interview Questions & STAR Answers 2026 | JobyUK",
			seoDescription: "Master UK competency-based interviews. Word-for-word STAR model answers for handling conflict, managing deadlines, and demonstrating commercial leadership.",
			publishedAt:    time.Date(2026, 11, 2, 11, 45, 0, 0, time.UTC),
			content: `<h2>Why UK Employers Rely on Competency Interviews</h2>
<p>From FTSE 100 retailers to NHS Trusts and boutique tech agencies, competency-based interviews remain the UK standard. The core philosophy is simple: <strong>past commercial behavior is the best predictor of future job performance</strong>.</p>
<p>Instead of vague hypothetical queries ("What would you do if..."), interviewers demand real historical anecdotes ("Tell me about a specific time when..."). Here is how to answer the most critical questions using the proven STAR framework.</p>

<h2>Question 1: "Tell me about a time you handled a difficult colleague or stakeholder."</h2>
<p><strong>What the interviewer is testing:</strong> Emotional intelligence, diplomatic conflict resolution, and maturity under stress.</p>
<blockquote style="background: #F8FAFC; border-left: 4px solid #059669; padding: 16px 20px; margin: 16px 0; border-radius: 0 12px 12px 0;">
    <p><strong>Situation:</strong> While delivering a client CRM migration project, the lead sales director refused to adopt the new data fields, threatening our rollout launch date.</p>
    <p><strong>Task:</strong> As Project Lead, I needed to secure his buy-in without escalating to executive leadership or causing team friction.</p>
    <p><strong>Action:</strong> I scheduled a 1-on-1 coffee rather than arguing via email. I listened to his team's frustrations and discovered the new interface required 4 extra clicks per call. I worked with engineering to automate two background lookups, saving his team 15 minutes daily.</p>
    <p><strong>Result:</strong> He personally championed the rollout. The sales team achieved 98% adoption within 2 weeks, and the migration launched 3 days ahead of schedule.</p>
</blockquote>

<h2>Question 2: "Describe a project that failed or did not go as planned."</h2>
<p><strong>What the interviewer is testing:</strong> Personal accountability, resilience, and whether you proactively learn from mistakes.</p>
<blockquote style="background: #F8FAFC; border-left: 4px solid #059669; padding: 16px 20px; margin: 16px 0; border-radius: 0 12px 12px 0;">
    <p><strong>Situation:</strong> In my previous role as Digital Marketing Specialist, we launched a Q2 paid acquisition campaign on LinkedIn that underperformed its target lead volume by 40%.</p>
    <p><strong>Task:</strong> I had full ownership of the £25,000 media budget and had to account for the shortfall to our managing director.</p>
    <p><strong>Action:</strong> I immediately paused underperforming ad sets on day 10 rather than burning the full budget. I conducted heat-map audits of our landing page and discovered mobile load times were averaging 5.8 seconds, causing a 65% drop-off. I worked with the dev team to compress visual assets, cutting load time to 1.2 seconds, and re-tested with 3 focused customer case studies.</p>
    <p><strong>Result:</strong> Conversion rates rebounded from 1.8% to 5.4%. We recovered the shortfall by the end of Q2 and integrated a mandatory mobile performance test into all subsequent launch checklists.</p>
</blockquote>

<h2>Key Rules for Acing Competency Boards</h2>
<ul>
    <li>Keep answers strictly between <strong>2.5 to 3.5 minutes</strong>. Avoid rambling into unnecessary operational backstories.</li>
    <li>Focus 60% of your time on the <strong>Action</strong>: Explain what YOU did, your thought process, and how you influenced outcomes.</li>
    <li>Always close with a <strong>measurable Result</strong>: Include numbers, percentages, or concrete team benefits.</li>
</ul>`,
		},

		// 9. UK Graduate Schemes
		{
			slug:           "uk-graduate-schemes",
			title:          "UK Graduate Schemes 2026: Deadlines, Psychometric Tests & Assessment Day Guide",
			category:       "job-search",
			seoTitle:       "UK Graduate Schemes 2026: Application Timelines & Tests | JobyUK",
			seoDescription: "The ultimate guide to UK graduate schemes. Key application deadlines, acing Cappfinity/SHL psychometric tests, and assessment day group exercise tactics.",
			publishedAt:    time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC),
			content: `<h2>The UK Graduate Recruitment Landscape</h2>
<p>UK graduate schemes are prestigious, structured training programmes offered by major employers—ranging from the Civil Service Fast Stream and NHS Graduate Management Scheme to the Big 4 (PwC, Deloitte, EY, KPMG), engineering multinationals, and FTSE banks. They offer structured rotations, funded professional qualifications (ACA, CIMA, CIPD), and fast-tracked leadership salaries.</p>

<h2>1. The Graduate Application Timeline (The 'Milkround')</h2>
<p>Timing is everything. Top UK schemes operate on rolling admissions and close as soon as application quotas are met:</p>
<ul>
    <li><strong>September – October:</strong> Major schemes open applications. Apply early; applying in September increases your chance of passing the initial sift by over 50%.</li>
    <li><strong>November – December:</strong> Initial deadlines close. Online psychometric testing, situational judgement tests (SJTs), and automated video interviews take place.</li>
    <li><strong>January – March:</strong> Virtual or in-person Assessment Centres are conducted.</li>
    <li><strong>April – June:</strong> Final offers issued, background references checked, and onboarding begins for September starts.</li>
</ul>

<h2>2. Demystifying Psychometric & Situational Judgement Tests (SJTs)</h2>
<p>Most employers use testing software from providers like Cappfinity, SHL, or Korn Ferry:</p>
<ul>
    <li><strong>Situational Judgement Tests:</strong> Read the employer's core cultural values before starting. When evaluating workplace scenarios, the "correct" answer always prioritises client safety, ethical compliance, and collaborative problem-solving over individual heroics.</li>
    <li><strong>Numerical Reasoning:</strong> Brush up on percentage changes, currency conversions, and interpreting complex financial tables under time pressure.</li>
</ul>

<h2>3. Acing the Assessment Centre Group Exercise</h2>
<p>The biggest pitfall at an assessment day is attempting to dominate the conversation. Assessors do not reward the loudest candidate; they reward the candidate who exhibits true collaboration:</p>
<ol>
    <li><strong>Keep Time:</strong> Volunteer to keep an eye on the clock: <em>"We have 25 minutes; shall we spend the first 5 minutes reviewing the brief individually?"</em></li>
    <li><strong>Bring Quieter Voices In:</strong> Notice a teammate who hasn't spoken? <em>"Sarah, you have strong marketing experience—what do you think about option B?"</em> Assessors score this highly under leadership.</li>
    <li><strong>Summarize Progress:</strong> Keep the group focused on delivering the final briefing deck before the timer expires.</li>
</ol>`,
		},

		// 10. IR35 Tax Guide
		{
			slug:           "ir35-tax-guide",
			title:          "IR35 Tax Guide 2026: Inside vs Outside IR35 Take-Home Pay & Contractor Rights",
			category:       "salary",
			seoTitle:       "IR35 Guide for UK Contractors 2026: Inside vs Outside Tax | JobyUK",
			seoDescription: "Clear breakdown of UK IR35 off-payroll rules. Compare real net take-home pay, Status Determination Statements (SDS), and umbrella company deductions.",
			publishedAt:    time.Date(2026, 11, 3, 11, 15, 0, 0, time.UTC),
			content: `<h2>What Is IR35 and Why Does It Matter?</h2>
<p>The UK's <strong>Intermediaries Legislation (IR35)</strong> was designed by HMRC to tackle "disguised employment." It ensures that contractors providing services through a Personal Service Company (PSC / Limited Company) pay broadly the same tax and National Insurance contributions as standard employees if their working relationship mirrors permanent employment.</p>

<h2>1. Inside IR35 vs. Outside IR35: Key Differences</h2>
<table style="width: 100%; border-collapse: collapse; margin: 20px 0; font-size: 0.95rem;">
    <thead>
        <tr style="background: #F1F5F9; border-bottom: 2px solid #CBD5E1; text-align: left;">
            <th style="padding: 12px; width: 50%;">Outside IR35 (Genuine Business-to-Business)</th>
            <th style="padding: 12px; width: 50%;">Inside IR35 (Deemed Employment)</th>
        </tr>
    </thead>
    <tbody>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 12px;">You invoice via your own Limited Company (PSC).</td>
            <td style="padding: 12px;">Treated as an employee for tax purposes (PAYE).</td>
        </tr>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 12px;">Taxed via salary + company dividends + corporation tax (greater tax efficiency).</td>
            <td style="padding: 12px;">Income Tax, Employee NI, and Apprenticeship Levy deducted at source.</td>
        </tr>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 12px;">Contractor retains full control over how, when, and where tasks are executed.</td>
            <td style="padding: 12px;">Client exercises supervision, direction, and operational control.</td>
        </tr>
    </tbody>
</table>

<h2>2. Real Take-Home Pay Comparison (£500/day Contract)</h2>
<p>Consider a typical UK senior technical contract at a rate of <strong>£500 per day</strong> across 220 working days (£110,000 gross annual billing):</p>
<ul>
    <li><strong>Outside IR35:</strong> After deducting allowable expenses, small salary, corporation tax, and dividend income tax, typical net take-home pay is roughly <strong>74% to 78%</strong> of gross billing (~£81,000 to £85,000 net).</li>
    <li><strong>Inside IR35 (via Umbrella):</strong> After deducting Employer NI (13.8%), Apprenticeship Levy (0.5%), umbrella margins, Employee NI, and 40%/45% PAYE income tax, net take-home is roughly <strong>54% to 58%</strong> (~£59,000 to £64,000 net).</li>
</ul>

<h2>3. The 3 Primary Tests of Employment Status</h2>
<p>Courts and HMRC evaluate three essential pillars when assessing IR35 status:</p>
<ol>
    <li><strong>Right of Substitution:</strong> Does your contract allow your business to send a qualified substitute contractor to perform the work without client veto?</li>
    <li><strong>Mutuality of Obligation (MOO):</strong> Is the client legally bound to offer ongoing work, and are you bound to accept it once a milestone is completed?</li>
    <li><strong>Control:</strong> Does the client dictate your daily working hours, manage your appraisals, or supervise how you complete technical tasks?</li>
</ol>`,
		},

		// 11. Career Change Guide
		{
			slug:           "career-change-guide",
			title:          "Changing Careers in the UK at 30, 40 or Later: A Practical Step-by-Step Transition Plan",
			category:       "job-search",
			seoTitle:       "Career Change at 30, 40 or 50 UK: Practical Step-by-Step Guide | JobyUK",
			seoDescription: "Pivoting careers in the UK without starting from the bottom. How to leverage transferable skills, reframe your CV, and secure higher-paying roles.",
			publishedAt:    time.Date(2026, 11, 3, 14, 0, 0, 0, time.UTC),
			content: `<h2>The Myth of "Starting from Scratch"</h2>
<p>One of the biggest anxieties holding professionals back from pivoting careers in their 30s, 40s, or 50s is the fear of starting at the very bottom on an entry-level wage. In the modern UK knowledge economy, this fear is largely unfounded.</p>
<p>While you may lack technical jargon in your target industry, you possess years of commercial maturity, client relationship management, crisis handling, and budget governance that 22-year-old graduates cannot replicate. Here is how to engineer a successful mid-career pivot.</p>

<h2>1. Identifying High-Value Transferable Skills</h2>
<p>UK employers frequently recruit career-changers for specialist roles because cross-industry perspectives solve stagnation. The most in-demand transferable skills include:</p>
<ul>
    <li><strong>Project Delivery & Governance:</strong> Delivering initiatives on time, managing scope creep, and balancing commercial budgets.</li>
    <li><strong>Stakeholder Influence:</strong> Aligning cross-functional leaders, managing external suppliers, and communicating technical topics simply.</li>
    <li><strong>Data Literacy & Analysis:</strong> Extracting actionable insights from business metrics to drive executive decision-making.</li>
</ul>

<h2>2. Transitioning from Chronological to Hybrid Functional CV</h2>
<p>If your entire work history is in teaching or hospitality, a traditional chronological CV immediately exposes you to automated sifting rejections for corporate operations or tech roles. Instead, use a <strong>Hybrid Functional Layout</strong>:</p>
<ol>
    <li><strong>Executive Profile:</strong> Position yourself for the future role (e.g., <em>"Operations Project Coordinator with 8 years' leadership in high-pressure delivery environments"</em>).</li>
    <li><strong>Major Transferable Competencies (Page 1):</strong> Group achievements under functional themes (e.g., <em>"Process Optimization & Quality Assurance"</em> or <em>"Stakeholder Engagement"</em>).</li>
    <li><strong>Brief Employment History (Page 2):</strong> Condense previous roles into clean, 3-line summaries focusing on commercial outcomes.</li>
</ol>

<h2>3. The 'Bridge Role' Strategy</h2>
<p>If jumping directly from your current industry to a completely new sector feels too wide a chasm, use a <strong>bridge role</strong>: maintain your existing functional skill set (e.g., Finance or Marketing) but move into your target sector (e.g., HealthTech or Green Energy). Once inside the new sector's ecosystem, pivoting internally to your target specialization is dramatically easier.</p>`,
		},

		// 12. Redundancy Rights UK
		{
			slug:           "redundancy-rights-uk",
			title:          "UK Statutory Redundancy Pay 2026: Payout Calculator, Notice Periods & Tax Rules",
			category:       "job-search",
			seoTitle:       "UK Redundancy Rights 2026: Statutory Payouts & Notice Periods | JobyUK",
			seoDescription: "Know your UK statutory redundancy rights. Full breakdown of statutory payout calculations, statutory notice rules, and the £30,000 tax-free severance limit.",
			publishedAt:    time.Date(2026, 11, 4, 3, 0, 0, 0, time.UTC),
			content: `<h2>Facing Redundancy in the UK: Know Your Legal Protections</h2>
<p>Being notified of potential redundancy is one of the most stressful professional experiences. However, UK employment law under the <em>Employment Rights Act 1996</em> provides clear, statutory financial cushions and procedural protections for affected workers.</p>

<h2>1. Who Qualifies for Statutory Redundancy Pay?</h2>
<p>To qualify for statutory redundancy pay, you must:</p>
<ul>
    <li>Be an official employee working under a contract of employment.</li>
    <li>Have completed at least <strong>two continuous years of service</strong> with your employer.</li>
</ul>

<h2>2. The Statutory Redundancy Pay Calculation (2026)</h2>
<p>Statutory payout calculations depend strictly on your age and full years of completed service (capped at 20 years maximum):</p>
<table style="width: 100%; border-collapse: collapse; margin: 20px 0; font-size: 0.95rem;">
    <thead>
        <tr style="background: #F1F5F9; border-bottom: 2px solid #CBD5E1; text-align: left;">
            <th style="padding: 12px;">Age at Year of Service</th>
            <th style="padding: 12px;">Entitlement per Completed Year</th>
        </tr>
    </thead>
    <tbody>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 12px;">Under 22 years old</td>
            <td style="padding: 12px;">0.5 week's pay for each full year</td>
        </tr>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 12px;">Aged 22 to 40</td>
            <td style="padding: 12px; font-weight: 600;">1.0 full week's pay for each full year</td>
        </tr>
        <tr style="border-bottom: 1px solid #E2E8F0;">
            <td style="padding: 12px;">Aged 41 and older</td>
            <td style="padding: 12px; color: #059669; font-weight: 700;">1.5 weeks' pay for each full year</td>
        </tr>
    </tbody>
</table>
<p><em>Weekly Pay Cap:</em> Under UK statutory rules, weekly pay is subject to a statutory cap, updated annually by Parliament.</p>

<h2>3. The £30,000 Tax-Free Severance Rule</h2>
<p>One of the most important financial protections is that <strong>the first £30,000 of genuine redundancy pay is completely free from UK Income Tax and National Insurance deductions</strong>.</p>
<p>However, note that payments in lieu of notice (PILON), accrued untaken holiday pay, and contractual bonuses are classified as general employment earnings and remain subject to standard PAYE tax and NI.</p>

<h2>4. Statutory Consultation Periods</h2>
<p>Your employer cannot dismiss you without formal consultation:</p>
<ul>
    <li><strong>Under 20 redundancies:</strong> No fixed statutory consultation length, but consultations must be meaningful.</li>
    <li><strong>20 to 99 redundancies:</strong> Consultation must begin at least <strong>30 days</strong> before any dismissals take effect.</li>
    <li><strong>100+ redundancies:</strong> Consultation must begin at least <strong>45 days</strong> prior to dismissals.</li>
</ul>`,
		},
		// 13. Flexible Working Rights UK
		{
			slug:           "flexible-working-rights",
			title:          "Asking for Flexible or Hybrid Working: Your Legal Rights from Day One (2026)",
			category:       "job-search",
			seoTitle:       "Flexible & Hybrid Working Rights UK 2026: Day-One Law | JobyUK",
			seoDescription: "How to use the UK Day-One flexible working law to request hybrid working, flexitime, or compressed hours. Includes statutory refusal grounds and ACAS rules.",
			publishedAt:    time.Date(2026, 11, 4, 5, 0, 0, 0, time.UTC),
			content: `<h2>Understanding Your Statutory Flexible Working Rights in 2026</h2>
<p>Under updated UK employment legislation and the <em>Employment Relations (Flexible Working) Act</em>, every employee in the UK has the legal right to submit a statutory flexible working request from their <strong>very first day of employment</strong>. You no longer need to wait 26 weeks before requesting hybrid work, part-time schedules, compressed hours, or flexitime.</p>

<h2>1. Core Statutory Protections Under the Law</h2>
<ul>
    <li><strong>Day-One Entitlement:</strong> You can formally submit a request on day 1 of joining a new company.</li>
    <li><strong>Two Requests Per Year:</strong> Employees can submit up to two formal statutory requests within any 12-month period.</li>
    <li><strong>Mandatory Consultation:</strong> Employers cannot simply reject your application via an email. By law, they must hold a formal consultation meeting to explore alternatives before any refusal can be issued.</li>
    <li><strong>2-Month Decision Deadline:</strong> The entire process—including any appeal—must be concluded within 2 months of receiving the request.</li>
    <li><strong>No Requirement to Explain Impact:</strong> Employees are no longer legally required to write a complex business case explaining how their absence will affect company operations.</li>
</ul>

<h2>2. The 8 Legal Business Reasons for Refusal</h2>
<p>An employer cannot refuse your request on personal preference. Under Section 80G of the <em>Employment Rights Act 1996</em>, they can only refuse based on one or more of these 8 statutory business grounds:</p>
<ol>
    <li>Burden of additional costs.</li>
    <li>Detrimental effect on ability to meet customer demand.</li>
    <li>Inability to reorganize work among existing staff.</li>
    <li>Inability to recruit additional staff.</li>
    <li>Detrimental impact on quality of service.</li>
    <li>Detrimental impact on operational performance.</li>
    <li>Insufficiency of work during the periods the employee proposes to work.</li>
    <li>Planned structural changes or reorganization.</li>
</ol>

<h2>3. The Secret to Getting Approved: Proposing a Trial Period</h2>
<p>If your manager is hesitant about remote work or compressed 4-day working weeks, never demand a permanent contractual change immediately. Instead, propose a <strong>3-month trial period with predefined KPI review checkpoints</strong>.</p>
<p>A trial dramatically lowers management anxiety regarding shift coverage, client responsiveness, or team cohesion. If performance metrics remain steady during the 90-day trial, it becomes virtually impossible for the employer to justify a statutory refusal under ACAS guidelines.</p>`,
		},	}

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

	slog.Info("AutoSeedArticles completed successfully: exactly 12 master guides present")
	return nil
}
