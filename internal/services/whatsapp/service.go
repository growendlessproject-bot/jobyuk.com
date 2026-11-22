package whatsapp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	qrcode "github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"

	"jobyuk/internal/models"
	"jobyuk/internal/repository"
)

// DeviceSession manages an independent whatsmeow connection and SQLite session store.
type DeviceSession struct {
	ID            string // "alerts" or "support"
	Label         string // "Job Alerts Gateway" or "Billing & Support Concierge"
	DBPath        string
	client        *whatsmeow.Client
	container     *sqlstore.Container
	mu            sync.RWMutex
	isConnected   bool
	phone         string
	name          string
	qrData        string
	qrExpiry      time.Time
	isPairing     bool
	pairingCancel context.CancelFunc
	repo          *repository.WhatsAppRepository
}

// newDeviceSession initializes a standalone whatsmeow client with isolated SQLite storage.
func newDeviceSession(id, label, dbURI string, repo *repository.WhatsAppRepository) *DeviceSession {
	ds := &DeviceSession{
		ID:     id,
		Label:  label,
		DBPath: dbURI,
		repo:   repo,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	container, err := sqlstore.New(ctx, "sqlite", dbURI, waLog.Noop)
	if err != nil {
		slog.Error("Failed to initialize whatsmeow sqlstore", "device", id, "err", err)
		return ds
	}
	ds.container = container

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil || deviceStore == nil {
		deviceStore = container.NewDevice()
	}

	client := whatsmeow.NewClient(deviceStore, waLog.Noop)
	ds.client = client
	ds.attachClientHandlers(client)

	// If previously authenticated, reconnect automatically
	if client.Store.ID != nil {
		go func() {
			if err := client.Connect(); err != nil {
				slog.Warn("Failed to reconnect existing WhatsApp session", "device", ds.ID, "err", err)
			} else {
				ds.mu.Lock()
				ds.isConnected = true
				ds.phone = "+" + client.Store.ID.User
				ds.name = ds.Label
				phone := ds.phone
				ds.mu.Unlock()
				_ = ds.repo.UpdateGatewayConnection(context.Background(), ds.ID, true, phone, ds.Label)
				slog.Info("Existing WhatsApp session reconnected automatically", "device", ds.ID, "phone", phone)
			}
		}()
	}

	return ds
}

// attachClientHandlers binds event listeners to a whatsmeow client instance.
func (ds *DeviceSession) attachClientHandlers(client *whatsmeow.Client) {
	client.AddEventHandler(func(rawEvt interface{}) {
		switch rawEvt.(type) {
		case *events.Connected:
			ds.mu.Lock()
			ds.isConnected = true
			if ds.client != nil && ds.client.Store != nil && ds.client.Store.ID != nil {
				ds.phone = "+" + ds.client.Store.ID.User
			}
			ds.name = ds.Label
			ds.qrData = ""
			ds.isPairing = false
			phone := ds.phone
			ds.mu.Unlock()

			_ = ds.repo.UpdateGatewayConnection(context.Background(), ds.ID, true, phone, ds.Label)
			slog.Info("Authentic WhatsApp Web Device Connected!", "device", ds.ID, "phone", phone)

		case *events.LoggedOut:
			ds.mu.Lock()
			ds.isConnected = false
			ds.phone = ""
			ds.name = ""
			ds.qrData = ""
			ds.isPairing = false
			ds.mu.Unlock()

			_ = ds.repo.UpdateGatewayConnection(context.Background(), ds.ID, false, "", "")
			slog.Warn("WhatsApp Device Logged Out", "device", ds.ID)
		}
	})
}

// ResetSession terminates existing connections, deletes stale session keys in SQLite, and prepares a fresh client.
func (ds *DeviceSession) ResetSession(ctx context.Context) error {
	ds.mu.Lock()
	if ds.pairingCancel != nil {
		ds.pairingCancel()
		ds.pairingCancel = nil
	}
	cli := ds.client
	ds.isConnected = false
	ds.phone = ""
	ds.name = ""
	ds.qrData = ""
	ds.qrExpiry = time.Time{}
	ds.isPairing = false
	ds.mu.Unlock()

	if cli != nil {
		cli.Disconnect()
		if cli.Store != nil {
			_ = cli.Store.Delete(ctx)
		}
	}

	ds.mu.Lock()
	if ds.container != nil {
		newStore := ds.container.NewDevice()
		newCli := whatsmeow.NewClient(newStore, waLog.Noop)
		ds.client = newCli
		ds.attachClientHandlers(newCli)
	}
	ds.mu.Unlock()

	_ = ds.repo.UpdateGatewayConnection(ctx, ds.ID, false, "", "")
	slog.Info("WhatsApp device session reset cleanly", "device", ds.ID)
	return nil
}

// StartPairing initiates the QR code stream for this specific device session.
func (ds *DeviceSession) StartPairing() error {
	ds.mu.Lock()
	if ds.client == nil {
		ds.mu.Unlock()
		return fmt.Errorf("whatsapp client not initialized for %s", ds.ID)
	}

	// If device is legitimately connected and active, do nothing
	if ds.isConnected && ds.client.Store != nil && ds.client.Store.ID != nil && ds.client.IsConnected() {
		ds.phone = "+" + ds.client.Store.ID.User
		ds.mu.Unlock()
		return nil
	}

	// If active pairing is already generating a valid, non-expired QR code, return immediately
	if ds.isPairing && ds.qrData != "" && time.Now().Before(ds.qrExpiry) {
		ds.mu.Unlock()
		return nil
	}

	if ds.pairingCancel != nil {
		ds.pairingCancel()
		ds.pairingCancel = nil
	}

	// If device is offline, ensure any dead/orphaned session is cleaned up
	if !ds.isConnected {
		if ds.client != nil {
			ds.client.Disconnect()
			if ds.client.Store != nil && ds.client.Store.ID != nil {
				_ = ds.client.Store.Delete(context.Background())
			}
		}
		if ds.container != nil {
			newStore := ds.container.NewDevice()
			newCli := whatsmeow.NewClient(newStore, waLog.Noop)
			ds.client = newCli
			ds.attachClientHandlers(newCli)
		}
	} else if ds.client.IsConnected() && ds.client.Store.ID == nil {
		ds.client.Disconnect()
	}

	pairCtx, pairCancel := context.WithCancel(context.Background())
	ds.pairingCancel = pairCancel
	ds.isPairing = true
	ds.qrData = ""
	ds.mu.Unlock()

	qrChan, err := ds.client.GetQRChannel(pairCtx)
	if err != nil {
		// If already connected error occurs, disconnect and retry once
		if errors.Is(err, whatsmeow.ErrQRAlreadyConnected) || strings.Contains(err.Error(), "already connected") {
			ds.client.Disconnect()
			time.Sleep(150 * time.Millisecond)
			qrChan, err = ds.client.GetQRChannel(pairCtx)
		}
	}
	if err != nil {
		if errors.Is(err, whatsmeow.ErrQRStoreContainsID) || strings.Contains(err.Error(), "already logged in") {
			ds.client.Disconnect()
			if ds.client.Store != nil {
				_ = ds.client.Store.Delete(pairCtx)
			}
			ds.mu.Lock()
			if ds.container != nil {
				newStore := ds.container.NewDevice()
				newCli := whatsmeow.NewClient(newStore, waLog.Noop)
				ds.client = newCli
				ds.attachClientHandlers(newCli)
			}
			ds.mu.Unlock()

			qrChan, err = ds.client.GetQRChannel(pairCtx)
			if err != nil {
				ds.mu.Lock()
				ds.isPairing = false
				ds.mu.Unlock()
				return err
			}
		} else {
			ds.mu.Lock()
			ds.isPairing = false
			ds.mu.Unlock()
			return err
		}
	}

	go func() {
		defer func() {
			ds.mu.Lock()
			ds.isPairing = false
			ds.mu.Unlock()
		}()

		for {
			select {
			case <-pairCtx.Done():
				return
			case evt, ok := <-qrChan:
				if !ok {
					return
				}
				switch evt.Event {
				case whatsmeow.QRChannelEventCode:
					ds.mu.Lock()
					ds.qrData = evt.Code
					ds.qrExpiry = time.Now().Add(evt.Timeout)
					ds.mu.Unlock()
					slog.Info("Whatsmeow: Authentic QR Code received from WhatsApp", "device", ds.ID, "timeout", evt.Timeout)

				case whatsmeow.QRChannelSuccess.Event:
					ds.mu.Lock()
					ds.isConnected = true
					if ds.client.Store.ID != nil {
						ds.phone = "+" + ds.client.Store.ID.User
					}
					ds.name = ds.Label
					ds.qrData = ""
					phone := ds.phone
					ds.mu.Unlock()

					_ = ds.repo.UpdateGatewayConnection(context.Background(), ds.ID, true, phone, ds.Label)
					slog.Info("Whatsmeow: QR pairing authenticated by phone!", "device", ds.ID, "phone", phone)
					return

				case whatsmeow.QRChannelTimeout.Event:
					ds.mu.Lock()
					ds.qrData = ""
					ds.mu.Unlock()
					slog.Debug("Whatsmeow: QR code expired, awaiting refresh", "device", ds.ID)
					return

				default:
					slog.Debug("Whatsmeow: QR pairing event", "device", ds.ID, "event", evt.Event)
				}
			}
		}
	}()

	if !ds.client.IsConnected() {
		go func() {
			if err := ds.client.Connect(); err != nil {
				slog.Error("Whatsmeow: Connect failed during pairing", "device", ds.ID, "err", err)
			}
		}()
	}

	return nil
}

// GenerateNewQR forces a fresh pairing stream and waits up to 15s for the first QR code.
func (ds *DeviceSession) GenerateNewQR() (string, error) {
	ds.mu.Lock()
	if ds.isConnected {
		ds.mu.Unlock()
		return "", fmt.Errorf("device is already connected")
	}

	// Fast-path: if an unexpired authentic QR is already available, reuse it immediately
	if ds.qrData != "" && time.Now().Before(ds.qrExpiry) {
		data := ds.qrData
		ds.mu.Unlock()
		pngBytes, err := qrcode.Encode(data, qrcode.Medium, 280)
		if err == nil {
			return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes), nil
		}
		ds.mu.Lock()
	}

	if ds.pairingCancel != nil {
		ds.pairingCancel()
		ds.pairingCancel = nil
	}
	ds.qrData = ""
	ds.isPairing = false
	ds.mu.Unlock()

	if err := ds.StartPairing(); err != nil {
		return "", err
	}

	// Wait up to 15s (150 * 100ms) for WhatsApp WebSocket handshake to complete
	for i := 0; i < 150; i++ {
		ds.mu.RLock()
		data := ds.qrData
		expired := time.Now().After(ds.qrExpiry)
		connected := ds.isConnected
		ds.mu.RUnlock()

		if connected {
			return "", nil
		}
		if data != "" && !expired {
			pngBytes, err := qrcode.Encode(data, qrcode.Medium, 280)
			if err != nil {
				return "", err
			}
			return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes), nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return "", fmt.Errorf("timeout waiting for WhatsApp QR from server")
}

// GetQRStatus returns the active QR Data URI or expiry state.
func (ds *DeviceSession) GetQRStatus() (qrURI string, isExpired bool, isConnected bool, phone string) {
	ds.mu.RLock()
	if ds.isConnected {
		phone := ds.phone
		ds.mu.RUnlock()
		return "", false, true, phone
	}
	data := ds.qrData
	expired := ds.qrData == "" || time.Now().After(ds.qrExpiry)
	ds.mu.RUnlock()

	if expired {
		return "", true, false, ""
	}

	pngBytes, err := qrcode.Encode(data, qrcode.Medium, 280)
	if err != nil {
		return "", true, false, ""
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes), false, false, ""
}

// Disconnect logs out and clears state for this specific device.
func (ds *DeviceSession) Disconnect(ctx context.Context) error {
	ds.mu.Lock()
	if ds.pairingCancel != nil {
		ds.pairingCancel()
		ds.pairingCancel = nil
	}
	cli := ds.client
	ds.isConnected = false
	ds.phone = ""
	ds.name = ""
	ds.qrData = ""
	ds.isPairing = false
	ds.mu.Unlock()

	if cli != nil {
		_ = cli.Logout(ctx)
		cli.Disconnect()
		if cli.Store != nil {
			_ = cli.Store.Delete(ctx)
		}
	}

	ds.mu.Lock()
	if ds.container != nil {
		newStore := ds.container.NewDevice()
		newCli := whatsmeow.NewClient(newStore, waLog.Noop)
		ds.client = newCli
		ds.attachClientHandlers(newCli)
	}
	ds.mu.Unlock()

	return ds.repo.UpdateGatewayConnection(ctx, ds.ID, false, "", "")
}

// IsConnected returns connection status of this device.
func (ds *DeviceSession) IsConnected() bool {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.isConnected && ds.client != nil && ds.client.IsConnected()
}

// GetStatus returns status tuple.
func (ds *DeviceSession) GetStatus() (bool, string, string) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.isConnected, ds.phone, ds.name
}

