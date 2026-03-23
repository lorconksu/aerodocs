package store_test

import (
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
