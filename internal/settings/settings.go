package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SiteSettings holds all dynamic site configurations.
type SiteSettings struct {
	ContactEmail        string `json:"contact_email"`
	SocialTwitter       string `json:"social_twitter"`
	SocialLinkedIn      string `json:"social_linkedin"`
	SocialFacebook      string `json:"social_facebook"`
	SocialInstagram     string `json:"social_instagram"`
	SocialPinterest     string `json:"social_pinterest"`
	GoogleAnalyticsID   string `json:"google_analytics_id"`
	AdSensePublisherID  string `json:"adsense_publisher_id"`
	AdsTxt              string `json:"ads_txt"`
	DefaultMetaTitle    string `json:"default_meta_title"`
	DefaultMetaDesc     string `json:"default_meta_desc"`
	DefaultMetaKeywords string `json:"default_meta_keywords"`
	DefaultOGImage      string `json:"default_og_image"`
	InsightsTitle       string `json:"insights_title"`
	InsightsSubtitle    string `json:"insights_subtitle"`
	HiringTitle         string `json:"hiring_title"`
	HiringCtaText       string `json:"hiring_cta_text"`
	HiringCtaLink       string `json:"hiring_cta_link"`
	ApplyCtaText        string `json:"apply_cta_text"`
	AntiAdblockEnabled  bool   `json:"anti_adblock_enabled"`
}

var (
	instance *Manager
	once     sync.Once
)

// Manager handles concurrent read and write of site settings.
type Manager struct {
	mu       sync.Mutex
	filePath string
	lastMod  time.Time
	settings SiteSettings
}

// GetManager returns the singleton settings manager instance.
func GetManager() *Manager {
	once.Do(func() {
		dir := "data"
		_ = os.MkdirAll(dir, 0755)
		filePath := filepath.Join(dir, "site_settings.json")

		defaultSettings := SiteSettings{
			ContactEmail:        "contact@jobyuk.com",
			SocialTwitter:       "https://twitter.com/jobyuk",
			SocialLinkedIn:      "https://linkedin.com/company/jobyuk",
			SocialFacebook:      "https://facebook.com/jobyuk",
			SocialInstagram:     "https://instagram.com/jobyuk",
			SocialPinterest:     "https://pinterest.com/jobyuk",
			GoogleAnalyticsID:   "",
			AdSensePublisherID:  "",
			DefaultMetaTitle:    "JobyUK.com - Direct UK Job Search & Career Platform",
			DefaultMetaDesc:     "Find your next job in the UK. Direct vacancy discovery, verified employers, authentic salary insights, and career advice across England, Scotland, Wales, and Northern Ireland.",
			DefaultMetaKeywords: "jobs UK, UK careers, employment, software engineer jobs, nurse vacancies, accountant jobs UK",
			DefaultOGImage:      "/static/images/og-image.jpg",
			InsightsTitle:       "Career Tips & Job Market Insights",
			InsightsSubtitle:    "Practical guides, salary data, and honest advice to help you get ahead in the UK job market.",
			HiringTitle:         "Hiring? Get your vacancy in front of thousands of UK job seekers",
			HiringCtaText:       "Take the first step",
			HiringCtaLink:       "/employers/post-job/",
			ApplyCtaText:        "Apply for this role",
			AntiAdblockEnabled:  true,
		}

		instance = &Manager{
			filePath: filePath,
			settings: defaultSettings,
		}
		instance.mu.Lock()
		instance.loadLocked()
		instance.mu.Unlock()
	})
	return instance
}

func (m *Manager) loadLocked() {
	info, err := os.Stat(m.filePath)
	if err != nil {
		_ = m.saveLocked()
		return
	}
	m.lastMod = info.ModTime()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return
	}

	var s SiteSettings
	if err := json.Unmarshal(data, &s); err == nil {
		if s.ContactEmail != "" {
			m.settings.ContactEmail = s.ContactEmail
		}
		if s.SocialTwitter != "" {
			m.settings.SocialTwitter = s.SocialTwitter
		}
		if s.SocialLinkedIn != "" {
			m.settings.SocialLinkedIn = s.SocialLinkedIn
		}
		if s.SocialFacebook != "" {
			m.settings.SocialFacebook = s.SocialFacebook
		}
		if s.SocialInstagram != "" {
			m.settings.SocialInstagram = s.SocialInstagram
		}
		if s.SocialPinterest != "" {
			m.settings.SocialPinterest = s.SocialPinterest
		}
		m.settings.GoogleAnalyticsID = s.GoogleAnalyticsID
		m.settings.AdSensePublisherID = s.AdSensePublisherID
		m.settings.AdsTxt = s.AdsTxt
		if m.settings.AdsTxt == "" {
			if b, err := os.ReadFile(filepath.Join("data", "ads.txt")); err == nil && len(b) > 0 {
				m.settings.AdsTxt = string(b)
			}
		}
		if s.DefaultMetaTitle != "" {
			m.settings.DefaultMetaTitle = s.DefaultMetaTitle
		}
		if s.DefaultMetaDesc != "" {
			m.settings.DefaultMetaDesc = s.DefaultMetaDesc
		}
		if s.DefaultMetaKeywords != "" {
			m.settings.DefaultMetaKeywords = s.DefaultMetaKeywords
		}
		if s.DefaultOGImage != "" {
			m.settings.DefaultOGImage = s.DefaultOGImage
		}
		if s.InsightsTitle != "" {
			m.settings.InsightsTitle = s.InsightsTitle
		}
		if s.InsightsSubtitle != "" {
			m.settings.InsightsSubtitle = s.InsightsSubtitle
		}
		if s.HiringTitle != "" {
			m.settings.HiringTitle = s.HiringTitle
		}
		if s.HiringCtaText != "" {
			m.settings.HiringCtaText = s.HiringCtaText
		}
		if s.HiringCtaLink != "" {
			m.settings.HiringCtaLink = s.HiringCtaLink
		}
		if s.ApplyCtaText != "" {
			m.settings.ApplyCtaText = s.ApplyCtaText
		}
		var rawMap map[string]interface{}
		if err := json.Unmarshal(data, &rawMap); err == nil {
			if val, exists := rawMap["anti_adblock_enabled"]; exists {
				if b, ok := val.(bool); ok {
					m.settings.AntiAdblockEnabled = b
				}
			}
		}
	}
}

