package handlers

import (
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"jobyuk/internal/middleware"
	"jobyuk/internal/settings"
)

// Renderer handles template loading and rendering.
type Renderer struct {
	templates map[string]*template.Template
	mu        sync.RWMutex
	baseDir   string
	funcMap   template.FuncMap
}

// stripTagsHelper strips all HTML tags after unescaping entities
func stripTagsHelper(s string) string {
	s = html.UnescapeString(s)
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			result.WriteRune(' ')
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	out := result.String()
	for strings.Contains(out, "  ") {
		out = strings.ReplaceAll(out, "  ", " ")
	}
	for strings.Contains(out, "\n ") {
		out = strings.ReplaceAll(out, "\n ", "\n")
	}
	return strings.TrimSpace(out)
}

type phoneCountryMatch struct {
	dial string
	flag string
	code string
	name string
}

var phoneDialTable = []phoneCountryMatch{
	{dial: "+971", flag: "🇦🇪", code: "AE", name: "United Arab Emirates"},
	{dial: "+966", flag: "🇸🇦", code: "SA", name: "Saudi Arabia"},
	{dial: "+974", flag: "🇶🇦", code: "QA", name: "Qatar"},
	{dial: "+965", flag: "🇰🇼", code: "KW", name: "Kuwait"},
	{dial: "+968", flag: "🇴🇲", code: "OM", name: "Oman"},
	{dial: "+973", flag: "🇧🇭", code: "BH", name: "Bahrain"},
	{dial: "+880", flag: "🇧🇩", code: "BD", name: "Bangladesh"},
	{dial: "+977", flag: "🇳🇵", code: "NP", name: "Nepal"},
	{dial: "+852", flag: "🇭🇰", code: "HK", name: "Hong Kong"},
	{dial: "+353", flag: "🇮🇪", code: "IE", name: "Ireland"},
	{dial: "+358", flag: "🇫🇮", code: "FI", name: "Finland"},
	{dial: "+351", flag: "🇵🇹", code: "PT", name: "Portugal"},
	{dial: "+234", flag: "🇳🇬", code: "NG", name: "Nigeria"},
	{dial: "+254", flag: "🇰🇪", code: "KE", name: "Kenya"},
	{dial: "+44", flag: "🇬🇧", code: "GB", name: "United Kingdom"},
	{dial: "+92", flag: "🇵🇰", code: "PK", name: "Pakistan"},
	{dial: "+91", flag: "🇮🇳", code: "IN", name: "India"},
	{dial: "+61", flag: "🇦🇺", code: "AU", name: "Australia"},
	{dial: "+49", flag: "🇩🇪", code: "DE", name: "Germany"},
	{dial: "+33", flag: "🇫🇷", code: "FR", name: "France"},
	{dial: "+39", flag: "🇮🇹", code: "IT", name: "Italy"},
	{dial: "+34", flag: "🇪🇸", code: "ES", name: "Spain"},
	{dial: "+31", flag: "🇳🇱", code: "NL", name: "Netherlands"},
	{dial: "+41", flag: "🇨🇭", code: "CH", name: "Switzerland"},
	{dial: "+46", flag: "🇸🇪", code: "SE", name: "Sweden"},
	{dial: "+47", flag: "🇳🇴", code: "NO", name: "Norway"},
	{dial: "+45", flag: "🇩🇰", code: "DK", name: "Denmark"},
	{dial: "+32", flag: "🇧🇪", code: "BE", name: "Belgium"},
	{dial: "+43", flag: "🇦🇹", code: "AT", name: "Austria"},
	{dial: "+48", flag: "🇵🇱", code: "PL", name: "Poland"},
	{dial: "+90", flag: "🇹🇷", code: "TR", name: "Turkey"},
	{dial: "+27", flag: "🇿🇦", code: "ZA", name: "South Africa"},
	{dial: "+20", flag: "🇪🇬", code: "EG", name: "Egypt"},
	{dial: "+60", flag: "🇲🇾", code: "MY", name: "Malaysia"},
	{dial: "+65", flag: "🇸🇬", code: "SG", name: "Singapore"},
	{dial: "+64", flag: "🇳🇿", code: "NZ", name: "New Zealand"},
	{dial: "+94", flag: "🇱🇰", code: "LK", name: "Sri Lanka"},
	{dial: "+63", flag: "🇵🇭", code: "PH", name: "Philippines"},
	{dial: "+62", flag: "🇮🇩", code: "ID", name: "Indonesia"},
	{dial: "+86", flag: "🇨🇳", code: "CN", name: "China"},
	{dial: "+81", flag: "🇯🇵", code: "JP", name: "Japan"},
	{dial: "+82", flag: "🇰🇷", code: "KR", name: "South Korea"},
	{dial: "+1", flag: "🇺🇸", code: "US", name: "United States"},
}