// dispatch delivers text via this specific device client with human typing simulation, jitter, and contact pre-registration.
func (ds *DeviceSession) dispatch(ctx context.Context, recipientPhone, message, logLabel string) error {
	return ds.dispatchWithContact(ctx, recipientPhone, "", message, logLabel)
}

func (ds *DeviceSession) dispatchWithContact(ctx context.Context, recipientPhone, contactName, message, logLabel string) error {
	normPhone := repository.NormalizePhoneNumber(recipientPhone)
	_ = ds.repo.IncrementDailyMessages(ctx)

	ds.mu.RLock()
	cli := ds.client
	connected := ds.isConnected && cli != nil && cli.IsConnected()
	ds.mu.RUnlock()

	cleanPhone := strings.TrimPrefix(normPhone, "+")
	cleanPhone = strings.ReplaceAll(cleanPhone, " ", "")

	if connected && cleanPhone != "" {
		targetJID := types.NewJID(cleanPhone, types.DefaultUserServer)

		// 1. Anti-Ban: Pre-register / save contact in whatsmeow internal ContactStore before sending
		if cli.Store != nil && cli.Store.Contacts != nil {
			cName := strings.TrimSpace(contactName)
			if cName == "" {
				suffix := cleanPhone
				if len(cleanPhone) > 4 {
					suffix = cleanPhone[len(cleanPhone)-4:]
				}
				cName = fmt.Sprintf("JobyUK.com Candidate %s", suffix)
			}
			firstName := cName
			if idx := strings.Index(cName, " "); idx > 0 {
				firstName = cName[:idx]
			}
			_ = cli.Store.Contacts.PutContactName(ctx, targetJID, cName, firstName)
		}

		// 2. Anti-Ban: Check if recipient is registered on WhatsApp (prevents dead number penalties)
		resp, err := cli.IsOnWhatsApp(ctx, []string{cleanPhone})
		if err == nil && len(resp) > 0 && !resp[0].IsIn {
			slog.Warn("Target phone is not registered on WhatsApp", "device", ds.ID, "phone", cleanPhone)
			return fmt.Errorf("phone number %s is not registered on WhatsApp", cleanPhone)
		}

		// 3. Anti-Ban: Human Typing Simulation ("is typing...")
		_ = cli.SendChatPresence(ctx, targetJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)

		// Dynamic typing delay (1.2s to 2.4s) with crypto/rand jitter
		nBig, _ := rand.Int(rand.Reader, big.NewInt(1200))
		typingMs := 1200 + int(nBig.Int64())
		select {
		case <-time.After(time.Duration(typingMs) * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}

		// 4. Pause presence right before dispatch
		_ = cli.SendChatPresence(ctx, targetJID, types.ChatPresencePaused, types.ChatPresenceMediaText)

		// 5. Send message
		_, err = cli.SendMessage(ctx, targetJID, &waE2E.Message{
			Conversation: proto.String(message),
		})
		if err != nil {
			slog.Error("Failed to deliver message via whatsmeow", "device", ds.ID, "to", targetJID, "err", err)
			return err
		}
		slog.Info("Real WhatsApp Message Sent Successfully via whatsmeow", "device", ds.ID, "to", targetJID, "label", logLabel, "typing_ms", typingMs)
		return nil
	}

	return fmt.Errorf("device %s not connected", ds.ID)
}

// ContactSyncer defines an optional external sync provider (e.g. Google People API).
type ContactSyncer interface {
	IsConnected() bool
	CreateContact(ctx context.Context, phone, fullName string) error
}

// Service manages the quad WhatsApp gateway (OTP + Alerts + Billing + Support) with smart routing and graceful fallback.
type Service struct {
	repo           *repository.WhatsAppRepository
	baseURL        string
	otpSession     *DeviceSession
	alertsSession  *DeviceSession
	billingSession *DeviceSession
	supportSession *DeviceSession
	contactSyncer  ContactSyncer
}

// SetContactSyncer injects the external contact sync provider (Google Contacts).
func (s *Service) SetContactSyncer(syncer ContactSyncer) {
	s.contactSyncer = syncer
}

// NewService creates a new WhatsApp Gateway Service managing four isolated device channels.
func NewService(repo *repository.WhatsAppRepository, baseURL string) *Service {
	s := &Service{
		repo:    repo,
		baseURL: strings.TrimRight(baseURL, "/"),
	}

	_ = os.MkdirAll("data", 0755)

	// Backward Compatibility: If data/whatsapp.db exists and data/whatsapp_alerts.db does not, migrate it over
	if _, err := os.Stat("data/whatsapp_alerts.db"); os.IsNotExist(err) {
		if _, errOld := os.Stat("data/whatsapp.db"); errOld == nil {
			copyFile("data/whatsapp.db", "data/whatsapp_alerts.db")
			if _, errWAL := os.Stat("data/whatsapp.db-wal"); errWAL == nil {
				copyFile("data/whatsapp.db-wal", "data/whatsapp_alerts.db-wal")
			}
			if _, errSHM := os.Stat("data/whatsapp.db-shm"); errSHM == nil {
				copyFile("data/whatsapp.db-shm", "data/whatsapp_alerts.db-shm")
			}
			slog.Info("Migrated existing WhatsApp session store to data/whatsapp_alerts.db")
		}
	}

	// Device 1: OTP Verification Gateway (dedicated for all OTP codes)
	s.otpSession = newDeviceSession("otp", "OTP Verification Gateway", "file:data/whatsapp_otp.db?_pragma=foreign_keys(1)", repo)

	// Device 2: Job Alerts Gateway (dedicated for job alert dispatch)
	s.alertsSession = newDeviceSession("alerts", "Job Alerts Gateway", "file:data/whatsapp_alerts.db?_pragma=foreign_keys(1)", repo)

	// Device 3: Payment & Invoice Gateway (dedicated for payment confirmations and invoices)
	s.billingSession = newDeviceSession("billing", "Payment & Invoice Gateway", "file:data/whatsapp_billing.db?_pragma=foreign_keys(1)", repo)

	// Device 4: Support Concierge Gateway (dedicated for customer support)
	s.supportSession = newDeviceSession("support", "Support Concierge Gateway", "file:data/whatsapp_support.db?_pragma=foreign_keys(1)", repo)

	slog.Info("WhatsApp Quad-Gateway Service initialized", "devices", []string{"otp", "alerts", "billing", "support"})
	return s
}

// copyFile is a helper to copy SQLite databases cleanly.
func copyFile(src, dst string) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer out.Close()

	_, _ = io.Copy(out, in)
}

