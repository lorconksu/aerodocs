package store_test

import "testing"

func TestConfigGetSet(t *testing.T) {
	s := testStore(t)

	// Set a value
	if err := s.SetConfig("jwt_key", "test-secret"); err != nil {
		t.Fatalf("set config: %v", err)
	}

	// Get the value
	val, err := s.GetConfig("jwt_key")
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if val != "test-secret" {
		t.Fatalf("expected 'test-secret', got '%s'", val)
	}
}

func TestConfigGetMissing(t *testing.T) {
	s := testStore(t)

	_, err := s.GetConfig("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}
