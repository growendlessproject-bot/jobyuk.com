package whatsapp

import (
	"fmt"
	"strings"

	"github.com/nyaruka/phonenumbers"
)

// Country represents a sovereign state or territory with international dial code and emoji flag.
type Country struct {
	ISO      string `json:"iso"`
	Name     string `json:"name"`
	DialCode string `json:"dial_code"`
	Flag     string `json:"flag"`
	Priority bool   `json:"priority"`
}

// PriorityCountries returns the top-tier priority countries with UK first.
var PriorityCountries = []Country{
	{ISO: "GB", Name: "United Kingdom", DialCode: "+44", Flag: "🇬🇧", Priority: true},
	{ISO: "US", Name: "United States", DialCode: "+1", Flag: "🇺🇸", Priority: true},
	{ISO: "CA", Name: "Canada", DialCode: "+1", Flag: "🇨🇦", Priority: true},
	{ISO: "AU", Name: "Australia", DialCode: "+61", Flag: "🇦🇺", Priority: true},
	{ISO: "DE", Name: "Germany", DialCode: "+49", Flag: "🇩🇪", Priority: true},
	{ISO: "FR", Name: "France", DialCode: "+33", Flag: "🇫🇷", Priority: true},
	{ISO: "NL", Name: "Netherlands", DialCode: "+31", Flag: "🇳🇱", Priority: true},
	{ISO: "IE", Name: "Ireland", DialCode: "+353", Flag: "🇮🇪", Priority: true},
	{ISO: "CH", Name: "Switzerland", DialCode: "+41", Flag: "🇨🇭", Priority: true},
	{ISO: "SE", Name: "Sweden", DialCode: "+46", Flag: "🇸🇪", Priority: true},
	{ISO: "NZ", Name: "New Zealand", DialCode: "+64", Flag: "🇳🇿", Priority: true},
	{ISO: "AE", Name: "United Arab Emirates", DialCode: "+971", Flag: "🇦🇪", Priority: true},
	{ISO: "SA", Name: "Saudi Arabia", DialCode: "+966", Flag: "🇸🇦", Priority: true},
	{ISO: "QA", Name: "Qatar", DialCode: "+974", Flag: "🇶🇦", Priority: true},
	{ISO: "SG", Name: "Singapore", DialCode: "+65", Flag: "🇸🇬", Priority: true},
	{ISO: "PK", Name: "Pakistan", DialCode: "+92", Flag: "🇵🇰", Priority: true},
	{ISO: "IN", Name: "India", DialCode: "+91", Flag: "🇮🇳", Priority: true},
}

