package notify

import (
	"strings"
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func TestRenderEmail_AllEventTypes(t *testing.T) {
	tests := []struct {
		eventType string
		context   map[string]string
	}{
		{
			eventType: model.NotifyAgentOffline,
			context:   map[string]string{"server_name": "web-01"},
		},
		{
			eventType: model.NotifyAgentOnline,
			context:   map[string]string{"server_name": "web-01"},
		},
		{
			eventType: model.NotifyAgentRegistered,
			context:   map[string]string{"server_name": "db-02"},
		},
		{
			eventType: model.NotifyLoginFailed,
			context:   map[string]string{"username": "alice", "ip": "1.2.3.4"},
		},
		{
			eventType: model.NotifyUserCreated,
			context:   map[string]string{"username": "bob"},
		},
		{
			eventType: model.NotifyTOTPChanged,
			context:   map[string]string{"username": "carol", "detail": "TOTP enabled"},
		},
		{
			eventType: model.NotifyPasswordChanged,
			context:   map[string]string{"username": "dave"},
		},
		{
			eventType: model.NotifyFileUploaded,
			context:   map[string]string{"filename": "report.pdf", "server_name": "files-01", "uploader": "eve"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			subject, body := RenderEmail(tt.eventType, tt.context)
			if subject == "" {
				t.Errorf("expected non-empty subject for event %q", tt.eventType)
			}
			if body == "" {
				t.Errorf("expected non-empty body for event %q", tt.eventType)
			}
			if !strings.HasPrefix(subject, "[AeroDocs] ") {
				t.Errorf("subject should start with '[AeroDocs] ', got: %q", subject)
			}
		})
	}
}

func TestRenderEmail_PlaceholderSubstitution(t *testing.T) {
	subject, body := RenderEmail(model.NotifyAgentOffline, map[string]string{
		"server_name": "prod-server-01",
	})

	if !strings.Contains(subject, "prod-server-01") {
		t.Errorf("expected subject to contain 'prod-server-01', got: %q", subject)
	}
	if !strings.Contains(body, "prod-server-01") {
		t.Errorf("expected body to contain 'prod-server-01', got: %q", body)
	}
	if strings.Contains(body, "{{server_name}}") {
		t.Errorf("expected body to have no unreplaced placeholders, got: %q", body)
	}
}

func TestRenderEmail_MultiPlaceholder(t *testing.T) {
	subject, body := RenderEmail(model.NotifyLoginFailed, map[string]string{
		"username": "mallory",
		"ip":       "10.0.0.1",
	})

	if !strings.Contains(subject, "[AeroDocs]") {
		t.Errorf("subject missing prefix, got: %q", subject)
	}
	if !strings.Contains(body, "mallory") {
		t.Errorf("expected body to contain username 'mallory', got: %q", body)
	}
	if !strings.Contains(body, "10.0.0.1") {
		t.Errorf("expected body to contain IP '10.0.0.1', got: %q", body)
	}
}

func TestRenderEmail_UnknownEventFallback(t *testing.T) {
	subject, body := RenderEmail("unknown.event.type", nil)

	if subject == "" {
		t.Error("expected non-empty subject for unknown event")
	}
	if body == "" {
		t.Error("expected non-empty body for unknown event")
	}
	if !strings.HasPrefix(subject, "[AeroDocs] ") {
		t.Errorf("subject should start with '[AeroDocs] ', got: %q", subject)
	}
	// The fallback should mention the event type somewhere
	if !strings.Contains(body, "unknown.event.type") {
		t.Errorf("expected fallback body to contain event type, got: %q", body)
	}
}

func TestRenderEmail_UnknownEventFallbackWithContext(t *testing.T) {
	subject, body := RenderEmail("custom.event", map[string]string{"extra": "value"})

	if subject == "" {
		t.Error("expected non-empty subject")
	}
	if body == "" {
		t.Error("expected non-empty body")
	}
	if strings.Contains(subject, "{{") {
		t.Errorf("subject has unreplaced placeholders: %q", subject)
	}
}