// GetSession retrieves the requested device session by name.
func (s *Service) GetSession(device string) *DeviceSession {
	switch device {
	case "otp":
		return s.otpSession
	case "billing", "payment", "invoice":
		return s.billingSession
	case "support":
		return s.supportSession
	default:
		return s.alertsSession
	}
}

// GetDeviceStatus returns connection status, phone, and label for a specific device.
func (s *Service) GetDeviceStatus(device string) (bool, string, string) {
	return s.GetSession(device).GetStatus()
}

// GetStatus returns the alerts device status for backward compatibility.
func (s *Service) GetStatus() (bool, string, string) {
	return s.alertsSession.GetStatus()
}

// GetQRStatus returns QR code data for a device (defaults to "alerts").
func (s *Service) GetQRStatus(device ...string) (qrURI string, isExpired bool, isConnected bool, phone string) {
	dev := "alerts"
	if len(device) > 0 && device[0] != "" {
		dev = device[0]
	}
	return s.GetSession(dev).GetQRStatus()
}

// GenerateNewQR forces fresh QR pairing for a device (defaults to "alerts").
func (s *Service) GenerateNewQR(device ...string) (string, error) {
	dev := "alerts"
	if len(device) > 0 && device[0] != "" {
		dev = device[0]
	}
	return s.GetSession(dev).GenerateNewQR()
}

