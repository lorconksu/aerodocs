package notify

import (
	"log"
	"strconv"
	"sync"

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

// Notifier dispatches email notifications asynchronously via a background worker.
type Notifier struct {
	store *store.Store
	queue chan emailJob
	done  chan struct{}
	wg    sync.WaitGroup
}

// New creates a Notifier and starts the background worker goroutine.
func New(st *store.Store) *Notifier {
	n := &Notifier{
		store: st,
		queue: make(chan emailJob, queueSize),
		done:  make(chan struct{}),
	}
	n.wg.Add(1)
	go n.worker()
	return n
}

// Close stops the background worker and waits for it to finish.
func (n *Notifier) Close() {
	close(n.queue)
	n.wg.Wait()
	close(n.done)
}

// Notify loads SMTP config, resolves recipients, renders emails, and enqueues jobs.
// Jobs are dropped (non-blocking) if the queue is full.
func (n *Notifier) Notify(eventType string, context map[string]string) {
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
	cfg := LoadSMTPConfig(n.store)

	for job := range n.queue {
		// Reload config each time so we pick up any changes
		cfg = LoadSMTPConfig(n.store)

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
	_ = cfg // suppress unused warning after loop ends
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
