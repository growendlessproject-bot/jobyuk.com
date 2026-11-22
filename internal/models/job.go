package models

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Job represents a job listing sourced from multiple providers (Reed.co.uk, FreeHire, Direct ATS).
type Job struct {
	ID             int64      `json:"id" db:"id"`
	ReedJobID      *int64     `json:"reed_job_id,omitempty" db:"reed_job_id"`
	Source         string     `json:"source" db:"source"`
	ExternalSlug   string     `json:"external_slug,omitempty" db:"external_slug"`
	Slug           string     `json:"slug" db:"slug"`
	Title          string     `json:"title" db:"title"`
	Description    string     `json:"description" db:"description"`
	EmployerID     *int64     `json:"employer_id,omitempty" db:"employer_id"`
	LocationID     *int       `json:"location_id,omitempty" db:"location_id"`
	LocationName   string     `json:"location_name" db:"location_name"`
	MinSalary      *float64   `json:"min_salary,omitempty" db:"min_salary"`
	MaxSalary      *float64   `json:"max_salary,omitempty" db:"max_salary"`
	YearlyMin      *float64   `json:"yearly_min,omitempty" db:"yearly_min"`
	YearlyMax      *float64   `json:"yearly_max,omitempty" db:"yearly_max"`
	Currency       string     `json:"currency" db:"currency"`
	SalaryType     string     `json:"salary_type" db:"salary_type"`
	ContractType   string     `json:"contract_type" db:"contract_type"`
	JobType        string     `json:"job_type" db:"job_type"`
	IsGraduate     bool       `json:"is_graduate" db:"is_graduate"`
	IsRemote       bool       `json:"is_remote" db:"is_remote"`
	ExternalURL    string     `json:"external_url" db:"external_url"`
	ReedURL        string     `json:"reed_url" db:"reed_url"`
	DatePosted     time.Time  `json:"date_posted" db:"date_posted"`
	ExpirationDate *time.Time `json:"expiration_date,omitempty" db:"expiration_date"`
	Status         string     `json:"status" db:"status"` // active, expired, removed
	PinterestPinURL string    `json:"pinterest_pin_url,omitempty" db:"pinterest_pin_url"`
	LastLinkCheckedAt *time.Time `json:"last_link_checked_at,omitempty" db:"last_link_checked_at"`
	LinkHTTPStatus    int        `json:"link_http_status,omitempty" db:"link_http_status"`
	LinkStatus        string     `json:"link_status,omitempty" db:"link_status"`
	LinkFailCount     int        `json:"link_fail_count,omitempty" db:"link_fail_count"`
	LinkCheckReason   string     `json:"link_check_reason,omitempty" db:"link_check_reason"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`

	// Joined fields (not stored directly)
	EmployerName        string `json:"employer_name,omitempty" db:"employer_name"`
	CompanyLogoURLValue string `json:"company_logo_url,omitempty" db:"company_logo_url"`
	CompanySlug         string `json:"company_slug,omitempty" db:"company_slug"`
}

// WorkMode returns "Remote" or "On-site".
func (j *Job) WorkMode() string {
	if j.IsRemote {
		return "Remote"
	}
	return "On-site"
}

// ContractTypeSimple returns a clean contract type name.
func (j *Job) ContractTypeSimple() string {
	if j.ContractType != "" {
		return strings.Title(strings.ToLower(j.ContractType))
	}
	return "Permanent"
}

// JobTypeSimple returns a clean job type name.
func (j *Job) JobTypeSimple() string {
	if j.JobType != "" {
		return strings.Title(strings.ToLower(j.JobType))
	}
	return "Full-time"
}

// IsExpired returns true if the job has expired.
func (j *Job) IsExpired() bool {
	if j.Status == "expired" || j.Status == "removed" {
		return true
	}
	if j.ExpirationDate != nil && j.ExpirationDate.Before(time.Now()) {
		return true
	}
	return false
}

// IsRecentlyExpired returns true if the job expired within the last 30 days.
func (j *Job) IsRecentlyExpired() bool {
	if j.ExpirationDate == nil {
		return false
	}
	return j.ExpirationDate.Before(time.Now()) &&
		j.ExpirationDate.After(time.Now().AddDate(0, 0, -30))
}

