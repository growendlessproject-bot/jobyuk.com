package freehire

import (
	"strings"
)

// directATSDomains contains verified Applicant Tracking Systems (ATS) where candidates apply directly to companies.
var directATSDomains = []string{
	"workdayjobs.com",
	"myworkdayjobs.com",
	"greenhouse.io",
	"lever.co",
	"jobs.ashbyhq.com",
	"ashbyhq.com",
	"pinpointhq.com",
	"teamtailor.com",
	"hire.trakstar.com",
	"trakstar.com",
	"icims.com",
	"breezy.hr",
	"bamboohr.com",
	"smartrecruiters.com",
	"jobvite.com",
	"personio.com",
	"personio.de",
	"personio.es",
	"factorialhr.com",
	"factorialhr.pt",
	"applicantpro.com",
	"recruitee.com",
	"workable.com",
	"applytojob.com",
	"jazzhr.com",
	"submittable.com",
	"oraclecloud.com",
	"rippling-ats.com",
	"eightfold.ai",
}

// intermediateAggregators contains job boards and aggregators that should be excluded to avoid intermediate redirects.
var intermediateAggregators = []string{
	"indeed.com",
	"reed.co.uk",
	"adzuna",
	"himalayas.app",
	"4dayweek.io",
	"arbeitnow.co.uk",
	"arbeitnow.com",
	"remote.com/jobs",
	"whatjobs.com",
	"whatjobs-",
	"mycareersfuture.gov.sg",
	"powertofly.com",
	"ziprecruiter",
	"monster.com",
	"totaljobs.com",
	"cv-library.co.uk",
	"glassdoor.com",
	"jooble.org",
	"jobicy.com",
}

// IsDirectATS checks if the URL belongs to a verified applicant tracking system.
func IsDirectATS(urlStr string) bool {
	urlLower := strings.ToLower(urlStr)
	for _, domain := range directATSDomains {
		if strings.Contains(urlLower, domain) {
			return true
		}
	}
	return false
}

// IsAggregator checks if the URL belongs to a known aggregator/job board.
func IsAggregator(urlStr string) bool {
	urlLower := strings.ToLower(urlStr)
	for _, agg := range intermediateAggregators {
		if strings.Contains(urlLower, agg) {
			return true
		}
	}
	return false
}

// IsTargetableUKDirectLink returns true if the URL is not an aggregator and represents a direct employer application.
func IsTargetableUKDirectLink(urlStr string) bool {
	if urlStr == "" {
		return false
	}
	if IsAggregator(urlStr) {
		return false
	}
	return true
}