// AllCountries contains the comprehensive world list alphabetically.
var AllCountries = []Country{
	{ISO: "AF", Name: "Afghanistan", DialCode: "+93", Flag: "🇦🇫"},
	{ISO: "AL", Name: "Albania", DialCode: "+355", Flag: "🇦🇱"},
	{ISO: "DZ", Name: "Algeria", DialCode: "+213", Flag: "🇩🇿"},
	{ISO: "AD", Name: "Andorra", DialCode: "+376", Flag: "🇦🇩"},
	{ISO: "AO", Name: "Angola", DialCode: "+244", Flag: "🇦🇴"},
	{ISO: "AG", Name: "Antigua and Barbuda", DialCode: "+1268", Flag: "🇦🇬"},
	{ISO: "AR", Name: "Argentina", DialCode: "+54", Flag: "🇦🇷"},
	{ISO: "AM", Name: "Armenia", DialCode: "+374", Flag: "🇦🇲"},
	{ISO: "AU", Name: "Australia", DialCode: "+61", Flag: "🇦🇺"},
	{ISO: "AT", Name: "Austria", DialCode: "+43", Flag: "🇦🇹"},
	{ISO: "AZ", Name: "Azerbaijan", DialCode: "+994", Flag: "🇦🇿"},
	{ISO: "BS", Name: "Bahamas", DialCode: "+1242", Flag: "🇧🇸"},
	{ISO: "BH", Name: "Bahrain", DialCode: "+973", Flag: "🇧🇭"},
	{ISO: "BD", Name: "Bangladesh", DialCode: "+880", Flag: "🇧🇩"},
	{ISO: "BB", Name: "Barbados", DialCode: "+1246", Flag: "🇧🇧"},
	{ISO: "BY", Name: "Belarus", DialCode: "+375", Flag: "🇧🇾"},
	{ISO: "BE", Name: "Belgium", DialCode: "+32", Flag: "🇧🇪"},
	{ISO: "BZ", Name: "Belize", DialCode: "+501", Flag: "🇧🇿"},
	{ISO: "BJ", Name: "Benin", DialCode: "+229", Flag: "🇧🇯"},
	{ISO: "BT", Name: "Bhutan", DialCode: "+975", Flag: "🇧🇹"},
	{ISO: "BO", Name: "Bolivia", DialCode: "+591", Flag: "🇧🇴"},
	{ISO: "BA", Name: "Bosnia and Herzegovina", DialCode: "+387", Flag: "🇧🇦"},
	{ISO: "BW", Name: "Botswana", DialCode: "+267", Flag: "🇧🇼"},
	{ISO: "BR", Name: "Brazil", DialCode: "+55", Flag: "🇧🇷"},
	{ISO: "BN", Name: "Brunei", DialCode: "+673", Flag: "🇧🇳"},
	{ISO: "BG", Name: "Bulgaria", DialCode: "+359", Flag: "🇧🇬"},
	{ISO: "BF", Name: "Burkina Faso", DialCode: "+226", Flag: "🇧🇫"},
	{ISO: "BI", Name: "Burundi", DialCode: "+257", Flag: "🇧🇮"},
	{ISO: "KH", Name: "Cambodia", DialCode: "+855", Flag: "🇰🇭"},
	{ISO: "CM", Name: "Cameroon", DialCode: "+237", Flag: "🇨🇲"},
	{ISO: "CA", Name: "Canada", DialCode: "+1", Flag: "🇨🇦"},
	{ISO: "CV", Name: "Cape Verde", DialCode: "+238", Flag: "🇨🇻"},
	{ISO: "CF", Name: "Central African Republic", DialCode: "+236", Flag: "🇨🇫"},
	{ISO: "TD", Name: "Chad", DialCode: "+235", Flag: "🇹🇩"},
	{ISO: "CL", Name: "Chile", DialCode: "+56", Flag: "🇨🇱"},
	{ISO: "CN", Name: "China", DialCode: "+86", Flag: "🇨🇳"},
	{ISO: "CO", Name: "Colombia", DialCode: "+57", Flag: "🇨🇴"},
	{ISO: "KM", Name: "Comoros", DialCode: "+269", Flag: "🇰🇲"},
	{ISO: "CG", Name: "Congo", DialCode: "+242", Flag: "🇨🇬"},
	{ISO: "CR", Name: "Costa Rica", DialCode: "+506", Flag: "🇨🇷"},
	{ISO: "HR", Name: "Croatia", DialCode: "+385", Flag: "🇭🇷"},
	{ISO: "CU", Name: "Cuba", DialCode: "+53", Flag: "🇨🇺"},
	{ISO: "CY", Name: "Cyprus", DialCode: "+357", Flag: "🇨🇾"},
	{ISO: "CZ", Name: "Czech Republic", DialCode: "+420", Flag: "🇨🇿"},
	{ISO: "DK", Name: "Denmark", DialCode: "+45", Flag: "🇩🇰"},
	{ISO: "DJ", Name: "Djibouti", DialCode: "+253", Flag: "🇩🇯"},
	{ISO: "DM", Name: "Dominica", DialCode: "+1767", Flag: "🇩🇲"},
	{ISO: "DO", Name: "Dominican Republic", DialCode: "+1809", Flag: "🇩🇴"},
	{ISO: "EC", Name: "Ecuador", DialCode: "+593", Flag: "🇪🇨"},
	{ISO: "EG", Name: "Egypt", DialCode: "+20", Flag: "🇪🇬"},
	{ISO: "SV", Name: "El Salvador", DialCode: "+503", Flag: "🇸🇻"},
	{ISO: "GQ", Name: "Equatorial Guinea", DialCode: "+240", Flag: "🇬🇶"},
	{ISO: "ER", Name: "Eritrea", DialCode: "+291", Flag: "🇪🇷"},
	{ISO: "EE", Name: "Estonia", DialCode: "+372", Flag: "🇪🇪"},
	{ISO: "SZ", Name: "Eswatini", DialCode: "+268", Flag: "🇸🇿"},
	{ISO: "ET", Name: "Ethiopia", DialCode: "+251", Flag: "🇪🇹"},
	{ISO: "FJ", Name: "Fiji", DialCode: "+679", Flag: "🇫🇯"},
	{ISO: "FI", Name: "Finland", DialCode: "+358", Flag: "🇫🇮"},
	{ISO: "FR", Name: "France", DialCode: "+33", Flag: "🇫🇷"},
	{ISO: "GA", Name: "Gabon", DialCode: "+241", Flag: "🇬🇦"},
	{ISO: "GM", Name: "Gambia", DialCode: "+220", Flag: "🇬🇲"},
	{ISO: "GE", Name: "Georgia", DialCode: "+995", Flag: "🇬🇪"},
	{ISO: "DE", Name: "Germany", DialCode: "+49", Flag: "🇩🇪"},
	{ISO: "GH", Name: "Ghana", DialCode: "+233", Flag: "🇬🇭"},
	{ISO: "GR", Name: "Greece", DialCode: "+30", Flag: "🇬🇷"},
	{ISO: "GD", Name: "Grenada", DialCode: "+1473", Flag: "🇬🇩"},
	{ISO: "GT", Name: "Guatemala", DialCode: "+502", Flag: "🇬🇹"},
	{ISO: "GN", Name: "Guinea", DialCode: "+224", Flag: "🇬🇳"},
	{ISO: "GW", Name: "Guinea-Bissau", DialCode: "+245", Flag: "🇬🇼"},
	{ISO: "GY", Name: "Guyana", DialCode: "+592", Flag: "🇬🇾"},
	{ISO: "HT", Name: "Haiti", DialCode: "+509", Flag: "🇭🇹"},
	{ISO: "HN", Name: "Honduras", DialCode: "+504", Flag: "🇭🇳"},
	{ISO: "HK", Name: "Hong Kong", DialCode: "+852", Flag: "🇭🇰"},
	{ISO: "HU", Name: "Hungary", DialCode: "+36", Flag: "🇭🇺"},
	{ISO: "IS", Name: "Iceland", DialCode: "+354", Flag: "🇮🇸"},
	{ISO: "IN", Name: "India", DialCode: "+91", Flag: "🇮🇳"},
	{ISO: "ID", Name: "Indonesia", DialCode: "+62", Flag: "🇮🇩"},
	{ISO: "IR", Name: "Iran", DialCode: "+98", Flag: "🇮🇷"},
	{ISO: "IQ", Name: "Iraq", DialCode: "+964", Flag: "🇮🇶"},
	{ISO: "IE", Name: "Ireland", DialCode: "+353", Flag: "🇮🇪"},
	{ISO: "IL", Name: "Israel", DialCode: "+972", Flag: "🇮🇱"},
	{ISO: "IT", Name: "Italy", DialCode: "+39", Flag: "🇮🇹"},
	{ISO: "CI", Name: "Ivory Coast", DialCode: "+225", Flag: "🇨🇮"},
	{ISO: "JM", Name: "Jamaica", DialCode: "+1876", Flag: "🇯🇲"},
	{ISO: "JP", Name: "Japan", DialCode: "+81", Flag: "🇯🇵"},
	{ISO: "JO", Name: "Jordan", DialCode: "+962", Flag: "🇯🇴"},
	{ISO: "KZ", Name: "Kazakhstan", DialCode: "+7", Flag: "🇰🇿"},
	{ISO: "KE", Name: "Kenya", DialCode: "+254", Flag: "🇰🇪"},
	{ISO: "KW", Name: "Kuwait", DialCode: "+965", Flag: "🇰🇼"},
	{ISO: "KG", Name: "Kyrgyzstan", DialCode: "+996", Flag: "🇰🇬"},
	{ISO: "LA", Name: "Laos", DialCode: "+856", Flag: "🇱🇦"},
	{ISO: "LV", Name: "Latvia", DialCode: "+371", Flag: "🇱🇻"},
	{ISO: "LB", Name: "Lebanon", DialCode: "+961", Flag: "🇱🇧"},
	{ISO: "LS", Name: "Lesotho", DialCode: "+266", Flag: "🇱🇸"},
	{ISO: "LR", Name: "Liberia", DialCode: "+231", Flag: "🇱🇷"},
	{ISO: "LY", Name: "Libya", DialCode: "+218", Flag: "🇱🇾"},
	{ISO: "LI", Name: "Liechtenstein", DialCode: "+423", Flag: "🇱🇮"},
	{ISO: "LT", Name: "Lithuania", DialCode: "+370", Flag: "🇱🇹"},
	{ISO: "LU", Name: "Luxembourg", DialCode: "+352", Flag: "🇱🇺"},
	{ISO: "MO", Name: "Macau", DialCode: "+853", Flag: "🇲🇴"},
	{ISO: "MG", Name: "Madagascar", DialCode: "+261", Flag: "🇲🇬"},
	{ISO: "MW", Name: "Malawi", DialCode: "+265", Flag: "🇲🇼"},
	{ISO: "MY", Name: "Malaysia", DialCode: "+60", Flag: "🇲🇾"},
	{ISO: "MV", Name: "Maldives", DialCode: "+960", Flag: "🇲🇻"},
	{ISO: "ML", Name: "Mali", DialCode: "+223", Flag: "🇲🇱"},
	{ISO: "MT", Name: "Malta", DialCode: "+356", Flag: "🇲🇹"},
	{ISO: "MR", Name: "Mauritania", DialCode: "+222", Flag: "🇲🇷"},
	{ISO: "MU", Name: "Mauritius", DialCode: "+230", Flag: "🇲🇺"},
	{ISO: "MX", Name: "Mexico", DialCode: "+52", Flag: "🇲🇽"},
	{ISO: "MD", Name: "Moldova", DialCode: "+373", Flag: "🇲🇩"},
	{ISO: "MC", Name: "Monaco", DialCode: "+377", Flag: "🇲🇨"},
	{ISO: "MN", Name: "Mongolia", DialCode: "+976", Flag: "🇲🇳"},
	{ISO: "ME", Name: "Montenegro", DialCode: "+382", Flag: "🇲🇪"},
	{ISO: "MA", Name: "Morocco", DialCode: "+212", Flag: "🇲🇦"},
	{ISO: "MZ", Name: "Mozambique", DialCode: "+258", Flag: "🇲🇿"},
	{ISO: "MM", Name: "Myanmar", DialCode: "+95", Flag: "🇲🇲"},
	{ISO: "NA", Name: "Namibia", DialCode: "+264", Flag: "🇳🇦"},
	{ISO: "NP", Name: "Nepal", DialCode: "+977", Flag: "🇳🇵"},
	{ISO: "NL", Name: "Netherlands", DialCode: "+31", Flag: "🇳🇱"},
	{ISO: "NZ", Name: "New Zealand", DialCode: "+64", Flag: "🇳🇿"},
	{ISO: "NI", Name: "Nicaragua", DialCode: "+505", Flag: "🇳🇮"},
	{ISO: "NE", Name: "Niger", DialCode: "+227", Flag: "🇳🇪"},
	{ISO: "NG", Name: "Nigeria", DialCode: "+234", Flag: "🇳🇬"},
	{ISO: "NO", Name: "Norway", DialCode: "+47", Flag: "🇳🇴"},
	{ISO: "OM", Name: "Oman", DialCode: "+968", Flag: "🇴🇲"},
	{ISO: "PK", Name: "Pakistan", DialCode: "+92", Flag: "🇵🇰"},
	{ISO: "PS", Name: "Palestine", DialCode: "+970", Flag: "🇵🇸"},
	{ISO: "PA", Name: "Panama", DialCode: "+507", Flag: "🇵🇦"},
	{ISO: "PG", Name: "Papua New Guinea", DialCode: "+675", Flag: "🇵🇬"},
	{ISO: "PY", Name: "Paraguay", DialCode: "+595", Flag: "🇵🇾"},
	{ISO: "PE", Name: "Peru", DialCode: "+51", Flag: "🇵🇪"},
	{ISO: "PH", Name: "Philippines", DialCode: "+63", Flag: "🇵🇭"},
	{ISO: "PL", Name: "Poland", DialCode: "+48", Flag: "🇵🇱"},
	{ISO: "PT", Name: "Portugal", DialCode: "+351", Flag: "🇵🇹"},
	{ISO: "QA", Name: "Qatar", DialCode: "+974", Flag: "🇶🇦"},
	{ISO: "RO", Name: "Romania", DialCode: "+40", Flag: "🇷🇴"},
	{ISO: "RU", Name: "Russia", DialCode: "+7", Flag: "🇷🇺"},
	{ISO: "RW", Name: "Rwanda", DialCode: "+250", Flag: "🇷🇼"},
	{ISO: "KN", Name: "Saint Kitts and Nevis", DialCode: "+1869", Flag: "🇰🇳"},
	{ISO: "LC", Name: "Saint Lucia", DialCode: "+1758", Flag: "🇱🇨"},
	{ISO: "VC", Name: "Saint Vincent", DialCode: "+1784", Flag: "🇻🇨"},
	{ISO: "SA", Name: "Saudi Arabia", DialCode: "+966", Flag: "🇸🇦"},
	{ISO: "SN", Name: "Senegal", DialCode: "+221", Flag: "🇸🇳"},
	{ISO: "RS", Name: "Serbia", DialCode: "+381", Flag: "🇷🇸"},
	{ISO: "SC", Name: "Seychelles", DialCode: "+248", Flag: "🇸🇨"},
	{ISO: "SL", Name: "Sierra Leone", DialCode: "+232", Flag: "🇸🇱"},
	{ISO: "SG", Name: "Singapore", DialCode: "+65", Flag: "🇸🇬"},
	{ISO: "SK", Name: "Slovakia", DialCode: "+421", Flag: "🇸🇰"},
	{ISO: "SI", Name: "Slovenia", DialCode: "+386", Flag: "🇸🇮"},
	{ISO: "SO", Name: "Somalia", DialCode: "+252", Flag: "🇸🇴"},
	{ISO: "ZA", Name: "South Africa", DialCode: "+27", Flag: "🇿🇦"},
	{ISO: "KR", Name: "South Korea", DialCode: "+82", Flag: "🇰🇷"},
	{ISO: "ES", Name: "Spain", DialCode: "+34", Flag: "🇪🇸"},
	{ISO: "LK", Name: "Sri Lanka", DialCode: "+94", Flag: "🇱🇰"},
	{ISO: "SD", Name: "Sudan", DialCode: "+249", Flag: "🇸🇩"},
	{ISO: "SE", Name: "Sweden", DialCode: "+46", Flag: "🇸🇪"},
	{ISO: "CH", Name: "Switzerland", DialCode: "+41", Flag: "🇨🇭"},
	{ISO: "TW", Name: "Taiwan", DialCode: "+886", Flag: "🇹🇼"},
	{ISO: "TJ", Name: "Tajikistan", DialCode: "+992", Flag: "🇹🇯"},
	{ISO: "TZ", Name: "Tanzania", DialCode: "+255", Flag: "🇹🇿"},
	{ISO: "TH", Name: "Thailand", DialCode: "+66", Flag: "🇹🇭"},
	{ISO: "TR", Name: "Turkey", DialCode: "+90", Flag: "🇹🇷"},
	{ISO: "UG", Name: "Uganda", DialCode: "+256", Flag: "🇺🇬"},
	{ISO: "UA", Name: "Ukraine", DialCode: "+380", Flag: "🇺🇦"},
	{ISO: "AE", Name: "United Arab Emirates", DialCode: "+971", Flag: "🇦🇪"},
	{ISO: "GB", Name: "United Kingdom", DialCode: "+44", Flag: "🇬🇧"},
	{ISO: "US", Name: "United States", DialCode: "+1", Flag: "🇺🇸"},
	{ISO: "UY", Name: "Uruguay", DialCode: "+598", Flag: "🇺🇾"},
	{ISO: "UZ", Name: "Uzbekistan", DialCode: "+998", Flag: "🇺🇿"},
	{ISO: "VE", Name: "Venezuela", DialCode: "+58", Flag: "🇻🇪"},
	{ISO: "VN", Name: "Vietnam", DialCode: "+84", Flag: "🇻🇳"},
	{ISO: "YE", Name: "Yemen", DialCode: "+967", Flag: "🇾🇪"},
	{ISO: "ZM", Name: "Zambia", DialCode: "+260", Flag: "🇿🇲"},
	{ISO: "ZW", Name: "Zimbabwe", DialCode: "+263", Flag: "🇿🇼"},
}