// Disconnect logs out a specific device (defaults to "alerts").
func (s *Service) Disconnect(ctx context.Context, device ...string) error {
	dev := "alerts"
	if len(device) > 0 && device[0] != "" {
		dev = device[0]
	}
	return s.GetSession(dev).Disconnect(ctx)
}

// ResetDevice completely clears session keys and resets the device client for fresh pairing.
func (s *Service) ResetDevice(ctx context.Context, device ...string) error {
	dev := "alerts"
	if len(device) > 0 && device[0] != "" {
		dev = device[0]
	}
	return s.GetSession(dev).ResetSession(ctx)
}

// IsConnected returns true if at least one device is online.
func (s *Service) IsConnected() bool {
	return s.otpSession.IsConnected() || s.alertsSession.IsConnected() || s.billingSession.IsConnected() || s.supportSession.IsConnected()
}

// DispatchChannelMessage intelligently dispatches a message via the requested channel with graceful fallback.
func (s *Service) DispatchChannelMessage(ctx context.Context, preferredChannel, recipientPhone, message, logLabel string) error {
	return s.DispatchChannelMessageWithContact(ctx, preferredChannel, recipientPhone, "", message, logLabel)
}

// DispatchChannelMessageWithContact dispatches with pre-registered contact name, quad-gateway routing, and anti-ban protections.
func (s *Service) DispatchChannelMessageWithContact(ctx context.Context, preferredChannel, recipientPhone, contactName, message, logLabel string) error {
	settings, _ := s.repo.GetGatewaySettings(ctx)

	targetDevice := preferredChannel
	if settings != nil {
		switch preferredChannel {
		case "otp":
			if settings.OTPDevice != "" {
				targetDevice = settings.OTPDevice
			}
		case "alerts":
			if settings.AlertsDevice != "" {
				targetDevice = settings.AlertsDevice
			}
		case "billing", "payment", "invoice":
			if settings.BillingDevice != "" {
				targetDevice = settings.BillingDevice
			}
		case "support":
			if settings.SupportDevice != "" {
				targetDevice = settings.SupportDevice
			}
		}
	}

	primarySession := s.GetSession(targetDevice)
	if primarySession != nil && primarySession.IsConnected() {
		return primarySession.dispatchWithContact(ctx, recipientPhone, contactName, message, logLabel)
	}

	// Intelligent Quad-Gateway Fallback Chain:
	// otp → alerts → billing → support
	// alerts → otp → billing → support
	// billing → support → alerts → otp
	// support → billing → alerts → otp
	var fallbackOrder []*DeviceSession
	switch targetDevice {
	case "otp":
		fallbackOrder = []*DeviceSession{s.alertsSession, s.billingSession, s.supportSession}
	case "alerts":
		fallbackOrder = []*DeviceSession{s.otpSession, s.billingSession, s.supportSession}
	case "billing", "payment", "invoice":
		fallbackOrder = []*DeviceSession{s.supportSession, s.alertsSession, s.otpSession}
	case "support":
		fallbackOrder = []*DeviceSession{s.billingSession, s.alertsSession, s.otpSession}
	default:
		fallbackOrder = []*DeviceSession{s.otpSession, s.billingSession, s.supportSession}
	}

	for _, fb := range fallbackOrder {
		if fb != nil && fb.IsConnected() {
			slog.Warn("Targeted WhatsApp device is offline, automatically dispatched via fallback connected device",
				"target", targetDevice, "fallback", fb.ID, "phone", fb.phone)
			return fb.dispatchWithContact(ctx, recipientPhone, contactName, message, logLabel+" (fallback:"+fb.ID+")")
		}
	}

	normPhone := repository.NormalizePhoneNumber(recipientPhone)
	slog.Info("WhatsApp Dispatch Logged (All gateway devices offline)", "recipient", normPhone, "label", logLabel)
	return nil
}

