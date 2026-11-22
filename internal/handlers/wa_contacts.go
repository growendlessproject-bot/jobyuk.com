package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"jobyuk/internal/repository"
)

// ContactRequest payload for saving a WhatsApp contact.
type ContactRequest struct {
	Phone    string `json:"phone"`
	FullName string `json:"full_name"`
	Notes    string `json:"notes"`
	Source   string `json:"source"`
}

// SaveContactAPI allows saving a contact into whatsmeow's contact store and DB.
// Endpoint: POST /api/wa/contacts
func (h *WhatsAppClientHandlers) SaveContactAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ContactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Invalid JSON body"})
		return
	}

	normPhone := repository.NormalizePhoneNumber(req.Phone)
	if normPhone == "" || len(normPhone) < 8 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Valid phone number required"})
		return
	}

	fullName := strings.TrimSpace(req.FullName)
	if fullName == "" {
		suffix := normPhone
		if len(normPhone) > 4 {
			suffix = normPhone[len(normPhone)-4:]
		}
		fullName = fmt.Sprintf("JobyUK.com Candidate %s", suffix)
	}

	// Save to whatsmeow internal contacts
	err := h.waSvc.SaveContact(r.Context(), normPhone, fullName)
	if err != nil {
		slog.Warn("Failed to pre-register contact in whatsmeow", "phone", normPhone, "err", err)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"phone":        normPhone,
		"contact_name": fullName,
		"message":      "Contact successfully pre-registered in WhatsApp session.",
	})
}