// FormattedSalary returns a human-readable salary string.
func (j *Job) FormattedSalary() string {
	if j.YearlyMin == nil && j.YearlyMax == nil {
		return "Competitive"
	}
	if j.YearlyMin != nil && j.YearlyMax != nil {
		if *j.YearlyMin == *j.YearlyMax {
			return formatGBP(*j.YearlyMin) + " per annum"
		}
		return formatGBP(*j.YearlyMin) + " - " + formatGBP(*j.YearlyMax) + " per annum"
	}
	if j.YearlyMin != nil {
		return "From " + formatGBP(*j.YearlyMin) + " per annum"
	}
	return "Up to " + formatGBP(*j.YearlyMax) + " per annum"
}

// SalaryLevel returns "high", "mid", or "low" for CSS class assignment.
func (j *Job) SalaryLevel() string {
	if j.YearlyMax != nil {
		if *j.YearlyMax >= 50000 {
			return "high"
		}
		if *j.YearlyMax >= 30000 {
			return "mid"
		}
	}
	return "low"
}

// TimeAgo returns a human-readable time since posting.
func (j *Job) TimeAgo() string {
	diff := time.Since(j.DatePosted)
	switch {
	case diff < time.Hour:
		return "Just now"
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	default:
		return j.DatePosted.Format("2 Jan 2006")
	}
}

func formatGBP(amount float64) string {
	if amount >= 1000 {
		return fmt.Sprintf("£%.0fk", amount/1000)
	}
	return fmt.Sprintf("£%.0f", amount)
}

// IsNew returns true if the job was posted within the last 3 days.
func (j *Job) IsNew() bool {
	return time.Since(j.DatePosted) < 3*24*time.Hour
}

// IsEasyApply returns true if the job has a direct external application URL.
func (j *Job) IsEasyApply() bool {
	return j.ExternalURL != ""
}

// CompanyInitial returns the first letter of the employer name for logo fallback.
func (j *Job) CompanyInitial() string {
	if j.EmployerName != "" {
		return string(j.EmployerName[0])
	}
	return "?"
}

