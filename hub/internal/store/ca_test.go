package store_test

import (
	"testing"
)

func TestSaveAndLoadCA(t *testing.T) {
	s := testStore(t)

	certDER := []byte("fake-cert-der")
	keyEncrypted := []byte("fake-key-encrypted")

	if err := s.SaveCA(certDER, keyEncrypted); err != nil {
		t.Fatalf("SaveCA: %v", err)
	}

	gotCert, gotKey, err := s.LoadCA()
	if err != nil {
		t.Fatalf("LoadCA: %v", err)
	}
	if string(gotCert) != string(certDER) {
		t.Fatalf("cert mismatch")
	}
	if string(gotKey) != string(keyEncrypted) {
		t.Fatalf("key mismatch")
	}
}

func TestLoadCA_NotFound(t *testing.T) {
	s := testStore(t)
	_, _, err := s.LoadCA()
	if err == nil {
		t.Fatal("expected error for missing CA")
	}
}

func TestSaveCA_Overwrite(t *testing.T) {
	s := testStore(t)

	s.SaveCA([]byte("cert1"), []byte("key1"))
	s.SaveCA([]byte("cert2"), []byte("key2"))

	cert, key, err := s.LoadCA()
	if err != nil {
		t.Fatalf("LoadCA: %v", err)
	}
	if string(cert) != "cert2" || string(key) != "key2" {
		t.Fatal("expected overwritten values")
	}
}