// SaveContact registers a contact name with a phone number in all connected WhatsApp device stores and Google Contacts.
func (s *Service) SaveContact(ctx context.Context, phone, contactName string) error {
	normPhone := repository.NormalizePhoneNumber(phone)
	cleanPhone := strings.TrimPrefix(normPhone, "+")
	cleanPhone = strings.ReplaceAll(cleanPhone, " ", "")
	if cleanPhone == "" {
		return fmt.Errorf("invalid phone number")
	}

	targetJID := types.NewJID(cleanPhone, types.DefaultUserServer)
	cName := strings.TrimSpace(contactName)
	if cName == "" {
		suffix := cleanPhone
		if len(cleanPhone) > 4 {
			suffix = cleanPhone[len(cleanPhone)-4:]
		}
		cName = fmt.Sprintf("JobyUK.com Candidate %s", suffix)
	}
	firstName := cName
	if idx := strings.Index(cName, " "); idx > 0 {
		firstName = cName[:idx]
	}

	for _, session := range []*DeviceSession{s.otpSession, s.alertsSession, s.billingSession, s.supportSession} {
		if session != nil {
			session.mu.RLock()
			cli := session.client
			session.mu.RUnlock()
			if cli != nil && cli.Store != nil && cli.Store.Contacts != nil {
				err := cli.Store.Contacts.PutContactName(ctx, targetJID, cName, firstName)
				if err != nil {
					slog.Warn("whatsmeow PutContactName warning", "device", session.ID, "err", err)
				} else {
					slog.Info("Contact saved in whatsmeow session store", "device", session.ID, "name", cName, "phone", cleanPhone)
				}
			}
		}
	}

	// Also sync to Google Contacts (People API) if connected
	if s.contactSyncer != nil && s.contactSyncer.IsConnected() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.contactSyncer.CreateContact(syncCtx, normPhone, cName); err != nil {
			slog.Warn("Failed to sync contact to Google Contacts", "phone", normPhone, "name", cName, "err", err)
		} else {
			slog.Info("Contact synced to Google Contacts successfully", "phone", normPhone, "name", cName)
		}
	}

	return nil
}

// DispatchTextMessage delivers text via the default alerts channel with graceful fallback.
func (s *Service) DispatchTextMessage(ctx context.Context, recipientPhone, message, logLabel string) error {
	return s.DispatchChannelMessage(ctx, "alerts", recipientPhone, message, logLabel)
}

// Generate6DigitOTP creates a crypto-secure 6-digit verification code.
func Generate6DigitOTP() string {
	nBig, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	return fmt.Sprintf("%06d", nBig.Int64()+100000)
}

// SendOTP sends a 6-digit code to the user's WhatsApp and saves it in the database.
func (s *Service) SendOTP(ctx context.Context, phone string, contactName ...string) (string, error) {
	normPhone := repository.NormalizePhoneNumber(phone)
	if normPhone == "" {
		return "", fmt.Errorf("invalid phone number")
	}

	code := Generate6DigitOTP()

	if err := s.repo.SaveOTP(ctx, normPhone, code); err != nil {
		slog.Error("Failed to save OTP to database", "err", err, "phone", normPhone)
		return "", fmt.Errorf("failed to save OTP: %w", err)
	}

	name := ""
	if len(contactName) > 0 && contactName[0] != "" {
		name = contactName[0]
	}

	msg := fmt.Sprintf(
		"👋 *JobyUK.com Verification*\n\n"+
			"Your verification code is: *%s*\n\n"+
			"Enter this code on JobyUK.com to activate your WhatsApp job alerts. Valid for 5 minutes.\n\n"+
			"🔒 _Keep this code confidential._\n"+
			"💡 _Tip: Save JobyUK.com to your contacts so job alerts reach you instantly._\n"+
			"_Reply STOP to mute alerts._",
		code,
	)

	go func() {
		dispatchCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		_ = s.DispatchChannelMessageWithContact(dispatchCtx, "otp", normPhone, name, msg, "OTP Verification")
	}()

	slog.Info("WhatsApp OTP generated and dispatched", "phone", normPhone, "code", code)
	return code, nil
}

