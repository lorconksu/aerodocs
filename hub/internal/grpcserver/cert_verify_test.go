package grpcserver

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/ca"
	"github.com/wyiu/aerodocs/hub/internal/model"
	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

func TestExtractServerIDFromCert(t *testing.T) {
	cert := &x509.Certificate{
		Subject: pkix.Name{CommonName: "server-42"},
	}
	got := extractServerIDFromCert(cert)
	if got != "server-42" {
		t.Fatalf("expected 'server-42', got '%s'", got)
	}
}

func TestExtractServerIDFromCert_NilCert(t *testing.T) {
	got := extractServerIDFromCert(nil)
	if got != "" {
		t.Fatalf("expected empty string for nil cert, got '%s'", got)
	}
}

func TestHandleCertRenewal(t *testing.T) {
	h, st := testHandler(t)

	// Set up CA
	caCert, caKey, err := ca.GenerateCA()
	if err != nil {
		t.Fatalf("generate CA: %v", err)
	}
	h.caCert = caCert
	h.caKey = caKey

	// Create and register server
	st.CreateServer(&model.Server{ID: "s-cert", Name: "cert-test", Status: "online", Labels: "{}"})
	stream := &mockStream{}
	h.connMgr.Register("s-cert", stream)

	// Generate a CSR
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	csrTemplate := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "s-cert"},
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, csrTemplate, clientKey)
	if err != nil {
		t.Fatalf("create CSR: %v", err)
	}

	req := &pb.CertRenewRequest{Csr: csrDER}
	h.handleCertRenewal("s-cert", stream, req)

	if len(stream.sent) == 0 {
		t.Fatal("expected a CertRenewResponse to be sent")
	}

	resp := stream.sent[0].GetCertRenewResponse()
	if resp == nil {
		t.Fatal("expected CertRenewResponse payload")
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if len(resp.ClientCert) == 0 {
		t.Fatal("expected non-empty client cert")
	}
	if len(resp.CaCert) == 0 {
		t.Fatal("expected non-empty CA cert")
	}

	// Verify the returned certificate
	clientCert, err := x509.ParseCertificate(resp.ClientCert)
	if err != nil {
		t.Fatalf("parse returned client cert: %v", err)
	}
	if clientCert.Subject.CommonName != "s-cert" {
		t.Fatalf("expected CN 's-cert', got '%s'", clientCert.Subject.CommonName)
	}
	if time.Until(clientCert.NotAfter) < 11*time.Hour {
		t.Fatal("expected cert validity of ~12 hours")
	}
}

func TestHandleCertRenewal_NoCA(t *testing.T) {
	h, st := testHandler(t)
	// No CA configured (caCert and caKey are nil)

	st.CreateServer(&model.Server{ID: "s-noca", Name: "noca-test", Status: "online", Labels: "{}"})
	stream := &mockStream{}
	h.connMgr.Register("s-noca", stream)

	req := &pb.CertRenewRequest{Csr: []byte("fake-csr")}
	h.handleCertRenewal("s-noca", stream, req)

	if len(stream.sent) == 0 {
		t.Fatal("expected a CertRenewResponse to be sent")
	}
	resp := stream.sent[0].GetCertRenewResponse()
	if resp == nil {
		t.Fatal("expected CertRenewResponse payload")
	}
	if resp.Error != "CA not configured" {
		t.Fatalf("expected 'CA not configured' error, got '%s'", resp.Error)
	}
}

func TestHandleCertRenewal_InvalidCSR(t *testing.T) {
	h, st := testHandler(t)

	caCert, caKey, err := ca.GenerateCA()
	if err != nil {
		t.Fatalf("generate CA: %v", err)
	}
	h.caCert = caCert
	h.caKey = caKey

	st.CreateServer(&model.Server{ID: "s-badcsr", Name: "badcsr-test", Status: "online", Labels: "{}"})
	stream := &mockStream{}
	h.connMgr.Register("s-badcsr", stream)

	req := &pb.CertRenewRequest{Csr: []byte("not-a-valid-csr")}
	h.handleCertRenewal("s-badcsr", stream, req)

	if len(stream.sent) == 0 {
		t.Fatal("expected a CertRenewResponse to be sent")
	}
	resp := stream.sent[0].GetCertRenewResponse()
	if resp == nil {
		t.Fatal("expected CertRenewResponse payload")
	}
	if resp.Error == "" {
		t.Fatal("expected error for invalid CSR")
	}
}

func TestRouteAgentMessage_CertRenewRequest(t *testing.T) {
	h, st := testHandler(t)

	caCert, caKey, err := ca.GenerateCA()
	if err != nil {
		t.Fatalf("generate CA: %v", err)
	}
	h.caCert = caCert
	h.caKey = caKey

	st.CreateServer(&model.Server{ID: "s-route-cert", Name: "route-cert", Status: "online", Labels: "{}"})
	stream := &mockStream{}
	h.connMgr.Register("s-route-cert", stream)

	// Generate a valid CSR
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "s-route-cert"},
	}, clientKey)
	if err != nil {
		t.Fatalf("create CSR: %v", err)
	}

	msg := &pb.AgentMessage{
		Payload: &pb.AgentMessage_CertRenewRequest{
			CertRenewRequest: &pb.CertRenewRequest{Csr: csrDER},
		},
	}
	err = h.routeAgentMessage("s-route-cert", stream, msg)
	if err != nil {
		t.Fatalf("route cert renew request: %v", err)
	}

	if len(stream.sent) == 0 {
		t.Fatal("expected CertRenewResponse to be sent via routing")
	}
	resp := stream.sent[0].GetCertRenewResponse()
	if resp == nil {
		t.Fatal("expected CertRenewResponse payload")
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
}