var (
	foreignLocationKeywords = []string{
		"canada", "quebec", "montreal", "toronto", "vancouver",
		"france", "paris", "lyon", "marseille",
		"germany", "berlin", "munich", "frankfurt", "hamburg",
		"ireland", "dublin", "cork",
		"netherlands", "amsterdam", "rotterdam", "the hague",
		"spain", "madrid", "barcelona", "valencia",
		"united states", "usa", "us", "new york", "california", "san francisco", "texas",
		"australia", "sydney", "melbourne", "brisbane",
		"india", "mumbai", "delhi", "bangalore", "hyderabad",
		"poland", "warsaw", "krakow",
		"sweden", "stockholm", "gothenburg",
		"switzerland", "geneva", "zurich", "basel",
		"italy", "rome", "milan",
		"brazil", "sao paulo", "rio de janeiro",
		"japan", "tokyo",
		"singapore",
		"mexico", "mexico city",
		"belgium", "brussels", "antwerp",
		"austria", "vienna",
		"denmark", "copenhagen",
		"norway", "oslo",
		"finland", "helsinki",
		"portugal", "lisbon", "porto",
		"czechia", "czech republic", "prague",
		"china", "beijing", "shanghai", "hong kong",
		"uae", "dubai", "abu dhabi",
		"south africa", "johannesburg", "cape town",
		"new zealand", "auckland", "wellington",
		"romania", "bucharest",
		"hungary", "budapest",
		"greece", "athens",
	}

	ukPrefixRegex        = regexp.MustCompile(`^(?i)(united\s+kingdom|great\s+britain|uk)\s*[-–—:\s,]+\s*`)
	ukCountrySuffixRegex = regexp.MustCompile(`(?i)\s*[,–—\-]?\s*(united\s+kingdom\s+of\s+great\s+britain\s+and\s+northern\s+ireland|united\s+kingdom|great\s+britain|uk)\s*$`)
	ukRegionPrefixRegex  = regexp.MustCompile(`^(?i)(england|scotland|wales|northern\s+ireland)\s*[,–—\-:\s]+\s*`)
	multiUkSplitRegex    = regexp.MustCompile(`(?i)(?:,\s*|\s+)(?:United Kingdom(?: of Great Britain and Northern Ireland)?|UK|GB)\s*,\s*`)
	workTypeSuffixRegex  = regexp.MustCompile(`(?i)\s*[-–—/]\s*(on-site|onsite|hybrid|remote)\s*$`)
	foreignSlashRegex    = regexp.MustCompile(`(?i)\s*/\s*(nyc|new york|berlin|paris|zurich|amsterdam|madrid|dublin|usa|us)\s*$`)
	gbrPrefixRegex       = regexp.MustCompile(`(?i)^gbr\s*[-–—:\s]+`)

	ukCitiesIndex = []string{
		"London", "Manchester", "Birmingham", "Leeds", "Glasgow", "Edinburgh", "Liverpool",
		"Bristol", "Sheffield", "Newcastle", "Nottingham", "Cardiff", "Belfast", "Southampton",
		"Oxford", "Cambridge", "Brighton", "Reading", "Coventry", "Leicester", "Derby",
		"Plymouth", "Newport", "Swansea", "Aberdeen", "Milton Keynes", "Norwich", "York",
		"Exeter", "Bath", "Chester", "Preston", "Swindon", "Bournemouth", "Luton",
		"Warrington", "Blackpool", "Middlesbrough", "Huddersfield", "Blackburn", "Bolton",
		"Stockport", "Rochdale", "Salford", "Wigan", "Oldham", "Bury", "Carlisle", "Crewe",
		"Stoke", "Wolverhampton", "Dudley", "Walsall", "Solihull", "Telford", "Worcester",
		"Stafford", "Bradford", "Wakefield", "Hull", "Doncaster", "Rotherham", "Barnsley",
		"Halifax", "Harrogate", "Sunderland", "Durham", "Darlington", "Gateshead", "Hartlepool",
		"Northampton", "Lincoln", "Mansfield", "Chesterfield", "Corby", "Kettering",
		"Cheltenham", "Gloucester", "Poole", "Torquay", "Taunton", "Salisbury", "Truro",
		"Portsmouth", "Guildford", "Crawley", "Slough", "Basingstoke", "Maidstone",
		"Chelmsford", "Colchester", "Southend", "Bedford", "Stevenage", "Watford", "St Albans",
		"Inverness", "Dundee", "Perth", "Stirling", "Wrexham", "Derry", "Lisburn", "Newry",
		"Titchfield", "Fareham",
	}
)

func isForeignLocation(s string) bool {
	sLower := strings.ToLower(strings.TrimSpace(s))
	if sLower == "" {
		return false
	}
	if sLower == "northern ireland" || strings.Contains(sLower, "belfast") {
		return false
	}
	for _, f := range foreignLocationKeywords {
		if sLower == f || strings.HasPrefix(sLower, f+" ") || strings.HasSuffix(sLower, " "+f) || strings.Contains(sLower, f) {
			return true
		}
	}
	return false
}

func containsSentenceIndicators(s string) bool {
	sLower := strings.ToLower(s)
	indicators := []string{
		"location for", "locations for", "locations are", "based in", "successful",
		"please note", "due to the nature", "requirement of", "occasional", "office based",
		"travel", "hybrid -", "hybrid:", "on-site:", "remote:", "anywhere in",
	}
	for _, ind := range indicators {
		if strings.Contains(sLower, ind) {
			return true
		}
	}
	if strings.Contains(s, ".") || strings.Contains(s, ";") || strings.Contains(s, "!") {
		return true
	}
	return false
}

func extractCitiesFromText(text string) []string {
	textLower := strings.ToLower(text)
	var found []string
	seen := make(map[string]bool)

	for _, city := range ukCitiesIndex {
		pattern := `\b` + strings.ToLower(city) + `\b`
		re := regexp.MustCompile(pattern)
		if re.MatchString(textLower) {
			key := strings.ToLower(city)
			if !seen[key] {
				seen[key] = true
				found = append(found, city)
			}
		}
	}
	return found
}