// SendTrialVerificationOTP sends a 6-digit verification code to confirm phone ownership before activating the 3-day free trial.
func (s *Service) SendTrialVerificationOTP(ctx context.Context, phone, code string, contactName ...string) error {
	normPhone := repository.NormalizePhoneNumber(phone)
	if normPhone == "" {
		return fmt.Errorf("invalid phone number")
	}

	name := ""
	if len(contactName) > 0 && contactName[0] != "" {
		name = contactName[0]
	}

	// CRITICAL: Ensure candidate contact is actively saved in WhatsApp session and Google Contacts BEFORE dispatching message!
	_ = s.SaveContact(ctx, normPhone, name)

	msg := fmt.Sprintf(
		"👋 *JobyUK.com 3-Day Free Trial*\n\n"+
			"Your 6-digit verification code is: *%s*\n\n"+
			"Enter this code on JobyUK.com to verify your phone number and activate your *3-Day Free Job Alerts Trial*.\n\n"+
			"⏱️ _Code expires in 10 minutes._\n"+
			"💡 _Save JobyUK.com in your contacts so job matches arrive instantly in your inbox without spam filters._\n"+
			"_Reply STOP to unsubscribe at any time._",
		code,
	)

	go func() {
		dispatchCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		_ = s.DispatchChannelMessageWithContact(dispatchCtx, "otp", normPhone, name, msg, "Trial Verification OTP")
	}()

	slog.Info("Trial verification OTP dispatched", "phone", normPhone)
	return nil
}

// SendJobAlert dispatches an alert to a subscriber and logs it.
func (s *Service) SendJobAlert(ctx context.Context, sub *models.WhatsAppSubscriber, job *models.Job) error {
	if sub == nil || job == nil {
		return fmt.Errorf("subscriber or job is nil")
	}

	salary := job.FormattedSalary()
	if salary == "" {
		salary = "Competitive"
	}

	jobURL := fmt.Sprintf("%s/jobs/%s", s.baseURL, job.Slug)
	if s.baseURL == "" {
		jobURL = fmt.Sprintf("https://jobyuk.com/jobs/%s", job.Slug)
	}

	trialFooter := ""
	if sub.IsTrial {
		hoursLeft := sub.TrialHoursRemaining
		if hoursLeft <= 0 {
			hoursLeft = 72
		}
		trialFooter = fmt.Sprintf("\n\n━━━━━━━━━━━━━━━━━━━━\n⚡ *3-Day Free Trial* (%d hours remaining)\nManage preferences or unlock full access in your dashboard:\n👉 %s/my-alerts/login\n━━━━━━━━━━━━━━━━━━━━", hoursLeft, s.baseURL)
	}

	msg := fmt.Sprintf(
		"*JobyUK.com Job Alert*\n\n"+
			"*%s*\n\n"+
			"*Company:* %s\n"+
			"*Location:* %s\n"+
			"*Salary:* %s\n"+
			"*Type:* %s (%s)\n\n"+
			"*View and apply:*\n%s\n\n"+
			"_You are receiving this alert based on your JobyUK.com job alert preferences. To update your settings, visit %s/my-alerts._%s",
		job.Title,
		job.EmployerName,
		job.LocationName,
		salary,
		job.ContractTypeSimple(),
		job.WorkMode(),
		jobURL,
		s.baseURL,
		trialFooter,
	)

	status := "sent"
	var errMsg string

	err := s.DispatchChannelMessage(ctx, "alerts", sub.PhoneNumber, msg, fmt.Sprintf("Job #%d Alert", job.ID))
	if err != nil {
		status = "failed"
		errMsg = err.Error()
	}

	logEntry := &models.WhatsAppAlertLog{
		SubscriberID: &sub.ID,
		PhoneNumber:  sub.PhoneNumber,
		JobID:        &job.ID,
		JobTitle:     job.Title,
		Company:      job.EmployerName,
		Location:     job.LocationName,
		SalaryText:   salary,
		Status:       status,
		ErrorMessage: errMsg,
	}

	_ = s.repo.LogAlert(ctx, logEntry)
	return err
}