func parsePhoneCountry(phone string) phoneCountryMatch {
	p := strings.TrimSpace(phone)
	if p == "" {
		return phoneCountryMatch{dial: "", flag: "🌐", code: "INT", name: "International"}
	}
	if !strings.HasPrefix(p, "+") {
		p = "+" + p
	}
	for _, m := range phoneDialTable {
		if strings.HasPrefix(p, m.dial) {
			return m
		}
	}
	return phoneCountryMatch{dial: "", flag: "🌐", code: "INT", name: "International"}
}

func formatPhoneWithSpaces(phone string) string {
	p := strings.TrimSpace(phone)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "+") {
		p = "+" + p
	}
	m := parsePhoneCountry(p)
	if m.dial != "" && len(p) > len(m.dial) {
		national := strings.TrimSpace(p[len(m.dial):])
		if len(national) >= 7 {
			return fmt.Sprintf("%s %s %s", m.dial, national[:3], national[3:])
		}
		return fmt.Sprintf("%s %s", m.dial, national)
	}
	return p
}

// NewRenderer creates a new template renderer.
func NewRenderer(baseDir string) (*Renderer, error) {
	r := &Renderer{
		templates: make(map[string]*template.Template),
		baseDir:   baseDir,
		funcMap: template.FuncMap{
			"currentYear": func() int {
				return time.Now().Year()
			},
			"getSiteEmail": func() string {
				return settings.GetContactEmail()
			},
			"getSocialTwitter": func() string {
				return settings.GetSocialTwitter()
			},
			"getSocialLinkedIn": func() string {
				return settings.GetSocialLinkedIn()
			},
			"getSocialFacebook": func() string {
				return settings.GetSocialFacebook()
			},
			"getSocialInstagram": func() string {
				return settings.GetSocialInstagram()
			},
			"getSocialPinterest": func() string {
				return settings.GetSocialPinterest()
			},
			"getGoogleAnalyticsID": func() string {
				return settings.GetGoogleAnalyticsID()
			},
			"getAdSensePublisherID": func() string {
				return settings.GetAdSensePublisherID()
			},
			"getDefaultMetaTitle": func() string {
				return settings.GetDefaultMetaTitle()
			},
			"getDefaultMetaDesc": func() string {
				return settings.GetDefaultMetaDesc()
			},
			"getDefaultMetaKeywords": func() string {
				return settings.GetDefaultMetaKeywords()
			},
			"getDefaultOGImage": func() string {
				return settings.GetDefaultOGImage()
			},
			"getInsightsTitle": func() string {
				return settings.GetInsightsTitle()
			},
			"getInsightsSubtitle": func() string {
				return settings.GetInsightsSubtitle()
			},
			"getHiringTitle": func() string {
				return settings.GetHiringTitle()
			},
			"getHiringCtaText": func() string {
				return settings.GetHiringCtaText()
			},
			"getHiringCtaLink": func() string {
				return settings.GetHiringCtaLink()
			},
			"getApplyCtaText": func() string {
				return settings.GetApplyCtaText()
			},
			"whatsappAlertsEnabled": func() bool {
				return settings.IsWhatsAppAlertsEnabled()
			},
			"antiAdblockEnabled": func() bool {
				return settings.IsAntiAdblockEnabled()
			},
			"asset": func(path string) string {
				diskPath := filepath.Join("web", filepath.Clean(path))
				info, err := os.Stat(diskPath)
				if err != nil {
					return path
				}
				return fmt.Sprintf("%s?v=%d", path, info.ModTime().Unix())
			},
			"safeHTML": func(s string) template.HTML {
				s = html.UnescapeString(s)
				s = strings.TrimSpace(s)

				reLeading := regexp.MustCompile(`(?i)^(\s*<br\s*/?>\s*|</br>|\s*<p>\s*(\s|&nbsp;|\x{00a0})*\s*</p>|\s*<p>\s*<br\s*/?>\s*</p>|\n|\r)+`)
				reTrailing := regexp.MustCompile(`(?i)(\s*<br\s*/?>\s*|</br>|\s*<p>\s*(\s|&nbsp;|\x{00a0})*\s*</p>|\s*<p>\s*<br\s*/?>\s*</p>|\n|\r)+\s*$`)

				for {
					cleaned := reLeading.ReplaceAllString(s, "")
					cleaned = reTrailing.ReplaceAllString(cleaned, "")
					if cleaned == s {
						break
					}
					s = cleaned
				}

				return template.HTML(strings.TrimSpace(s))
			},
			"safeURL": func(s string) template.URL {
				return template.URL(s)
			},
			"lower": strings.ToLower,
			"upper": strings.ToUpper,
			"title": strings.Title,
			"join":  strings.Join,
			"replace": func(s, old, new string) string {
				return strings.ReplaceAll(s, old, new)
			},
			"add": func(a, b int) int {
				return a + b
			},
			"sub": func(a, b int) int {
				return a - b
			},
			"showWhatsAppChannelFAB": func(nav, canonical, title string) bool {
				n := strings.ToLower(strings.TrimSpace(nav))
				c := strings.ToLower(strings.TrimSpace(canonical))
				t := strings.ToLower(strings.TrimSpace(title))

				// Exclude Home page
				if n == "home" || strings.HasSuffix(c, "jobyuk.com/") || strings.HasSuffix(c, "jobyuk.com") || c == "/" {
					return false
				}
				// Exclude Privacy Policy
				if n == "privacy" || strings.Contains(c, "privacy") || strings.Contains(t, "privacy policy") {
					return false
				}
				// Exclude Terms of Service
				if n == "terms" || strings.Contains(c, "terms") || strings.Contains(t, "terms of service") {
					return false
				}
				// Exclude Portals (Admin, WhatsApp Gateway, WhatsApp Client, Pinterest, Employer Post Job)
				if n == "admin" || n == "wa_admin" || n == "wa_portal" || n == "wa_client" || n == "pinterest" || n == "employers" || n == "post_job" {
					return false
				}
				if strings.Contains(c, "wa-portal") || strings.Contains(c, "my-alerts") || strings.Contains(c, "pin-") || strings.Contains(c, "admin") || strings.Contains(c, "post-job") || strings.Contains(c, "employers") {
					return false
				}
				if strings.Contains(t, "gateway station") || strings.Contains(t, "portal") || strings.Contains(t, "pinterest poster") || strings.Contains(t, "my alerts") {
					return false
				}
				return true
			},
			"mul": func(a, b int) int {
				return a * b
			},
			"div": func(a, b int) int {
				if b == 0 {
					return 0
				}
				return a / b
			},
			"seq": func(start, end int) []int {
				var result []int
				for i := start; i <= end; i++ {
					result = append(result, i)
				}
				return result
			},
			"truncate": func(s string, maxLen int) string {
				if len(s) <= maxLen {
					return s
				}
				return s[:maxLen] + "..."
			},
			"stripHTML": func(s string) string {
				return stripTagsHelper(s)
			},
			"truncateText": func(s string, maxLen int) string {
				cleaned := stripTagsHelper(s)
				if len(cleaned) <= maxLen {
					return cleaned
				}
				return cleaned[:maxLen] + "..."
			},
			"cleanLocation": func(s string) string {
				s = strings.TrimSpace(s)
				if s == "" {
					return "United Kingdom"
				}
				if idx := strings.Index(s, ";"); idx != -1 {
					s = strings.TrimSpace(s[:idx])
				}
				parts := strings.Split(s, ",")
				if len(parts) > 2 {
					city := strings.TrimSpace(parts[0])
					country := strings.TrimSpace(parts[len(parts)-1])
					if strings.EqualFold(country, "United Kingdom") {
						return city + ", UK"
					}
					return city + ", " + country
				}
				return s
			},
			"formatNumber": func(n int) string {
				if n >= 1000000 {
					return fmt.Sprintf("%.1fM", float64(n)/1000000)
				}
				if n >= 1000 {
					return fmt.Sprintf("%.0fk", float64(n)/1000)
				}
				return fmt.Sprintf("%d", n)
			},
			"formatDate": func(t interface{}) string {
				if t == nil {
					return "N/A"
				}
				switch v := t.(type) {
				case time.Time:
					if v.IsZero() {
						return "N/A"
					}
					return v.Format("02 Jan 2006, 15:04")
				case *time.Time:
					if v == nil || v.IsZero() {
						return "N/A"
					}
					return v.Format("02 Jan 2006, 15:04")
				default:
					return "N/A"
				}
			},
			"formatDateOnly": func(t interface{}) string {
				if t == nil {
					return "N/A"
				}
				switch v := t.(type) {
				case time.Time:
					if v.IsZero() {
						return "N/A"
					}
					return v.Format("02 Jan 2006")
				case *time.Time:
					if v == nil || v.IsZero() {
						return "N/A"
					}
					return v.Format("02 Jan 2006")
				default:
					return "N/A"
				}
			},
			"float64": func(v interface{}) float64 {
				switch val := v.(type) {
				case float64:
					return val
				case *float64:
					if val == nil {
						return 0
					}
					return *val
				case int:
					return float64(val)
				case int64:
					return float64(val)
				default:
					return 0
				}
			},
			"formatSalary": func(v interface{}) string {
				var n float64
				switch val := v.(type) {
				case float64:
					n = val
				case *float64:
					if val == nil {
						return ""
					}
					n = *val
				case int:
					n = float64(val)
				case int64:
					n = float64(val)
				default:
					return ""
				}
				i := int(n)
				if i >= 1000 {
					thousands := i / 1000
					remainder := i % 1000
					if remainder == 0 {
						return fmt.Sprintf("%d,000", thousands)
					}
					return fmt.Sprintf("%d,%03d", thousands, remainder)
				}
				return fmt.Sprintf("%d", i)
			},
			"calcMonthly": func(n float64) string {
				m := int(n / 12.0)
				if m >= 1000 {
					return fmt.Sprintf("%d,%03d", m/1000, m%1000)
				}
				return fmt.Sprintf("%d", m)
			},
			"calcHourly": func(n float64) string {
				if n <= 0 {
					return "0.00"
				}
				return fmt.Sprintf("%.2f", n/(52.0*37.5))
			},
			"calcDaily": func(n float64) string {
				d := int(n / (52.0 * 5.0))
				return fmt.Sprintf("%d", d)
			},
			"dict": func(values ...interface{}) (map[string]interface{}, error) {
				if len(values)%2 != 0 {
					return nil, fmt.Errorf("invalid dict call: odd number of arguments")
				}
				dict := make(map[string]interface{}, len(values)/2)
				for i := 0; i < len(values); i += 2 {
					key, ok := values[i].(string)
					if !ok {
						return nil, fmt.Errorf("dict keys must be strings")
					}
					dict[key] = values[i+1]
				}
				return dict, nil
			},
			"phoneFlag": func(phone string) string {
				return parsePhoneCountry(phone).flag
			},
			"phoneCountryCode": func(phone string) string {
				return parsePhoneCountry(phone).code
			},
			"phoneDialCode": func(phone string) string {
				return parsePhoneCountry(phone).dial
			},
			"phoneCountryName": func(phone string) string {
				return parsePhoneCountry(phone).name
			},
			"formatPhoneDisplay": func(phone string) string {
				return formatPhoneWithSpaces(phone)
			},
			"formatPhoneWithFlag": func(phone string) template.HTML {
				p := strings.TrimSpace(phone)
				if p == "" {
					return ""
				}
				match := parsePhoneCountry(p)
				formatted := formatPhoneWithSpaces(p)
				return template.HTML(fmt.Sprintf(`<span class="wa-phone-with-flag" title="%s (%s)"><span class="wa-country-flag">%s</span> <span class="wa-phone-number">%s</span></span>`, match.name, match.dial, match.flag, formatted))
			},
		},
	}

	if err := r.loadTemplates(); err != nil {
		return nil, err
	}

	return r, nil
}

