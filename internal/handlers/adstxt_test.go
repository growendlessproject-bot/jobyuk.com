package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"jobyuk/internal/settings"
)

func TestAdsTxtHandler(t *testing.T) {
	initial := settings.GetAdsTxt()
	defer func() {
		_ = settings.UpdateAdsTxt(initial)
	}()

	customContent := "google.com, pub-9876543210987654, DIRECT, f08c47fec0942fa0\n"
	if err := settings.UpdateAdsTxt(customContent); err != nil {
		t.Fatalf("Failed to update ads.txt: %v", err)
	}

	h := &Handlers{}

	req := httptest.NewRequest(http.MethodGet, "/ads.txt", nil)
	rec := httptest.NewRecorder()

	h.AdsTxt(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected HTTP 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Expected Content-Type text/plain, got %s", ct)
	}

	body := rec.Body.String()
	if body != customContent {
		t.Errorf("Expected body %q, got %q", customContent, body)
	}
}

func TestAdminUpdateAdsTxt(t *testing.T) {
	initial := settings.GetAdsTxt()
	defer func() {
		_ = settings.UpdateAdsTxt(initial)
	}()

	adminH := &AdminHandlers{}

	// 1. Test update
	form := url.Values{}
	form.Set("action", "save")
	form.Set("ads_txt", "google.com, pub-9999999999999999, DIRECT, f08c47fec0942fa0")

	req := httptest.NewRequest(http.MethodPost, "/admin-lol/settings/ads-txt", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	adminH.UpdateAdsTxt(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("Expected redirect 303, got %d", rec.Code)
	}

	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "ads_saved=true") {
		t.Errorf("Expected Location to contain ads_saved=true, got %s", loc)
	}

	got := settings.GetAdsTxt()
	if !strings.Contains(got, "pub-9999999999999999") {
		t.Errorf("Expected ads.txt to contain updated pub ID, got %q", got)
	}

	// 2. Test delete / clear
	deleteForm := url.Values{}
	deleteForm.Set("action", "delete")

	reqDel := httptest.NewRequest(http.MethodPost, "/admin-lol/settings/ads-txt", strings.NewReader(deleteForm.Encode()))
	reqDel.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recDel := httptest.NewRecorder()

	adminH.UpdateAdsTxt(recDel, reqDel)

	if recDel.Code != http.StatusSeeOther {
		t.Errorf("Expected redirect 303, got %d", recDel.Code)
	}

	locDel := recDel.Header().Get("Location")
	if !strings.Contains(locDel, "ads_deleted=true") {
		t.Errorf("Expected Location to contain ads_deleted=true, got %s", locDel)
	}

	if strings.TrimSpace(settings.GetAdsTxt()) != "" {
		t.Errorf("Expected ads.txt to be empty after delete, got %q", settings.GetAdsTxt())
	}
}