// SendWelcomeWithPIN delivers an encrypted welcome security message with their confidential 6-digit access PIN.
func (s *Service) SendWelcomeWithPIN(ctx context.Context, phone, name, pin, planType string) error {
	normPhone := repository.NormalizePhoneNumber(phone)
	if normPhone == "" {
		return fmt.Errorf("invalid phone number")
	}

	portalURL := s.baseURL + "/my-alerts/login"
	if s.baseURL == "" {
		portalURL = "https://jobyuk.com/my-alerts/login"
	}

	if planType == "trial" || planType == "" {
		msg := fmt.Sprintf(
			"🎉 *Welcome to JobyUK.com WhatsApp Job Alerts!*\n\n"+
				"Hello %s,\n\n"+
				"Your *3-Day Free Trial* is now officially *ACTIVE*! 🚀\n\n"+
				"━━━━━━━━━━━━━━━━━━━━\n"+
				"🔑 *YOUR CONFIDENTIAL LOGIN PIN:*\n"+
				"👉 *%s*\n"+
				"━━━━━━━━━━━━━━━━━━━━\n\n"+
				"Use this 6-digit PIN with your phone number to sign into your dashboard anytime without waiting for OTP codes:\n"+
				"%s\n\n"+
				"Inside your dashboard, you can:\n"+
				"✓ Filter jobs by minimum salary & UK locations\n"+
				"✓ Choose alert frequency (Instant or Daily Digest)\n"+
				"✓ Manage your job alert preferences\n\n"+
				"🔒 _Keep this PIN safe. Matching UK job vacancies will be dispatched directly to your WhatsApp._",
			name, pin, portalURL,
		)
		return s.DispatchChannelMessage(ctx, "billing", normPhone, msg, "3-Day Free Trial Welcome + PIN")
	}

	planLabel := "Monthly Plan (£4.99/mo)"
	switch planType {
	case "6months":
		planLabel = "6-Month Plan (£24.99)"
	case "yearly":
		planLabel = "Annual Plan (£39.99)"
	}

	msg := fmt.Sprintf(
		"*Welcome to JobyUK.com WhatsApp Job Alerts!*\n\n"+
			"Hello %s,\n\n"+
			"Your subscription is active: *%s*\n\n"+
			"━━━━━━━━━━━━━━━━━━━━\n"+
			"*YOUR CONFIDENTIAL LOGIN PIN:*\n"+
			"👉 *%s*\n"+
			"━━━━━━━━━━━━━━━━━━━━\n\n"+
			"Use this 6-digit PIN with your phone number to sign into your dashboard anytime without waiting for OTP codes:\n"+
			"%s\n\n"+
			"_Please keep this PIN safe. Matching job vacancies will be dispatched directly to your WhatsApp._",
		name, planLabel, pin, portalURL,
	)

	return s.DispatchChannelMessage(ctx, "billing", normPhone, msg, "Welcome + Access PIN")
}

// SendTrialWelcomeWithPIN delivers an authentic welcome message for the 3-day free trial on the homepage.
func (s *Service) SendTrialWelcomeWithPIN(ctx context.Context, phone, pin, category, location string) error {
	normPhone := repository.NormalizePhoneNumber(phone)
	if normPhone == "" {
		return fmt.Errorf("invalid phone number")
	}

	portalURL := s.baseURL + "/my-alerts/login"
	if s.baseURL == "" {
		portalURL = "https://jobyuk.com/my-alerts/login"
	}

	msg := fmt.Sprintf(
		"🎉 *Welcome to JobyUK.com WhatsApp Job Alerts!*\n\n"+
			"Your *3-Day Free Trial* is now officially *ACTIVE*! 🚀\n\n"+
			"📌 *Your Trial Details:*\n"+
			"• Sector: %s\n"+
			"• Location: %s\n"+
			"• Trial Validity: *72 Hours*\n\n"+
			"━━━━━━━━━━━━━━━━━━━━\n"+
			"🔑 *YOUR 6-DIGIT DASHBOARD ACCESS PIN:*\n"+
			"👉 *%s*\n"+
			"━━━━━━━━━━━━━━━━━━━━\n\n"+
			"Sign in to your personal dashboard anytime using your phone number and PIN:\n"+
			"👉 %s\n\n"+
			"Inside your dashboard, you can:\n"+
			"✓ Filter by minimum salary & target UK cities\n"+
			"✓ Choose alert frequency (Instant or Daily Digest)\n"+
			"✓ Upgrade to keep alerts after your 3-day trial\n\n"+
			"_We'll dispatch fresh verified UK job openings straight to this chat!_",
		category, location, pin, portalURL,
	)

	return s.DispatchChannelMessage(ctx, "billing", normPhone, msg, "3-Day Free Trial Welcome")
}

// SendPaymentConfirmation sends a transaction receipt message to candidate's WhatsApp via the support channel.
func (s *Service) SendPaymentConfirmation(ctx context.Context, phone, planType, method string, amount float64, expiresAt time.Time) error {
	normPhone := repository.NormalizePhoneNumber(phone)
	if normPhone == "" {
		return fmt.Errorf("invalid phone number")
	}

	planName := "Monthly Plan"
	switch planType {
	case "6months":
		planName = "6-Month Plan"
	case "yearly":
		planName = "Annual Plan (12 Months)"
	}

	methodTitle := "Credit / Debit Card"
	switch method {
	case "apple_pay":
		methodTitle = "Apple Pay"
	case "google_pay":
		methodTitle = "Google Pay"
	case "uk_bank", "open_banking":
		methodTitle = "UK Pay by Bank (Faster Payments)"
	case "paypal":
		methodTitle = "PayPal"
	case "crypto":
		methodTitle = "Cryptocurrency (USDT/BTC)"
	}

	portalURL := s.baseURL + "/my-alerts"

	msg := fmt.Sprintf(
		"🧾 *Payment Receipt — JobyUK.com WhatsApp Alerts*\n\n"+
			"Thank you! Your payment has been processed successfully.\n\n"+
			"Subscription Details:\n"+
			"• Plan: *%s*\n"+
			"• Amount Paid: *£%.2f*\n"+
			"• Method: *%s*\n"+
			"• Status: *Active & Verified*\n"+
			"• Valid Until: *%s*\n\n"+
			"All alert controls and number change privileges are now fully unlocked!\n\n"+
			"Manage preferences anytime:\n%s",
		planName, amount, methodTitle, expiresAt.Format("02 Jan 2006"), portalURL,
	)

	return s.DispatchChannelMessage(ctx, "billing", normPhone, msg, "Payment Confirmation Receipt")
}