func cleanSingleLocation(loc string) string {
	loc = strings.TrimSpace(loc)
	if loc == "" {
		return ""
	}

	// If it contains sentences or explanatory phrases, extract the real UK cities!
	if containsSentenceIndicators(loc) || len(loc) > 35 {
		cities := extractCitiesFromText(loc)
		if len(cities) > 0 {
			if len(cities) == 1 {
				return cities[0]
			}
			return cities[0] + ", " + cities[1]
		}
		if strings.Contains(strings.ToLower(loc), "national") || strings.Contains(strings.ToLower(loc), "uk") {
			return "United Kingdom"
		}
		return ""
	}

	// Remove work style suffix and foreign parts
	loc = workTypeSuffixRegex.ReplaceAllString(loc, "")
	loc = foreignSlashRegex.ReplaceAllString(loc, "")
	if strings.HasPrefix(strings.ToLower(loc), "gbr-remote") {
		return "Remote"
	}
	loc = gbrPrefixRegex.ReplaceAllString(loc, "")
	loc = strings.TrimSpace(loc)

	// If string contains recognized UK cities despite irregular prefixes or dashes
	cities := extractCitiesFromText(loc)
	if len(cities) > 0 {
		if strings.Contains(loc, " - ") || containsSentenceIndicators(loc) || len(loc) > 30 {
			if len(cities) == 1 {
				return cities[0]
			}
			return cities[0] + ", " + cities[1]
		}
	}

	// Remove leading UK prefixes: "United Kingdom- Kettering" -> "Kettering"
	loc = ukPrefixRegex.ReplaceAllString(loc, "")
	loc = strings.TrimSpace(loc)

	// Remove trailing UK suffix: "London, United Kingdom of Great Britain and Northern Ireland" -> "London"
	loc = ukCountrySuffixRegex.ReplaceAllString(loc, "")
	loc = strings.TrimSpace(loc)
	loc = strings.TrimSuffix(loc, ",")
	loc = strings.TrimSpace(loc)

	// If starts with "England, London" -> "London"
	if ukRegionPrefixRegex.MatchString(loc) {
		trimmed := ukRegionPrefixRegex.ReplaceAllString(loc, "")
		if strings.TrimSpace(trimmed) != "" {
			loc = strings.TrimSpace(trimmed)
		}
	}

	// If it has multiple comma parts: e.g. "Geneva, Switzerland, London, England" or "London, England"
	parts := strings.Split(loc, ",")
	if len(parts) > 1 {
		var cleanParts []string
		seen := make(map[string]bool)
		for _, p := range parts {
			cp := strings.TrimSpace(p)
			if cp == "" {
				continue
			}
			// Skip foreign items
			if isForeignLocation(cp) {
				continue
			}
			cpLower := strings.ToLower(cp)
			// Drop redundant country/region if we have a city already
			if cpLower == "united kingdom" || cpLower == "uk" || cpLower == "great britain" || cpLower == "gb" ||
				cpLower == "united kingdom of great britain and northern ireland" || cpLower == "national" || cpLower == "england" {
				continue
			}
			if !seen[cpLower] {
				seen[cpLower] = true
				cleanParts = append(cleanParts, cp)
			}
		}
		if len(cleanParts) > 0 {
			if len(cleanParts) == 2 && (strings.EqualFold(cleanParts[1], "England") || strings.EqualFold(cleanParts[1], "UK")) {
				loc = cleanParts[0]
			} else {
				loc = strings.Join(cleanParts, ", ")
			}
		} else {
			return ""
		}
	} else {
		if isForeignLocation(loc) {
			return ""
		}
		if strings.EqualFold(loc, "national") || strings.EqualFold(loc, "england") || strings.EqualFold(loc, "gb") {
			return "United Kingdom"
		}
	}

	return strings.TrimSpace(loc)
}