// loadTemplates parses all template files.
func (r *Renderer) loadTemplates() error {
	// Load layout files
	layouts, err := filepath.Glob(filepath.Join(r.baseDir, "layouts", "*.html"))
	if err != nil {
		return fmt.Errorf("failed to find layout templates: %w", err)
	}

	// Load partial files
	partials, err := filepath.Glob(filepath.Join(r.baseDir, "partials", "*.html"))
	if err != nil {
		return fmt.Errorf("failed to find partial templates: %w", err)
	}

	// Combine layouts and partials as shared templates
	shared := append(layouts, partials...)

	// Load page templates
	err = filepath.WalkDir(filepath.Join(r.baseDir, "pages"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}

		// Template name is the relative path from pages/
		relPath, _ := filepath.Rel(filepath.Join(r.baseDir, "pages"), path)
		name := strings.TrimSuffix(relPath, ".html")
		name = strings.ReplaceAll(name, "\\", "/") // Normalize for Windows

		// Parse this page template with shared templates
		files := append([]string{path}, shared...)
		tmpl, err := template.New(filepath.Base(path)).Funcs(r.funcMap).ParseFiles(files...)
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", name, err)
		}

		r.templates[name] = tmpl
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to load page templates: %w", err)
	}

	// Load admin templates
	err = filepath.WalkDir(filepath.Join(r.baseDir, "admin"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}

		relPath, _ := filepath.Rel(filepath.Join(r.baseDir, "admin"), path)
		name := "admin/" + strings.TrimSuffix(relPath, ".html")
		name = strings.ReplaceAll(name, "\\", "/")

		adminLayout := filepath.Join(r.baseDir, "layouts", "admin.html")
		files := append([]string{path, adminLayout}, partials...)
		tmpl, err := template.New(filepath.Base(path)).Funcs(r.funcMap).ParseFiles(files...)
		if err != nil {
			return fmt.Errorf("failed to parse admin template %s: %w", name, err)
		}

		r.templates[name] = tmpl
		return nil
	})

	return err
}

