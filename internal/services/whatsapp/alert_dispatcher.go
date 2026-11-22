package whatsapp

import (
	"context"
	"fmt"
	cryptorand "crypto/rand"
	"log/slog"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"jobyuk/internal/models"
	"jobyuk/internal/repository"
)

// AlertDispatcher handles automated job alert matching and scheduled delivery.
type AlertDispatcher struct {
	waSvc   *Service
	repos   *repository.Repositories
	baseURL string
	mu      sync.Mutex
	running bool
}

// NewAlertDispatcher creates a new automated alert dispatcher.
func NewAlertDispatcher(waSvc *Service, repos *repository.Repositories, baseURL string) *AlertDispatcher {
	return &AlertDispatcher{
		waSvc:   waSvc,
		repos:   repos,
		baseURL: baseURL,
	}
}

// StartScheduled starts the background ticker that periodically runs dispatch cycles.
func (d *AlertDispatcher) StartScheduled(ctx context.Context, interval time.Duration) {
	if interval < 5*time.Minute {
		interval = 15 * time.Minute
	}

	slog.Info("WhatsApp Alert Dispatcher scheduled worker started", "interval", interval.String())

	// Wait 30 seconds after server launch before running initial cycle
	select {
	case <-time.After(30 * time.Second):
		d.RunDispatchCycle(ctx)
	case <-ctx.Done():
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("WhatsApp Alert Dispatcher stopped")
			return
		case <-ticker.C:
			d.RunDispatchCycle(ctx)
		}
	}
}

// TriggerImmediateCycle kicks off a dispatch cycle immediately in a background goroutine.
func (d *AlertDispatcher) TriggerImmediateCycle(ctx context.Context) {
	go d.RunDispatchCycle(ctx)
}

// ScheduleInitialCandidateAlert schedules the candidate's first high-match job alert.
// Triggered right after trial verification or subscription signup.
// Delays delivery by 3-5 minutes (randomized, within user's trust window).
func (d *AlertDispatcher) ScheduleInitialCandidateAlert(sub *models.WhatsAppSubscriber, delay time.Duration) {
	if sub == nil || sub.PhoneNumber == "" {
		return
	}

	if delay < 3*time.Minute {
		// Crypto-secure random delay between 3-5 minutes (180-300 seconds)
		nBig, err := cryptorand.Int(cryptorand.Reader, big.NewInt(121))
		delaySeconds := int64(180) // 3 minutes minimum
		if err == nil {
			delaySeconds += nBig.Int64() // + 0-120 seconds = 180-300 seconds (3-5 minutes)
		}
		delay = time.Duration(delaySeconds) * time.Second
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Recovered from panic in ScheduleInitialCandidateAlert", "panic", r)
			}
		}()

		slog.Info("Scheduling initial candidate job alert", "phone", sub.PhoneNumber, "delay", delay.String())
		time.Sleep(delay)

		// Create dedicated background context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		if !d.waSvc.IsConnected() {
			slog.Warn("Cannot dispatch initial candidate job alert: WhatsApp gateway offline", "phone", sub.PhoneNumber)
			return
		}

		// Find best matching active job based on Role / Keywords / Category + Location
		job, err := d.repos.WhatsApp.FindBestJobForSubscriber(ctx, sub)
		if err != nil || job == nil {
			slog.Warn("No available jobs found for initial candidate alert", "phone", sub.PhoneNumber, "err", err)
			return
		}

		// Double-check not already sent
		alreadySent, _ := d.repos.WhatsApp.HasSubscriberReceivedJob(ctx, sub.ID, job.ID)
		if alreadySent {
			slog.Info("Candidate already received this job, skipping duplicate", "phone", sub.PhoneNumber, "job_id", job.ID)
			return
		}

		// Dispatch via WhatsApp with typing simulation and contact pre-registration
		err = d.waSvc.SendJobAlert(ctx, sub, job)
		if err != nil {
			slog.Error("Failed to deliver initial candidate job alert", "phone", sub.PhoneNumber, "job_id", job.ID, "err", err)
			return
		}

		slog.Info("🎉 Initial candidate job alert delivered successfully!",
			"phone", sub.PhoneNumber,
			"job_id", job.ID,
			"job_title", job.Title,
			"location", job.LocationName,
		)
	}()
}