// DisplayLocation returns a clean, UK-only location string, deduplicated and formatted nicely.
func (j *Job) DisplayLocation() string {
	raw := strings.TrimSpace(j.LocationName)
	if raw == "" {
		return "United Kingdom"
	}

	// Split by semicolon first
	var rawChunks []string
	if strings.Contains(raw, ";") {
		for _, chunk := range strings.Split(raw, ";") {
			chunk = strings.TrimSpace(chunk)
			if chunk != "" {
				rawChunks = append(rawChunks, chunk)
			}
		}
	} else {
		rawChunks = []string{raw}
	}

	// Further check each chunk: if a chunk contains ", London, England, United Kingdom" or similar repeated country patterns
	var subChunks []string
	for _, chunk := range rawChunks {
		splitChunks := multiUkSplitRegex.Split(chunk, -1)
		for _, sc := range splitChunks {
			sc = strings.TrimSpace(sc)
			if sc != "" {
				subChunks = append(subChunks, sc)
			}
		}
	}

	var uniqueLocations []string
	seen := make(map[string]bool)

	for _, chunk := range subChunks {
		cleaned := cleanSingleLocation(chunk)
		if cleaned == "" || strings.EqualFold(cleaned, "United Kingdom") || strings.EqualFold(cleaned, "UK") {
			continue
		}

		key := strings.ToLower(cleaned)
		if !seen[key] {
			seen[key] = true
			uniqueLocations = append(uniqueLocations, cleaned)
		}
	}

	if len(uniqueLocations) == 0 {
		return "United Kingdom"
	}

	// Format neatly and ensure it NEVER exceeds card boundary:
	if len(uniqueLocations) == 1 {
		if len(uniqueLocations[0]) > 32 {
			return uniqueLocations[0][:30] + "..."
		}
		return uniqueLocations[0]
	}

	combined := uniqueLocations[0] + ", " + uniqueLocations[1]
	if len(uniqueLocations) == 2 {
		if len(combined) <= 32 {
			return combined
		}
		return fmt.Sprintf("%s (+1)", uniqueLocations[0])
	}

	// 3 or more locations
	if len(combined) <= 24 {
		return fmt.Sprintf("%s (+%d)", combined, len(uniqueLocations)-2)
	}
	return fmt.Sprintf("%s (+%d)", uniqueLocations[0], len(uniqueLocations)-1)
}

// CleanPrimaryLocation extracts a clean, single location name (City, Region, or "Remote") and its URL query param suitable for breadcrumbs and navigation.
func (j *Job) CleanPrimaryLocation() (name string, queryParam string) {
	if j.IsRemote {
		return "Remote", "remote_type=remote"
	}

	loc := j.DisplayLocation()
	locLower := strings.ToLower(strings.TrimSpace(loc))

	if locLower == "" || locLower == "united kingdom" || locLower == "uk" || locLower == "nationwide" || locLower == "national" {
		return "", ""
	}

	if locLower == "remote" || strings.Contains(locLower, "remote") {
		return "Remote", "remote_type=remote"
	}

	// If multiple comma-separated locations, take the primary (first) city
	primary := loc
	if strings.Contains(primary, ",") {
		primary = strings.TrimSpace(strings.Split(primary, ",")[0])
	}
	// Strip any trailing parentheses like "Newport (South Wales)" -> "Newport"
	if idx := strings.Index(primary, "("); idx != -1 {
		primary = strings.TrimSpace(primary[:idx])
	}
	// Strip work style or foreign slashes
	primary = workTypeSuffixRegex.ReplaceAllString(primary, "")
	primary = foreignSlashRegex.ReplaceAllString(primary, "")
	primary = strings.TrimSpace(primary)

	// Guard against sentences or abnormally long strings
	if len(primary) > 30 || strings.Contains(primary, ".") {
		return "", ""
	}

	if strings.EqualFold(primary, "United Kingdom") || strings.EqualFold(primary, "UK") || strings.EqualFold(primary, "England") {
		return "", ""
	}

	return primary, "location=" + url.QueryEscape(primary)
}

var slugRegex = regexp.MustCompile(`[^a-z0-9]+`)