// ============================================================================
// WHATSAPP CHANNEL (NEWSLETTER) BROADCAST ENGINE
// ============================================================================

// PostJobToChannel sends a formatted, high-CTR job alert to the official WhatsApp Channel.
func (s *Service) PostJobToChannel(ctx context.Context, job *models.Job, channelInvite string) (*models.WhatsAppChannelPost, error) {
	if job == nil {
		return nil, errors.New("cannot post nil job to channel")
	}
	if channelInvite == "" {
		channelInvite = "0029Vb9iXUMAu3aQOlanBy2m"
	}

	channelURL := "https://whatsapp.com/channel/" + channelInvite
	formattedMessage := FormatChannelJobMessage(job, s.baseURL, channelURL)

	logEntry := &models.WhatsAppChannelPost{
		JobID:         job.ID,
		ChannelInvite: channelInvite,
		JobTitle:      job.Title,
		Company:       job.EmployerName,
		Location:      job.DisplayLocation(),
		SalaryText:    job.FormattedSalary(),
		Status:        "failed",
	}

	// Select best connected WhatsApp session (alerts -> support -> otp -> billing)
	ds := s.alertsSession
	if ds == nil || !ds.IsConnected() {
		if s.supportSession != nil && s.supportSession.IsConnected() {
			ds = s.supportSession
		} else if s.otpSession != nil && s.otpSession.IsConnected() {
			ds = s.otpSession
		} else if s.billingSession != nil && s.billingSession.IsConnected() {
			ds = s.billingSession
		}
	}

	if ds == nil || !ds.IsConnected() {
		logEntry.ErrorMessage = "No active WhatsApp gateway session connected to broadcast to channel"
		_ = s.repo.LogChannelPost(ctx, logEntry)
		return logEntry, errors.New(logEntry.ErrorMessage)
	}

	ds.mu.RLock()
	cli := ds.client
	ds.mu.RUnlock()

	if cli == nil {
		logEntry.ErrorMessage = "WhatsApp client is not initialized"
		_ = s.repo.LogChannelPost(ctx, logEntry)
		return logEntry, errors.New(logEntry.ErrorMessage)
	}

	// 1. Resolve channel metadata via invite code
	newsletter, err := cli.GetNewsletterInfoWithInvite(ctx, channelInvite)
	if err != nil {
		slog.Error("Failed to resolve WhatsApp newsletter with invite code", "invite", channelInvite, "err", err)
		logEntry.ErrorMessage = fmt.Sprintf("Failed to resolve channel with invite '%s': %v", channelInvite, err)
		_ = s.repo.LogChannelPost(ctx, logEntry)
		return logEntry, err
	}

	targetJID := newsletter.ID
	logEntry.ChannelJID = targetJID.String()

	// 2. Ensure following newsletter
	_ = cli.FollowNewsletter(ctx, targetJID)

	// 3. Dispatch message to the newsletter channel
	_, err = cli.SendMessage(ctx, targetJID, &waE2E.Message{
		Conversation: proto.String(formattedMessage),
	})
	if err != nil {
		slog.Error("Failed to deliver job alert to WhatsApp channel", "channel", targetJID, "err", err)
		logEntry.ErrorMessage = fmt.Sprintf("Failed to send message: %v", err)
		_ = s.repo.LogChannelPost(ctx, logEntry)
		return logEntry, err
	}

	logEntry.Status = "posted"
	logEntry.ErrorMessage = ""
	_ = s.repo.LogChannelPost(ctx, logEntry)
	_ = s.repo.IncrementDailyMessages(ctx)

	slog.Info("Successfully posted high-CTR job alert to WhatsApp Channel",
		"job_id", job.ID,
		"title", job.Title,
		"company", job.EmployerName,
		"channel_jid", targetJID.String(),
	)

	return logEntry, nil
}

// FormatChannelJobMessage constructs a user-friendly, high-converting WhatsApp Channel job bulletin.
func FormatChannelJobMessage(job *models.Job, baseURL, channelURL string) string {
	if baseURL == "" {
		baseURL = "https://jobyuk.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	workType := "On-site"
	if job.IsRemote {
		workType = "Remote / Hybrid"
	}

	contract := "Permanent"
	if job.ContractType != "" {
		contract = strings.Title(strings.ToLower(job.ContractType))
	} else if job.JobType != "" {
		contract = strings.Title(strings.ToLower(strings.ReplaceAll(job.JobType, "_", " ")))
	}

	salary := "Competitive UK Salary"
	if s := job.FormattedSalary(); s != "" {
		salary = s
	}

	company := "Verified UK Employer"
	if job.EmployerName != "" {
		company = job.EmployerName
	}

	location := "United Kingdom"
	if loc := job.DisplayLocation(); loc != "" {
		location = loc
	}

	jobURL := fmt.Sprintf("%s/jobs/%s/?utm_source=wa_channel&utm_medium=post&utm_campaign=job_alerts", baseURL, job.Slug)

	return fmt.Sprintf(
`🇬🇧 *NEW UK JOB VACANCY*

💼 *Role*: %s
🏢 *Company*: %s
📍 *Location*: %s (%s)
💰 *Salary*: %s
⚡ *Type*: %s

✨ *Quick Highlights*:
• Verified vacancy actively hiring in the UK
• Fast-track review for early applicants

👉 *View Details & Apply Here*:
%s

──────────────────
📲 *Forward to friends & colleagues seeking work in the UK!*
🌟 *Follow our Channel for daily 20+ top UK jobs*:
%s`,
		job.Title,
		company,
		location,
		workType,
		salary,
		contract,
		jobURL,
		channelURL,
	)
}