// RunDispatchCycle processes all active subscribers and delivers newly matched jobs.
func (d *AlertDispatcher) RunDispatchCycle(ctx context.Context) {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		slog.Debug("WhatsApp Alert Dispatcher cycle already in progress, skipping")
		return
	}
	d.running = true
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		d.running = false
		d.mu.Unlock()
	}()

	if !d.waSvc.IsConnected() {
		slog.Debug("WhatsApp gateway offline, skipping alert dispatch cycle")
		return
	}

	subscribers, err := d.repos.WhatsApp.ListActiveAlertSubscribers(ctx)
	if err != nil {
		slog.Error("Failed to load active WhatsApp subscribers for dispatch", "err", err)
		return
	}

	if len(subscribers) == 0 {
		return
	}

	slog.Info("Starting WhatsApp automated alert dispatch cycle", "active_subscribers", len(subscribers))

	dispatchedCount := 0
	for _, sub := range subscribers {
		select {
		case <-ctx.Done():
			slog.Info("Dispatch cycle aborted: context cancelled")
			return
		default:
		}

		// Tier-Based Daily Alert Caps:
		// - Free Trial Candidates: Max 3 alerts/day
		// - Paid VIP Members: Max 7 alerts/day (Instant real-time priority)
		maxDailyAlerts := 7
		if sub.IsTrial || sub.PlanType == "trial" || sub.Source == "homepage_trial" || sub.TrialEndsAt != nil {
			maxDailyAlerts = 3
		}

		sentToday, _ := d.repos.WhatsApp.GetAlertsSentCountToday(ctx, sub.ID)
		if sentToday >= maxDailyAlerts {
			continue
		}

		// 30-Minute Cooling Period: Enforce minimum gap between consecutive alerts per subscriber
		// This prevents alert fatigue and ensures users don't ignore messages
		lastSentAt, _ := d.repos.WhatsApp.GetLastAlertSentTime(ctx, sub.ID)
		if lastSentAt != nil {
			elapsed := time.Since(*lastSentAt)
			if elapsed < 30*time.Minute {
				slog.Debug("Skipping subscriber due to 30-minute cooling period",
					"phone", sub.PhoneNumber,
					"last_alert", lastSentAt.Format(time.RFC3339),
					"elapsed_minutes", int(elapsed.Minutes()),
				)
				continue
			}
		}

		// Find best matching unsent job
		job, err := d.repos.WhatsApp.FindBestJobForSubscriber(ctx, sub)
		if err != nil || job == nil {
			continue
		}

		// Double-check not already sent
		alreadySent, _ := d.repos.WhatsApp.HasSubscriberReceivedJob(ctx, sub.ID, job.ID)
		if alreadySent {
			continue
		}

		// Send job alert
		sendErr := d.waSvc.SendJobAlert(ctx, sub, job)
		if sendErr == nil {
			dispatchedCount++
			slog.Info("Scheduled alert dispatched",
				"phone", sub.PhoneNumber,
				"job_id", job.ID,
				"title", job.Title,
			)

			// Human delay jitter between subscribers (4.0s - 7.5s) to avoid bot burst patterns
			jitter := time.Duration(4000+rand.Intn(3500)) * time.Millisecond
			time.Sleep(jitter)
		} else {
			slog.Warn("Failed to send scheduled job alert", "phone", sub.PhoneNumber, "err", sendErr)
		}
	}

	if dispatchedCount > 0 {
		slog.Info(fmt.Sprintf("Completed WhatsApp alert dispatch cycle: %d alerts sent", dispatchedCount))
	}
}

// ============================================================================
// WHATSAPP CHANNEL (NEWSLETTER) AUTO-POSTER WORKER
// ============================================================================

// StartChannelAutoPoster periodically checks and broadcasts high-CTR jobs throughout peak UK hours (08:00 - 20:00).
func (d *AlertDispatcher) StartChannelAutoPoster(ctx context.Context, interval time.Duration) {
	if interval < 10*time.Minute {
		interval = 35 * time.Minute // 35 minutes interval spreads 20 jobs cleanly over 12 peak daytime hours
	}

	slog.Info("WhatsApp Channel Auto-Poster worker started", "interval", interval.String(), "daily_cap", 20)

	// Initial check 45 seconds after launch
	select {
	case <-time.After(45 * time.Second):
		d.runChannelPostCycle(ctx)
	case <-ctx.Done():
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("WhatsApp Channel Auto-Poster stopped")
			return
		case <-ticker.C:
			d.runChannelPostCycle(ctx)
		}
	}
}

func (d *AlertDispatcher) runChannelPostCycle(ctx context.Context) {
	// Only post during active UK hours: 08:00 to 20:00 UTC
	hour := time.Now().UTC().Hour()
	if hour < 8 || hour >= 20 {
		slog.Debug("WhatsApp Channel Auto-Poster: outside active UK hours (08:00-20:00 UTC), skipping cycle", "hour", hour)
		return
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	_, _ = d.BroadcastNextChannelJob(ctxTimeout)
}

// BroadcastNextChannelJob selects the highest-CTR unposted UK job and broadcasts it to the WhatsApp Channel.
// Enforces a strict daily cap of 20 jobs and prevents any duplicates.
func (d *AlertDispatcher) BroadcastNextChannelJob(ctx context.Context) (*models.WhatsAppChannelPost, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 1. Check daily post limit (max 20 jobs/day)
	todayCount, err := d.repos.WhatsApp.GetChannelPostsTodayCount(ctx)
	if err != nil {
		slog.Error("Failed to check WhatsApp Channel today posts count", "err", err)
	} else if todayCount >= 20 {
		slog.Info("WhatsApp Channel reached daily posting cap (20/20). Pausing until tomorrow.", "today_count", todayCount)
		return nil, fmt.Errorf("daily channel posting limit reached (%d/20)", todayCount)
	}

	// 2. Find best high-CTR job not yet posted to the channel
	job, err := d.repos.WhatsApp.FindBestHighCTRJobForChannel(ctx)
	if err != nil {
		slog.Warn("No available unposted high-CTR jobs found for WhatsApp Channel", "err", err)
		return nil, fmt.Errorf("no unposted jobs found for channel: %w", err)
	}
	if job == nil {
		return nil, fmt.Errorf("no unposted jobs available")
	}

	// 3. Post to Channel via WhatsApp Service
	post, err := d.waSvc.PostJobToChannel(ctx, job, "0029Vb9iXUMAu3aQOlanBy2m")
	if err != nil {
		slog.Error("Failed to broadcast job to WhatsApp Channel", "job_id", job.ID, "err", err)
		return post, err
	}

	slog.Info("WhatsApp Channel Job broadcast successful",
		"job_id", job.ID,
		"title", job.Title,
		"company", job.EmployerName,
		"daily_progress", fmt.Sprintf("%d/20", todayCount+1),
	)

	return post, nil
}

