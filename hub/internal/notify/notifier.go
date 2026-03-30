package notify

import (
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wyiu/aerodocs/hub/internal/model"
	"github.com/wyiu/aerodocs/hub/internal/store"
)

const queueSize = 100

type emailJob struct {
	To        string
	UserID    string
	EventType string
	Subject   string
	Body      string
}

// DebounceDelay is how long to wait before sending agent offline notifications.
// If the agent reconnects within this window, the offline notification is cancelled.
var DebounceDelay = 60 * time.Second

// Notifier dispatches email notifications asynchronously via a background worker.
type Notifier struct {
	store    *store.Store
	queue    chan emailJob
	done     chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
	debounce map[string]*time.Timer // keyed by "eventType:serverID"
}

// New creates a Notifier and starts the background worker goroutine.
func New(st *store.Store) *Notifier {
	n := &Notifier{
		store:    st,
		queue:    make(chan emailJob, queueSize),
		done:     make(chan struct{}),
		debounce: make(map[string]*time.Timer),
	}
	n.wg.Add(1)
	go n.worker()
	return n
}

// Close cancels pending debounced notifications, stops the background worker,
// and waits for it to finish.
func (n *Notifier) Close() {
	n.mu.Lock()
	for key, timer := range n.debounce {
		timer.Stop()
		delete(n.debounce, key)
	}
	n.mu.Unlock()
	close(n.queue)
	n.wg.Wait()
	close(n.done)
}

// Notify loads SMTP config, resolves recipients, renders emails, and enqueues jobs.
// Agent offline/online events are debounced: if an agent reconnects within DebounceDelay,
// the offline notification is cancelled and the online notification is suppressed.
func (n *Notifier) Notify(eventType string, context map[string]string) {
	serverID := context["server_id"]

	// Agent online: cancel any pending offline notification for this server
	if eventType == model.NotifyAgentOnline && serverID != "" {
		n.cancelDebounce(model.NotifyAgentOffline + ":" + serverID)
		return // Suppress the online notification if offline was debounced
	}

	// Agent offline: debounce — only send if agent stays offline for DebounceDelay
	if eventType == model.NotifyAgentOffline && serverID != "" {
		n.scheduleDebounced(eventType, context)
		return
	}

	// All other events: send immediately
	n.enqueueNotification(eventType, context)
}

// scheduleDebounced delays sending a notification. If cancelled before the timer
// fires, the notification is never sent.
func (n *Notifier) scheduleDebounced(eventType string, context map[string]string) {
	key := eventType + ":" + context["server_id"]

	n.mu.Lock()
	defer n.mu.Unlock()

	// Cancel any existing timer for this key
	if timer, ok := n.debounce[key]; ok {
		timer.Stop()
	}

	n.debounce[key] = time.AfterFunc(DebounceDelay, func() {
		n.mu.Lock()
		delete(n.debounce, key)
		n.mu.Unlock()
		log.Printf("notify: debounce expired for %s, sending notification", key)
		n.enqueueNotification(eventType, context)
	})
}

// cancelDebounce stops a pending debounced notification.
func (n *Notifier) cancelDebounce(key string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if timer, ok := n.debounce[key]; ok {
		timer.Stop()
		delete(n.debounce, key)
		log.Printf("notify: cancelled debounced notification for %s (agent reconnected)", key)
	}
}

// enqueueNotification resolves recipients and enqueues email jobs.
func (n *Notifier) enqueueNotification(eventType string, context map[string]string) {
	cfg := LoadSMTPConfig(n.store)
	if !cfg.Enabled || cfg.Host == "" {
		return
	}

	recipients, err := n.store.GetEnabledRecipients(eventType)
	if err != nil {
		log.Printf("notify: get enabled recipients for %q: %v", eventType, err)
		return
	}

	subject, body := RenderEmail(eventType, context)

	for _, u := range recipients {
		if u.Email == "" {
			continue
		}
		job := emailJob{
			To:        u.Email,
			UserID:    u.ID,
			EventType: eventType,
			Subject:   subject,
			Body:      body,
		}
		select {
		case n.queue <- job:
		default:
			log.Printf("notify: queue full, dropping notification for user %s event %s", u.ID, eventType)
		}
	}
}

// worker processes queued email jobs until the queue channel is closed.
func (n *Notifier) worker() {
	defer n.wg.Done()

	for job := range n.queue {
		// Reload config each time so we pick up any changes
		cfg := LoadSMTPConfig(n.store)

		err := SendEmail(cfg, job.To, job.Subject, job.Body)

		id := uuid.New().String()
		status := "sent"
		var errMsg *string
		if err != nil {
			status = "failed"
			msg := err.Error()
			errMsg = &msg
			log.Printf("notify: send email to %s failed: %v", job.To, err)
		}

		if logErr := n.store.LogNotification(id, job.UserID, job.EventType, job.Subject, status, errMsg); logErr != nil {
			log.Printf("notify: log notification: %v", logErr)
		}
	}
}

// LoadSMTPConfig reads SMTP settings from the store's _config table.
func LoadSMTPConfig(st *store.Store) model.SMTPConfig {
	get := func(key string) string {
		v, _ := st.GetConfig(key)
		return v
	}

	port := 587
	if p := get("smtp_port"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	tls := false
	if t := get("smtp_tls"); t == "true" || t == "1" {
		tls = true
	}

	enabled := false
	if e := get("smtp_enabled"); e == "true" || e == "1" {
		enabled = true
	}

	return model.SMTPConfig{
		Host:     get("smtp_host"),
		Port:     port,
		Username: get("smtp_username"),
		Password: get("smtp_password"),
		From:     get("smtp_from"),
		TLS:      tls,
		Enabled:  enabled,
	}
}