func generateSlug(s string) string {
	slug := strings.ToLower(strings.TrimSpace(s))
	slug = slugRegex.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 80 {
		slug = slug[:80]
	}
	return slug
}

// CompanyLogoURL returns a brand logo image URL for the employer proxied through our clean endpoint.
func (j *Job) CompanyLogoURL() string {
	if j.CompanyLogoURLValue != "" {
		if strings.Contains(j.CompanyLogoURLValue, "/profileimages/logos/thumbs/") {
			parts := strings.Split(j.CompanyLogoURLValue, "/profileimages/logos/thumbs/")
			if len(parts) > 1 {
				filenamePart := parts[1]
				if idx := strings.Index(filenamePart, "?"); idx != -1 {
					filenamePart = filenamePart[:idx]
				}
				if filenamePart != "" && filenamePart != "." {
					return "/job/logo/" + filenamePart
				}
			}
		}
	}

	if j.CompanySlug != "" {
		return "/job/logo/fh-" + j.CompanySlug + ".png"
	}
	if j.EmployerName != "" {
		return "/job/logo/fh-" + generateSlug(j.EmployerName) + ".png"
	}

	return "https://ui-avatars.com/api/?name=Company&background=0A66C2&color=fff&size=128&bold=true&font-size=0.4"
}
// ApplicationsCount returns a simulated number of applications.
func (j *Job) ApplicationsCount() int {
	return int(j.ID%25 + 7)
}

// JobSearchParams holds search/filter criteria.
type JobSearchParams struct {
	Query        string  `json:"q"`
	Location     string  `json:"location"`
	ContractType string  `json:"type"`
	JobType      string  `json:"job_type"`
	MinSalary    float64 `json:"min_salary"`
	MaxSalary    float64 `json:"max_salary"`
	IsGraduate   *bool   `json:"graduate"`
	IsRemote     *bool   `json:"remote"`
	EasyApply    bool    `json:"easy_apply"`
	ExperienceLevel string  `json:"experience"`
	DatePostedRange string  `json:"date_posted"`
	RemoteType      string  `json:"remote_type"`
	RoleSlug     string  `json:"role"`
	IndustrySlug string  `json:"industry"`
	LocationSlug string  `json:"location_slug"`
	CompanySlug  string  `json:"company"`
	SortBy       string  `json:"sort"` // relevance, date, salary_asc, salary_desc
	Page         int     `json:"page"`
	PerPage      int     `json:"per_page"`
}

// JobSearchResult holds paginated search results.
type JobSearchResult struct {
	Jobs       []Job          `json:"jobs"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	PerPage    int            `json:"per_page"`
	TotalPages int            `json:"total_pages"`
	Facets     JobSearchFacets `json:"facets"`
}

// JobSearchFacets holds aggregated filter counts.
type JobSearchFacets struct {
	ContractTypes []FacetItem `json:"contract_types"`
	JobTypes      []FacetItem `json:"job_types"`
	Locations     []FacetItem `json:"locations"`
	Roles         []FacetItem `json:"roles"`
	Industries    []FacetItem `json:"industries"`
	SalaryRanges  []FacetItem `json:"salary_ranges"`
}

// FacetItem represents a single filter option with count.
type FacetItem struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// JobReport represents a user report on a job listing.
type JobReport struct {
	ID        int64     `json:"id" db:"id"`
	JobID     int64     `json:"job_id" db:"job_id"`
	Reason    string    `json:"reason" db:"reason"`
	Details   string    `json:"details" db:"details"`
	IPAddress string    `json:"ip_address" db:"ip_address"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// Joined fields
	JobTitle  string `json:"job_title,omitempty" db:"job_title"`
	JobSlug   string `json:"job_slug,omitempty" db:"job_slug"`
	JobStatus string `json:"job_status,omitempty" db:"job_status"`
}

// BlacklistedIP represents an IP address blocked from submitting reports.
type BlacklistedIP struct {
	IPAddress string    `json:"ip_address" db:"ip_address"`
	Reason    string    `json:"reason" db:"reason"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