// ListContactsAPI returns registered contacts for automated integration.
// Endpoint: GET /api/wa/contacts
func (h *WhatsAppClientHandlers) ListContactsAPI(w http.ResponseWriter, r *http.Request) {
	subs, _, err := h.repos.WhatsApp.ListSubscribers(r.Context(), "", "", "", 1000, 0)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	type ContactItem struct {
		ID        int       `json:"id"`
		Phone     string    `json:"phone"`
		FullName  string    `json:"full_name"`
		Status    string    `json:"status"`
		Plan      string    `json:"plan"`
		CreatedAt time.Time `json:"created_at"`
	}

	items := make([]ContactItem, 0, len(subs))
	for _, s := range subs {
		items = append(items, ContactItem{
			ID:        s.ID,
			Phone:     s.PhoneNumber,
			FullName:  s.FullName,
			Status:    s.Status,
			Plan:      s.PlanType,
			CreatedAt: s.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"total":    len(items),
		"contacts": items,
	})
}

// ExportContactsVCF generates a downloadable .vcf vCard address book of all subscribers.
// When opened on an iPhone or Android, all numbers import into the phone's native contacts in 1 click.
// Endpoint: GET /api/wa/contacts/export.vcf
func (h *WhatsAppClientHandlers) ExportContactsVCF(w http.ResponseWriter, r *http.Request) {
	subs, _, err := h.repos.WhatsApp.ListSubscribers(r.Context(), "", "", "", 10000, 0)
	if err != nil {
		http.Error(w, "Failed to retrieve contacts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var sb strings.Builder
	for _, sub := range subs {
		phone := repository.NormalizePhoneNumber(sub.PhoneNumber)
		if phone == "" {
			continue
		}

		name := strings.TrimSpace(sub.FullName)
		if name == "" {
			suffix := phone
			if len(phone) > 4 {
				suffix = phone[len(phone)-4:]
			}
			name = fmt.Sprintf("JobyUK.com Candidate %s", suffix)
		}

		firstName := name
		lastName := "JobyUK.com"
		if parts := strings.SplitN(name, " ", 2); len(parts) == 2 {
			firstName = parts[0]
			lastName = parts[1]
		}

		sb.WriteString("BEGIN:VCARD\r\n")
		sb.WriteString("VERSION:3.0\r\n")
		sb.WriteString(fmt.Sprintf("FN:%s (JobyUK.com)\r\n", name))
		sb.WriteString(fmt.Sprintf("N:%s;%s;;;\r\n", lastName, firstName))
		sb.WriteString(fmt.Sprintf("TEL;TYPE=CELL,VOICE:%s\r\n", phone))
		sb.WriteString("ORG:JobyUK.com Candidates\r\n")
		sb.WriteString(fmt.Sprintf("NOTE:JobyUK.com Job Alerts Candidate | Plan: %s | Status: %s\r\n", sub.PlanType, sub.Status))
		sb.WriteString("END:VCARD\r\n")
	}

	w.Header().Set("Content-Type", "text/vcard; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"JobyUK.com_whatsapp_contacts.vcf\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(sb.String()))
}

// ExportContactsVCF allows WhatsApp Admin to export vCard from the wa-portal.
// Endpoint: GET /wa-portal/contacts/export.vcf
func (h *WhatsAppAdminHandlers) ExportContactsVCF(w http.ResponseWriter, r *http.Request) {
	subs, _, err := h.repos.WhatsApp.ListSubscribers(r.Context(), "", "", "", 10000, 0)
	if err != nil {
		http.Error(w, "Failed to retrieve contacts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var sb strings.Builder
	for _, sub := range subs {
		phone := repository.NormalizePhoneNumber(sub.PhoneNumber)
		if phone == "" {
			continue
		}

		name := strings.TrimSpace(sub.FullName)
		if name == "" {
			suffix := phone
			if len(phone) > 4 {
				suffix = phone[len(phone)-4:]
			}
			name = fmt.Sprintf("JobyUK.com Candidate %s", suffix)
		}

		firstName := name
		lastName := "JobyUK.com"
		if parts := strings.SplitN(name, " ", 2); len(parts) == 2 {
			firstName = parts[0]
			lastName = parts[1]
		}

		sb.WriteString("BEGIN:VCARD\r\n")
		sb.WriteString("VERSION:3.0\r\n")
		sb.WriteString(fmt.Sprintf("FN:%s (JobyUK.com)\r\n", name))
		sb.WriteString(fmt.Sprintf("N:%s;%s;;;\r\n", lastName, firstName))
		sb.WriteString(fmt.Sprintf("TEL;TYPE=CELL,VOICE:%s\r\n", phone))
		sb.WriteString("ORG:JobyUK.com Candidates\r\n")
		sb.WriteString(fmt.Sprintf("NOTE:JobyUK.com Candidate | Plan: %s | Status: %s\r\n", sub.PlanType, sub.Status))
		sb.WriteString("END:VCARD\r\n")
	}

	w.Header().Set("Content-Type", "text/vcard; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"JobyUK.com_whatsapp_contacts.vcf\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(sb.String()))
}

// GoogleConnect initiates OAuth 2.0 flow to link admin's Google Account.
// Endpoint: GET /wa-portal/google-connect
func (h *WhatsAppAdminHandlers) GoogleConnect(w http.ResponseWriter, r *http.Request) {
	if h.googleContacts == nil {
		http.Error(w, "Google Contacts service not configured", http.StatusInternalServerError)
		return
	}
	authURL := h.googleContacts.GetAuthURL()
	http.Redirect(w, r, authURL, http.StatusSeeOther)
}

// GoogleCallback handles the OAuth 2.0 redirect from Google.
// Endpoint: GET /wa-portal/google-callback
func (h *WhatsAppAdminHandlers) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		errParam := r.URL.Query().Get("error")
		http.Redirect(w, r, "/wa-portal/subscribers?google_error="+url.QueryEscape(errParam), http.StatusSeeOther)
		return
	}

	if h.googleContacts != nil {
		if err := h.googleContacts.ExchangeCode(r.Context(), code); err != nil {
			slog.Error("Google Contacts Code Exchange failed", "err", err)
			http.Redirect(w, r, "/wa-portal/subscribers?google_error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
			return
		}
	}

	http.Redirect(w, r, "/wa-portal/subscribers?google_success=true", http.StatusSeeOther)
}

// GoogleDisconnect removes Google Contacts linkage.
// Endpoint: POST /wa-portal/google-disconnect
func (h *WhatsAppAdminHandlers) GoogleDisconnect(w http.ResponseWriter, r *http.Request) {
	if h.googleContacts != nil {
		_ = h.googleContacts.Disconnect()
	}
	http.Redirect(w, r, "/wa-portal/subscribers?google_disconnected=true", http.StatusSeeOther)
}