// Render renders a template with data.
func (r *Renderer) Render(w http.ResponseWriter, name string, data interface{}) error {
	return r.RenderWithStatus(w, name, http.StatusOK, data)
}

// RenderWithStatus renders a template with a specific HTTP status code.
func (r *Renderer) RenderWithStatus(w http.ResponseWriter, name string, status int, data interface{}) error {
	if os.Getenv("ENV") != "production" && os.Getenv("APP_ENV") != "production" {
		r.mu.Lock()
		_ = r.loadTemplates()
		r.mu.Unlock()
	}

	r.mu.RLock()
	templateToUse := name
	if pageData, ok := data.(PageData); ok {
		if pageData.IsMobile {
			if _, exists := r.templates[name+"_mobile"]; exists {
				templateToUse = name + "_mobile"
			}
		} else {
			if _, exists := r.templates[name+"_desktop"]; exists {
				templateToUse = name + "_desktop"
			}
		}
	} else if pageDataPtr, ok := data.(*PageData); ok && pageDataPtr != nil {
		if pageDataPtr.IsMobile {
			if _, exists := r.templates[name+"_mobile"]; exists {
				templateToUse = name + "_mobile"
			}
		} else {
			if _, exists := r.templates[name+"_desktop"]; exists {
				templateToUse = name + "_desktop"
			}
		}
	}

	tmpl, ok := r.templates[templateToUse]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("template not found: %s", templateToUse)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	return tmpl.ExecuteTemplate(w, "base", data)
}

