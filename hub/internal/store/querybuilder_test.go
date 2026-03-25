package store

import (
	"testing"
)

func TestQueryBuilder_Empty(t *testing.T) {
	qb := newQueryBuilder("SELECT * FROM servers")
	q, args := qb.Build()
	if q != "SELECT * FROM servers" {
		t.Fatalf("expected plain base query, got %q", q)
	}
	if len(args) != 0 {
		t.Fatalf("expected no args, got %v", args)
	}
}

func TestQueryBuilder_SingleWhere(t *testing.T) {
	qb := newQueryBuilder("SELECT * FROM servers")
	qb.Where("status = ?", "online")
	q, args := qb.Build()
	if q != "SELECT * FROM servers WHERE status = ?" {
		t.Fatalf("unexpected query: %q", q)
	}
	if len(args) != 1 || args[0] != "online" {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestQueryBuilder_MultipleWhere(t *testing.T) {
	qb := newQueryBuilder("SELECT * FROM servers")
	qb.Where("status = ?", "online")
	qb.Where("name LIKE ?", "%web%")
	q, args := qb.Build()
	if q != "SELECT * FROM servers WHERE status = ? AND name LIKE ?" {
		t.Fatalf("unexpected query: %q", q)
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
}

func TestQueryBuilder_OrderBy(t *testing.T) {
	qb := newQueryBuilder("SELECT * FROM servers")
	qb.OrderBy("created_at DESC")
	q, _ := qb.Build()
	if q != "SELECT * FROM servers ORDER BY created_at DESC" {
		t.Fatalf("unexpected query: %q", q)
	}
}

func TestQueryBuilder_LimitOffset(t *testing.T) {
	qb := newQueryBuilder("SELECT * FROM servers")
	qb.Limit(10)
	qb.Offset(20)
	q, _ := qb.Build()
	if q != "SELECT * FROM servers LIMIT 10 OFFSET 20" {
		t.Fatalf("unexpected query: %q", q)
	}
}

func TestQueryBuilder_LimitWithoutOffset(t *testing.T) {
	qb := newQueryBuilder("SELECT * FROM servers")
	qb.Limit(5)
	q, _ := qb.Build()
	if q != "SELECT * FROM servers LIMIT 5" {
		t.Fatalf("unexpected query: %q", q)
	}
}

func TestQueryBuilder_OffsetWithoutLimit(t *testing.T) {
	qb := newQueryBuilder("SELECT * FROM servers")
	qb.Offset(10)
	q, _ := qb.Build()
	if q != "SELECT * FROM servers OFFSET 10" {
		t.Fatalf("unexpected query: %q", q)
	}
}

func TestQueryBuilder_Full(t *testing.T) {
	qb := newQueryBuilder("SELECT * FROM servers")
	qb.Where("status = ?", "online")
	qb.OrderBy("created_at DESC")
	qb.Limit(10)
	qb.Offset(5)
	q, args := qb.Build()
	expected := "SELECT * FROM servers WHERE status = ? ORDER BY created_at DESC LIMIT 10 OFFSET 5"
	if q != expected {
		t.Fatalf("expected %q, got %q", expected, q)
	}
	if len(args) != 1 || args[0] != "online" {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestQueryBuilder_CountQuery_NoWhere(t *testing.T) {
	qb := newQueryBuilder("SELECT * FROM audit_logs")
	q, args := qb.CountQuery("audit_logs")
	if q != "SELECT COUNT(*) FROM audit_logs" {
		t.Fatalf("unexpected count query: %q", q)
	}
	if len(args) != 0 {
		t.Fatalf("expected no args, got %v", args)
	}
}

func TestQueryBuilder_CountQuery_WithWhere(t *testing.T) {
	qb := newQueryBuilder("SELECT * FROM audit_logs")
	qb.Where("user_id = ?", "user-1")
	q, args := qb.CountQuery("audit_logs")
	if q != "SELECT COUNT(*) FROM audit_logs WHERE user_id = ?" {
		t.Fatalf("unexpected count query: %q", q)
	}
	if len(args) != 1 || args[0] != "user-1" {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestQueryBuilder_WhereIn(t *testing.T) {
	qb := newQueryBuilder("DELETE FROM servers")
	qb.WhereIn("id", []interface{}{"s1", "s2", "s3"})
	q, args := qb.Build()
	if q != "DELETE FROM servers WHERE id IN (?,?,?)" {
		t.Fatalf("unexpected query: %q", q)
	}
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(args))
	}
}