func (m *Manager) saveLocked() error {
	data, err := json.MarshalIndent(m.settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(m.filePath, data, 0644); err != nil {
		return err
	}
	_ = os.WriteFile(filepath.Join("data", "ads.txt"), []byte(m.settings.AdsTxt), 0644)
	if info, err := os.Stat(m.filePath); err == nil {
		m.lastMod = info.ModTime()
	}
	return nil
}

// GetAll returns a copy of the current settings.
func (m *Manager) GetAll() SiteSettings {
	m.mu.Lock()
	defer m.mu.Unlock()

	if info, err := os.Stat(m.filePath); err == nil {
		if info.ModTime().After(m.lastMod) {
			m.loadLocked()
		}
	}

	return m.settings
}

// Update updates the site settings and saves to disk.
func (m *Manager) Update(newSettings SiteSettings) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.settings = newSettings
	return m.saveLocked()
}

// Helpers
func GetContactEmail() string {
	return GetManager().GetAll().ContactEmail
}

func GetSocialTwitter() string {
	return GetManager().GetAll().SocialTwitter
}

func GetSocialLinkedIn() string {
	return GetManager().GetAll().SocialLinkedIn
}

func GetSocialFacebook() string {
	return GetManager().GetAll().SocialFacebook
}

func GetSocialInstagram() string {
	return GetManager().GetAll().SocialInstagram
}

func GetSocialPinterest() string {
	return GetManager().GetAll().SocialPinterest
}

func GetGoogleAnalyticsID() string {
	id := strings.TrimSpace(GetManager().GetAll().GoogleAnalyticsID)
	if id == "" || strings.HasPrefix(strings.ToUpper(id), "G-TEST") || id == "G-TESTUK12345" {
		return ""
	}
	return id
}

func GetAdSensePublisherID() string {
	id := strings.TrimSpace(GetManager().GetAll().AdSensePublisherID)
	if id == "" || strings.Contains(id, "987654321") || strings.Contains(id, "00000000") || strings.Contains(id, "12345678") {
		return ""
	}
	// Normalize ca-pub- prefix
	if !strings.HasPrefix(id, "ca-pub-") && !strings.HasPrefix(id, "pub-") {
		return "ca-pub-" + id
	}
	if strings.HasPrefix(id, "pub-") {
		return "ca-" + id
	}
	return id
}

func GetDefaultMetaTitle() string {
	return GetManager().GetAll().DefaultMetaTitle
}

func GetDefaultMetaDesc() string {
	return GetManager().GetAll().DefaultMetaDesc
}

func GetDefaultMetaKeywords() string {
	return GetManager().GetAll().DefaultMetaKeywords
}

func GetDefaultOGImage() string {
	return GetManager().GetAll().DefaultOGImage
}

func GetInsightsTitle() string {
	return GetManager().GetAll().InsightsTitle
}

func GetInsightsSubtitle() string {
	return GetManager().GetAll().InsightsSubtitle
}

func GetHiringTitle() string {
	return GetManager().GetAll().HiringTitle
}

func GetHiringCtaText() string {
	return GetManager().GetAll().HiringCtaText
}

func GetHiringCtaLink() string {
	return GetManager().GetAll().HiringCtaLink
}

func GetApplyCtaText() string {
	txt := strings.TrimSpace(GetManager().GetAll().ApplyCtaText)
	if txt == "" {
		return "Apply for this role"
	}
	return txt
}

func GetAdsTxt() string {
	return GetManager().GetAll().AdsTxt
}

func UpdateAdsTxt(content string) error {
	mgr := GetManager()
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	mgr.settings.AdsTxt = content
	return mgr.saveLocked()
}

// IsWhatsAppAlertsEnabled returns true if WhatsApp alerts module is enabled via config/env.
func IsWhatsAppAlertsEnabled() bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv("ENABLE_WHATSAPP_ALERTS")))
	return val != "false" && val != "0" && val != "no"
}

// IsAntiAdblockEnabled returns true if Anti-Adblock wall enforcement is active.
func IsAntiAdblockEnabled() bool {
	return GetManager().GetAll().AntiAdblockEnabled
}