// RenderAdmin renders an admin template.
func (r *Renderer) RenderAdmin(w http.ResponseWriter, req *http.Request, name string, data PageData) error {
	if os.Getenv("ENV") != "production" && os.Getenv("APP_ENV") != "production" {
		r.mu.Lock()
		_ = r.loadTemplates()
		r.mu.Unlock()
	}

	r.mu.RLock()
	tmpl, ok := r.templates[name]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("admin template not found: %s", name)
	}

	// Inject CSRF token
	data.CSRFToken = middleware.GetCSRFToken(req)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.ExecuteTemplate(w, "admin", data)
}

// PageData is the base data structure passed to all page templates.
type PageData struct {
	// SEO
	Title       string
	Description string
	Keywords    string
	Canonical   string
	OGTitle     string
	OGDesc      string
	OGImage     string
	OGType      string
	Robots      string
	Schema      template.HTML
	PrevURL     string // rel="prev" for paginated pages
	NextURL     string // rel="next" for paginated pages

	// Breadcrumbs
	Breadcrumbs []BreadcrumbData

	// Navigation
	ActiveNav string

	// Security
	CSRFToken string

	// Content (page-specific)
	Data interface{}

	// Device Detection
	IsMobile bool
}

// BreadcrumbData holds breadcrumb navigation data.
type BreadcrumbData struct {
	Name string
	URL  string
}