// ParseAndValidatePhoneNumber parses any world phone number using Google libphonenumber,
// ensures international country code presence, and returns canonical E.164 format (e.g. +447123456789 or +923363220343).
func ParseAndValidatePhoneNumber(rawPhone, defaultRegion string) (string, error) {
	raw := strings.TrimSpace(rawPhone)
	raw = strings.ReplaceAll(raw, " ", "")
	raw = strings.ReplaceAll(raw, "-", "")
	raw = strings.ReplaceAll(raw, "(", "")
	raw = strings.ReplaceAll(raw, ")", "")
	raw = strings.ReplaceAll(raw, ".", "")

	if raw == "" {
		return "", fmt.Errorf("phone number cannot be empty")
	}

	// Reject raw numbers starting with 0 (e.g. 03363220343 or 07123456789)
	if strings.HasPrefix(raw, "0") && !strings.HasPrefix(raw, "00") {
		return "", fmt.Errorf("number cannot start with 0. Please select your country code and enter mobile number without leading 0")
	}

	if strings.HasPrefix(raw, "00") {
		raw = "+" + raw[2:]
	}

	// Reject invalid +0 prefix
	if strings.HasPrefix(raw, "+0") {
		return "", fmt.Errorf("country code cannot start with 0")
	}

	// If missing +, require at least 10 digits and check if starts with known dial code
	if !strings.HasPrefix(raw, "+") {
		if len(raw) < 8 || len(raw) > 15 {
			return "", fmt.Errorf("invalid phone length: must include country code")
		}
		raw = "+" + raw
	}

	num, err := phonenumbers.Parse(raw, "")
	if err != nil {
		if defaultRegion != "" {
			num, err = phonenumbers.Parse(raw, defaultRegion)
		}
		if err != nil {
			return "", fmt.Errorf("invalid international phone number: %w", err)
		}
	}

	if !phonenumbers.IsPossibleNumber(num) {
		return "", fmt.Errorf("phone number is not possible for country")
	}

	formatted := phonenumbers.Format(num, phonenumbers.E164)
	if len(formatted) < 8 || len(formatted) > 16 {
		return "", fmt.Errorf("phone number length invalid")
	}

	return formatted, nil
}
