package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// SalaryCalculator renders the interactive UK Salary & Take-Home Tax Calculator.
func (h *Handlers) SalaryCalculator(w http.ResponseWriter, r *http.Request) {
	year := time.Now().Year()
	data := PageData{
		Title:       "UK Take-Home Pay Calculator 2026/27 | Tax & NI | JobyUK",
		Description: "Work out your UK take-home pay for 2026/27. Free calculator with income tax, National Insurance, pension and student loan. Monthly and weekly.",
		Keywords:    fmt.Sprintf("take home pay calculator UK, UK salary calculator %d, salary after tax UK, PAYE tax calculator, 8 percent national insurance, UK net wage calculator", year),
		Canonical:   h.cfg.BaseURL + "/salary-calculator/",
		OGTitle:     "UK Take-Home Pay Calculator 2026/27 | Tax & NI | JobyUK",
		OGDesc:      "Work out your UK take-home pay for 2026/27. Free calculator with income tax, National Insurance, pension and student loan. Monthly and weekly.",
		OGImage:     h.cfg.BaseURL + "/static/images/og-image.jpg",
		OGType:      "website",
		Robots:      "index,follow",
		ActiveNav:   "salaries",
		Breadcrumbs: []BreadcrumbData{
			{Name: "Home", URL: "/"},
			{Name: "Salaries", URL: "/salaries/"},
			{Name: "Salary Calculator"},
		},
		Data: map[string]interface{}{
			"Year": year,
		},
	}

	if err := h.renderPage(w, r, "salary_calculator", data); err != nil {
		slog.Error("Failed to render salary calculator page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
