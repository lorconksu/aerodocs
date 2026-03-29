package notify

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

// mockSMTPServer handles a minimal SMTP conversation on ln,
// then sends all received data to the received channel.
func mockSMTPServer(t *testing.T, ln net.Listener, received chan<- string) {
	t.Helper()
	conn, err := ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	fmt.Fprintf(conn, "220 localhost ESMTP\r\n")
	buf := make([]byte, 4096)
	var allData string
	for {
		n, err := conn.Read(buf)
		if err != nil {
			break
		}
		data := string(buf[:n])
		allData += data
		if strings.HasPrefix(data, "EHLO") || strings.HasPrefix(data, "HELO") {
			fmt.Fprintf(conn, "250-localhost\r\n250 OK\r\n")
		} else if strings.HasPrefix(data, "MAIL FROM") {
			fmt.Fprintf(conn, "250 OK\r\n")
		} else if strings.HasPrefix(data, "RCPT TO") {
			fmt.Fprintf(conn, "250 OK\r\n")
		} else if strings.HasPrefix(data, "DATA") {
			fmt.Fprintf(conn, "354 Send data\r\n")
		} else if strings.Contains(data, "\r\n.\r\n") {
			fmt.Fprintf(conn, "250 OK\r\n")
		} else if strings.HasPrefix(data, "QUIT") {
			fmt.Fprintf(conn, "221 Bye\r\n")
			break
		}
	}
	received <- allData
}

func TestSendEmail_Disabled(t *testing.T) {
	cfg := model.SMTPConfig{
		Host:    "localhost",
		Port:    25,
		Enabled: false,
	}
	err := SendEmail(cfg, "to@example.com", "Subject", "Body")
	if err != nil {
		t.Errorf("expected nil error when disabled, got: %v", err)
	}
}

func TestSendEmail_PlainSMTP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer ln.Close()

	received := make(chan string, 1)
	go mockSMTPServer(t, ln, received)

	addr := ln.Addr().(*net.TCPAddr)
	cfg := model.SMTPConfig{
		Host:    "127.0.0.1",
		Port:    addr.Port,
		From:    "sender@example.com",
		Enabled: true,
		TLS:     false,
	}

	err = SendEmail(cfg, "recipient@example.com", "[AeroDocs] Test Subject", "Hello from AeroDocs.")
	if err != nil {
		t.Fatalf("SendEmail returned error: %v", err)
	}

	select {
	case data := <-received:
		if !strings.Contains(data, "MAIL FROM") {
			t.Errorf("expected MAIL FROM in SMTP exchange, got: %q", data)
		}
		if !strings.Contains(data, "RCPT TO") {
			t.Errorf("expected RCPT TO in SMTP exchange, got: %q", data)
		}
	case <-time.After(3 * time.Second):
		t.Error("timeout waiting for mock SMTP server to receive data")
	}
}

func TestSendEmail_MessageContents(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer ln.Close()

	received := make(chan string, 1)
	go mockSMTPServer(t, ln, received)

	addr := ln.Addr().(*net.TCPAddr)
	cfg := model.SMTPConfig{
		Host:    "127.0.0.1",
		Port:    addr.Port,
		From:    "noreply@aerodocs.local",
		Enabled: true,
		TLS:     false,
	}

	subject, body := RenderEmail(model.NotifyAgentOffline, map[string]string{
		"server_name": "test-server",
	})

	err = SendEmail(cfg, "admin@example.com", subject, body)
	if err != nil {
		t.Fatalf("SendEmail returned error: %v", err)
	}

	select {
	case data := <-received:
		if !strings.Contains(data, "test-server") {
			t.Errorf("expected message to contain 'test-server', got: %q", data)
		}
		if !strings.Contains(data, "[AeroDocs]") {
			t.Errorf("expected message to contain '[AeroDocs]', got: %q", data)
		}
	case <-time.After(3 * time.Second):
		t.Error("timeout waiting for mock SMTP server to receive data")
	}
}

func TestBuildMessage(t *testing.T) {
	msg := buildMessage("from@example.com", "to@example.com", "Test Subject", "Test body content.")

	if !strings.Contains(msg, "From: from@example.com") {
		t.Errorf("missing From header in message: %q", msg)
	}
	if !strings.Contains(msg, "To: to@example.com") {
		t.Errorf("missing To header in message: %q", msg)
	}
	if !strings.Contains(msg, "Subject: Test Subject") {
		t.Errorf("missing Subject header in message: %q", msg)
	}
	if !strings.Contains(msg, "MIME-Version: 1.0") {
		t.Errorf("missing MIME-Version header in message: %q", msg)
	}
	if !strings.Contains(msg, "Content-Type: text/plain") {
		t.Errorf("missing Content-Type header in message: %q", msg)
	}
	if !strings.Contains(msg, "Test body content.") {
		t.Errorf("missing body content in message: %q", msg)
	}
}
