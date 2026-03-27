# SonarQube Quality Gates & Version Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Achieve SonarQube zero-issue quality gates (0 vulnerabilities, 0 hotspots, 0 bugs, 0 code smells, <1% duplication, 99% coverage) and align all documentation/CI with actual runtime versions (Node.js 25, Go 1.26.1).

**Architecture:** Seven phases — version alignment, security hotspots, bug fix, code smell remediation, duplication reduction, Go coverage hardening, and frontend test infrastructure + coverage. Each phase is independently committable and produces measurable SonarQube improvement.

**Tech Stack:** Go 1.26.1, Node.js 25, Vitest, React Testing Library, jsdom, SonarQube/SonarCloud

---

## Phase 1: Version Alignment (CI + Documentation)

### Task 1.1: Update CI Workflow Node.js Version

**Files:**
- Modify: `.github/workflows/ci.yml:21`

- [ ] **Step 1: Update node-version from 20 to 25**

```yaml
# Change line 21 from:
          node-version: 20
# To:
          node-version: 25
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: update CI node version from 20 to 25 to match DHI hardened image"
```

### Task 1.2: Update Documentation Version References

**Files:**
- Modify: `README.md` (lines 60, 120-121)
- Modify: `docs/engineering/deployment.md` (lines 5-6)
- Modify: `docs/engineering/development.md` (lines 5-6)
- Modify: `docs/SDD.md` (lines 36-37)
- Modify: `docs/superpowers/plans/2026-03-23-foundation-and-auth.md` (line 9)

- [ ] **Step 1: Update README.md**

Change `Go 1.22+` → `Go 1.26+` and `Node.js 20+` → `Node.js 25+` at:
- Line 60: `| Language | Go 1.26+ |`
- Line 120: `- Go 1.26+`
- Line 121: `- Node.js 25+`

- [ ] **Step 2: Update docs/engineering/deployment.md**

- Line 5: `- Go 1.26+`
- Line 6: `- Node.js 25+`

- [ ] **Step 3: Update docs/engineering/development.md**

- Line 5: `- Go 1.26+`
- Line 6: `- Node.js 25+`

- [ ] **Step 4: Update docs/SDD.md**

- Line 36: `| Hub | Go 1.26+ | Central server...`
- Line 37: `| Agent | Go 1.26+ | Remote binary...`

- [ ] **Step 5: Update docs/superpowers/plans/2026-03-23-foundation-and-auth.md**

- Line 9: Change `Go 1.22+` → `Go 1.26+`

- [ ] **Step 6: Commit**

```bash
git add README.md docs/engineering/deployment.md docs/engineering/development.md docs/SDD.md docs/superpowers/plans/2026-03-23-foundation-and-auth.md
git commit -m "docs: update Go and Node.js version references to match actual (Go 1.26+, Node.js 25+)"
```

---

## Phase 2: Security Hotspot Fixes (6 hotspots → 0)

### Task 2.1: Refactor SQL Query Building to Eliminate Dynamic Query Hotspots

**Context:** All 4 SQL hotspots are false positives — user values use `?` parameterized placeholders. Only structural SQL (`WHERE`, `AND`, `LIMIT %d`, `OFFSET %d`) is dynamic, and integers come from validated Go `int` fields. Fix by extracting a small query-builder helper that makes the safety obvious to SonarQube.

**Files:**
- Create: `hub/internal/store/querybuilder.go`
- Modify: `hub/internal/store/servers.go` (lines 30-68, 100-140, 175-200)
- Modify: `hub/internal/store/audit.go` (lines 35-65)
- Test: `hub/internal/store/querybuilder_test.go`

- [ ] **Step 1: Write the failing test for querybuilder**

```go
// hub/internal/store/querybuilder_test.go
package store

import "testing"

func TestQueryBuilder_Empty(t *testing.T) {
	qb := newQueryBuilder("SELECT * FROM servers")
	query, args := qb.Build()
	if query != "SELECT * FROM servers" {
		t.Fatalf("expected base query, got %q", query)
	}
	if len(args) != 0 {
		t.Fatalf("expected no args, got %d", len(args))
	}
}

func TestQueryBuilder_WithWhere(t *testing.T) {
	qb := newQueryBuilder("SELECT * FROM servers")
	qb.Where("status = ?", "online")
	qb.Where("name LIKE ?", "%test%")
	query, args := qb.Build()

	expected := "SELECT * FROM servers WHERE status = ? AND name LIKE ?"
	if query != expected {
		t.Fatalf("expected %q, got %q", expected, query)
	}
	if len(args) != 2 || args[0] != "online" || args[1] != "%test%" {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestQueryBuilder_WithLimitOffset(t *testing.T) {
	qb := newQueryBuilder("SELECT * FROM servers")
	qb.Where("status = ?", "online")
	qb.OrderBy("created_at DESC")
	qb.Limit(10)
	qb.Offset(20)
	query, args := qb.Build()

	expected := "SELECT * FROM servers WHERE status = ? ORDER BY created_at DESC LIMIT 10 OFFSET 20"
	if query != expected {
		t.Fatalf("expected %q, got %q", expected, query)
	}
	if len(args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(args))
	}
}

func TestQueryBuilder_CountQuery(t *testing.T) {
	qb := newQueryBuilder("SELECT * FROM servers")
	qb.Where("status = ?", "online")
	countQuery, args := qb.CountQuery("servers")

	expected := "SELECT COUNT(*) FROM servers WHERE status = ?"
	if countQuery != expected {
		t.Fatalf("expected %q, got %q", expected, countQuery)
	}
	if len(args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(args))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd hub && go test ./internal/store/ -run TestQueryBuilder -v`
Expected: FAIL — `newQueryBuilder` not defined

- [ ] **Step 3: Implement querybuilder.go**

```go
// hub/internal/store/querybuilder.go
package store

import (
	"fmt"
	"strings"
)

// queryBuilder constructs parameterized SQL queries safely.
// All user-supplied values are passed as args (never interpolated).
// Only structural SQL keywords are appended to the query string.
type queryBuilder struct {
	base     string
	wheres   []string
	args     []interface{}
	orderBy  string
	limit    int
	offset   int
	hasLimit bool
	hasOff   bool
}

func newQueryBuilder(base string) *queryBuilder {
	return &queryBuilder{base: base}
}

// Where adds a parameterized WHERE clause. The condition must use "?" placeholders.
func (qb *queryBuilder) Where(condition string, args ...interface{}) {
	qb.wheres = append(qb.wheres, condition)
	qb.args = append(qb.args, args...)
}

func (qb *queryBuilder) OrderBy(clause string) {
	qb.orderBy = clause
}

func (qb *queryBuilder) Limit(n int) {
	qb.limit = n
	qb.hasLimit = true
}

func (qb *queryBuilder) Offset(n int) {
	qb.offset = n
	qb.hasOff = true
}

// Build returns the final query string and args.
func (qb *queryBuilder) Build() (string, []interface{}) {
	q := qb.base
	if len(qb.wheres) > 0 {
		q += " WHERE " + strings.Join(qb.wheres, " AND ")
	}
	if qb.orderBy != "" {
		q += " ORDER BY " + qb.orderBy
	}
	if qb.hasLimit {
		q += fmt.Sprintf(" LIMIT %d", qb.limit)
	}
	if qb.hasOff {
		q += fmt.Sprintf(" OFFSET %d", qb.offset)
	}
	return q, qb.args
}

// CountQuery returns a SELECT COUNT(*) query with the same WHERE clauses.
func (qb *queryBuilder) CountQuery(table string) (string, []interface{}) {
	q := "SELECT COUNT(*) FROM " + table
	if len(qb.wheres) > 0 {
		q += " WHERE " + strings.Join(qb.wheres, " AND ")
	}
	return q, qb.args
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd hub && go test ./internal/store/ -run TestQueryBuilder -v`
Expected: PASS

- [ ] **Step 5: Refactor servers.go to use queryBuilder**

Replace the manual string concatenation in `ListServers()`, `ListServersForUser()`, and `DeleteServers()` with `queryBuilder` calls. Ensure all existing tests still pass.

- [ ] **Step 6: Refactor audit.go to use queryBuilder**

Replace manual string concatenation in `ListAuditLogs()` with `queryBuilder`.

- [ ] **Step 7: Run all store tests**

Run: `cd hub && go test ./internal/store/ -v`
Expected: All PASS

- [ ] **Step 8: Commit**

```bash
git add hub/internal/store/querybuilder.go hub/internal/store/querybuilder_test.go hub/internal/store/servers.go hub/internal/store/audit.go
git commit -m "refactor: extract queryBuilder to eliminate SonarQube SQL hotspots"
```

### Task 2.2: Fix PRNG Security Hotspot in MermaidDiagram

**Files:**
- Modify: `web/src/pages/server-detail.tsx:135`

- [ ] **Step 1: Replace Math.random() with crypto.randomUUID()**

```tsx
// Change line 135 from:
    const id = `mermaid-${Math.random().toString(36).slice(2, 9)}`
// To:
    const id = `mermaid-${crypto.randomUUID()}`
```

- [ ] **Step 2: Commit**

```bash
git add web/src/pages/server-detail.tsx
git commit -m "fix: replace Math.random with crypto.randomUUID for Mermaid diagram IDs"
```

### Task 2.3: Fix /tmp Writable Directory Security Hotspot

**Files:**
- Modify: `agent/internal/client/client.go` (lines 360-390, `selfCleanup()` method)

- [ ] **Step 1: Replace all hardcoded /tmp paths with os.MkdirTemp**

The `selfCleanup()` method has **multiple** hardcoded `/tmp` paths:
- Line 367: `rm -rf /tmp/aerodocs-dropzone` (in shell heredoc)
- Line 369: `rm -f /tmp/aerodocs-cleanup.sh` (self-delete in heredoc)
- Line 371: `cleanupPath := "/tmp/aerodocs-cleanup.sh"` (Go variable)

Fix approach:
1. Use `os.MkdirTemp("", "aerodocs-cleanup-*")` for the cleanup script
2. Pass the script's own path into the heredoc so the self-delete line matches
3. The `/tmp/aerodocs-dropzone` removal inside the heredoc is fine — it's a known agent directory, not the script location

```go
tmpDir, err := os.MkdirTemp("", "aerodocs-cleanup-*")
if err != nil {
    return
}
scriptPath := filepath.Join(tmpDir, "cleanup.sh")

// Build script content with the actual script path for self-deletion
script := fmt.Sprintf(`#!/bin/sh
sleep 1
rm -rf /tmp/aerodocs-dropzone
/usr/bin/systemctl disable aerodocs-agent 2>/dev/null || true
/usr/bin/systemctl stop aerodocs-agent 2>/dev/null || true
rm -f /usr/local/bin/aerodocs-agent
rm -f /etc/systemd/system/aerodocs-agent.service
rm -rf /etc/aerodocs
rm -f %s
rm -rf %s
`, scriptPath, tmpDir)

os.WriteFile(scriptPath, []byte(script), 0700)
```

Update the `syscall.Exec` call to use `scriptPath` instead of the hardcoded path.

- [ ] **Step 2: Run agent tests**

Run: `cd agent && go test ./... -v`
Expected: All PASS

- [ ] **Step 3: Commit**

```bash
git add agent/internal/client/client.go
git commit -m "fix: use os.MkdirTemp instead of hardcoded /tmp for cleanup script"
```

---

## Phase 3: Bug Fix (1 bug → 0)

### Task 3.1: Add Keyboard Accessibility to Dropzone

**Files:**
- Modify: `web/src/pages/server-detail.tsx:645-654`

- [ ] **Step 1: Add role, tabIndex, and onKeyDown to the dropzone div**

```tsx
// Add these attributes to the div at line 645:
<div
  role="button"
  tabIndex={0}
  onKeyDown={(e) => {
    if ((e.key === 'Enter' || e.key === ' ') && !uploading) {
      e.preventDefault()
      fileInputRef.current?.click()
    }
  }}
  onDrop={handleDrop}
  onDragOver={handleDragOver}
  onDragLeave={handleDragLeave}
  onClick={() => !uploading && fileInputRef.current?.click()}
  // ... rest of props
>
```

- [ ] **Step 2: Commit**

```bash
git add web/src/pages/server-detail.tsx
git commit -m "fix: add keyboard accessibility to dropzone upload area"
```

---

## Phase 4: Code Smell Remediation (106 → 0)

**Code smell breakdown by category (106 total):**

| Category | Count | Tasks |
|----------|-------|-------|
| Duplicate string literals (Go) | ~28 | Task 4.1 |
| Cognitive complexity (Go + TS) | ~7 functions | Task 4.2 |
| Shell `[` → `[[` + stderr | ~12 | Task 4.3 |
| Readonly props (TS) | ~8 | Task 4.4 |
| Nested ternaries (TS) | ~14 | Task 4.5 |
| `replaceAll` / `codePointAt` / `Boolean` (TS) | ~16 | Task 4.5 |
| Array index keys (TS) | ~3 | Task 4.5 |
| Label association (TS) | ~3 | Task 4.5 |
| Negated conditions (TS) | ~4 | Task 4.5 |
| Type assertions / useState / nested templates | ~5 | Task 4.5 |
| Accessibility (`role`, `tabIndex`) | ~1 | Phase 3 (bug) |
| `Math.trunc` / `| 0` | ~2 | Task 4.5 |
| Misc (Do not use Array index keys, etc.) | ~3 | Task 4.5 |
| **Total** | **~106** | |

**Note:** Duplicate string constants in `server` and `store` packages are intentionally separate (same strings, different packages) since Go doesn't allow cross-package unexported constants.

### Task 4.1: Extract Go Error Constants (6 files)

**Files:**
- Create: `hub/internal/server/errors.go`
- Modify: `hub/internal/server/handlers_auth.go`
- Modify: `hub/internal/server/handlers_servers.go`
- Modify: `hub/internal/server/handlers_upload.go`
- Modify: `hub/internal/server/handlers_files.go`
- Modify: `hub/internal/store/users.go`
- Modify: `hub/internal/store/servers.go`

- [ ] **Step 1: Create hub/internal/server/errors.go with shared constants**

```go
package server

// Error message constants shared across handlers.
const (
	errAgentNotConnected  = "agent not connected"
	errInvalidRequestBody = "invalid request body"
	errUnexpectedResponse = "unexpected response type"
	errUserNotFound       = "user not found"
	errServerNotFound     = "server not found"
)
```

- [ ] **Step 2: Replace all string literals in handlers_auth.go, handlers_servers.go, handlers_upload.go, handlers_files.go**

Use find-and-replace for each literal:
- `"agent not connected"` → `errAgentNotConnected`
- `"invalid request body"` → `errInvalidRequestBody`
- `"unexpected response type"` → `errUnexpectedResponse`
- `"user not found"` → `errUserNotFound`
- `"server not found"` → `errServerNotFound`

- [ ] **Step 3: Extract store-level constants**

Add to `hub/internal/store/errors.go`:
```go
package store

const (
	errUserNotFound   = "user not found"
	errServerNotFound = "server not found"
	sqliteTimeFormat  = "2006-01-02 15:04:05"
)
```

Replace all `"user not found"`, `"server not found"`, and `"2006-01-02 15:04:05"` literals in `users.go` and `servers.go`.

- [ ] **Step 4: Run all tests**

Run: `cd hub && go test ./internal/... -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add hub/internal/server/errors.go hub/internal/store/errors.go hub/internal/server/handlers_auth.go hub/internal/server/handlers_servers.go hub/internal/server/handlers_upload.go hub/internal/server/handlers_files.go hub/internal/store/users.go hub/internal/store/servers.go
git commit -m "refactor: extract duplicate string literals into package-level constants"
```

### Task 4.2: Reduce Cognitive Complexity in Go Functions

**Files:**
- Modify: `agent/cmd/aerodocs-agent/main.go`
- Modify: `agent/internal/client/client.go`
- Modify: `hub/internal/grpcserver/handler.go`
- Modify: `hub/internal/server/handlers_upload.go`

- [ ] **Step 1: Refactor agent/cmd/aerodocs-agent/main.go**

Extract `runRegistration()` and `runReconnect()` helper functions from the `main()` function to reduce cognitive complexity from 23 to ≤15.

- [ ] **Step 2: Refactor agent/internal/client/client.go**

Extract each message type handler from the large switch statement in `handleMessage()` into individual methods (e.g., `handleFileListRequest()`, `handleFileReadRequest()`, etc.). Similarly decompose `connectAndStream()`.

- [ ] **Step 3: Refactor hub/internal/grpcserver/handler.go**

Extract message routing logic from the `Connect()` method's inner switch statement into a `routeAgentMessage()` helper.

- [ ] **Step 4: Refactor hub/internal/server/handlers_upload.go**

Extract the chunk-streaming loop from `handleUploadFile()` into a `streamFileToAgent()` helper.

- [ ] **Step 5: Run all tests**

Run: `cd hub && go test ./internal/... -v && cd ../agent && go test ./... -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add agent/cmd/aerodocs-agent/main.go agent/internal/client/client.go hub/internal/grpcserver/handler.go hub/internal/server/handlers_upload.go
git commit -m "refactor: reduce cognitive complexity in Go functions to meet SonarQube threshold"
```

### Task 4.3: Fix Shell Script Issues (install.sh)

**Files:**
- Modify: `hub/static/install.sh`

- [ ] **Step 1: Replace all `[ ]` with `[[ ]]` and add stderr redirect**

Replace every occurrence of `[ ... ]` with `[[ ... ]]` throughout the file. Add `>&2` to the error output at line 117.

- [ ] **Step 2: Commit**

```bash
git add hub/static/install.sh
git commit -m "fix: use bash [[ ]] conditionals and stderr redirect in install.sh"
```

### Task 4.4: Fix Frontend Code Smells — Readonly Props

**Files:**
- Modify: `web/src/pages/server-detail.tsx` (all component props)
- Modify: `web/src/pages/add-server-modal.tsx:13`
- Modify: `web/src/pages/create-user-modal.tsx:10`
- Modify: `web/src/components/logo.tsx:6`
- Modify: `web/src/hooks/use-auth.tsx:16`
- Modify: `web/src/App.tsx:16`

- [ ] **Step 1: Add `Readonly<>` wrapper to all component props**

For every component that accepts props (identified by SonarQube), wrap the props type in `Readonly<{...}>`. Example:

```tsx
// Before:
function AddServerModal({ onClose }: { onClose: () => void }) {
// After:
function AddServerModal({ onClose }: Readonly<{ onClose: () => void }>) {
```

Do this for all flagged components across all files.

- [ ] **Step 2: Commit**

```bash
git add web/src/pages/server-detail.tsx web/src/pages/add-server-modal.tsx web/src/pages/create-user-modal.tsx web/src/components/logo.tsx web/src/hooks/use-auth.tsx web/src/App.tsx
git commit -m "fix: mark React component props as Readonly"
```

### Task 4.5: Fix Frontend Code Smells — Nested Ternaries, Accessibility, Misc

**Files:**
- Modify: `web/src/pages/server-detail.tsx`
- Modify: `web/src/pages/add-server-modal.tsx`
- Modify: `web/src/pages/dashboard.tsx`
- Modify: `web/src/pages/settings.tsx`
- Modify: `web/src/pages/audit-logs.tsx`
- Modify: `web/src/pages/login-totp.tsx`
- Modify: `web/src/pages/setup-totp.tsx`
- Modify: `web/src/lib/avatar.ts`

- [ ] **Step 1: Replace nested ternaries with early returns or helper functions**

For each file, extract nested `a ? b : c ? d : e` into either:
- Early return pattern: `if (isLoading) return <Loading />; if (empty) return <Empty />;`
- Helper render functions: `const renderContent = () => { ... }`

- [ ] **Step 2: Fix replaceAll issues**

In `login-totp.tsx` and `setup-totp.tsx`, `server-detail.tsx`:
```tsx
// Change .replace() with regex to .replaceAll()
.replace(/\D/g, '')  →  .replaceAll(/\D/g, '')
```

- [ ] **Step 3: Fix Boolean arrow functions**

In `login-totp.tsx:34` and `setup-totp.tsx:38`:
```tsx
// Change:
newDigits.every(d => d)
// To:
newDigits.every(Boolean)
```
And the `disabled` props:
```tsx
// Change:
digits.some(d => !d)
// To:
digits.some(d => d === '')
```

- [ ] **Step 4: Fix Array index keys**

In `login-totp.tsx:95`, `setup-totp.tsx:122`, `server-detail.tsx:1050`:
```tsx
// Change: key={i}
// To: key={`digit-${i}`} (for TOTP inputs)
// To: key={`log-${i}`} (for log lines)
```

- [ ] **Step 5: Fix label association**

In `add-server-modal.tsx` and `server-detail.tsx`:
```tsx
// Add htmlFor to labels and id to inputs
<label htmlFor="server-name">Server Name</label>
<input id="server-name" ... />
```

- [ ] **Step 6: Fix Math.trunc and codePointAt in avatar.ts**

```tsx
// Change charCodeAt to codePointAt
ch.charCodeAt(0) → (ch.codePointAt(0) ?? 0)
// Change | 0 to Math.trunc
hash | 0 → Math.trunc(hash)
```

- [ ] **Step 7: Fix nested template literal in dashboard.tsx**

```tsx
// Change:
`/servers${qs ? `?${qs}` : ''}`
// To:
const url = qs ? `/servers?${qs}` : '/servers'
```

- [ ] **Step 8: Fix negated conditions**

Invert `if (!condition) { ... } else { ... }` patterns.

- [ ] **Step 9: Fix useState destructuring in settings.tsx**

Rename `setAvatarColorState` to follow convention or restructure to avoid name collision.

- [ ] **Step 10: Fix unnecessary type assertions in server-detail.tsx**

Remove `as` assertions where the receiver already accepts the original type.

- [ ] **Step 11: Run TypeScript check**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 12: Commit**

```bash
git add web/src/
git commit -m "fix: resolve all SonarQube frontend code smells (ternaries, a11y, types)"
```

---

## Phase 5: Duplication Reduction (3.5% → <1%)

### Task 5.1: Extract Shared TOTP Digit Input Component

**Context:** `login-totp.tsx` (33.9% dup) and `setup-totp.tsx` (27.7% dup) share identical digit input logic: `handleDigitChange`, `handlePaste`, `handleKeyDown`, digit state, and the 6-input rendering. Extract a shared component.

**Files:**
- Create: `web/src/components/totp-digit-input.tsx`
- Modify: `web/src/pages/login-totp.tsx`
- Modify: `web/src/pages/setup-totp.tsx`

- [ ] **Step 1: Create shared TOTPDigitInput component**

```tsx
// web/src/components/totp-digit-input.tsx
import { useState, useRef } from 'react'

export function useTOTPDigits(onComplete: (code: string) => void) {
  const [digits, setDigits] = useState(['', '', '', '', '', ''])
  const inputRefs = useRef<(HTMLInputElement | null)[]>([])

  const handleDigitChange = (index: number, value: string) => {
    if (!/^\d*$/.test(value)) return
    const newDigits = [...digits]
    newDigits[index] = value.slice(-1)
    setDigits(newDigits)
    if (value && index < 5) inputRefs.current[index + 1]?.focus()
    if (newDigits.every(Boolean) && index === 5) onComplete(newDigits.join(''))
  }

  const handlePaste = (e: React.ClipboardEvent) => {
    e.preventDefault()
    const pasted = e.clipboardData.getData('text').replaceAll(/\D/g, '').slice(0, 6)
    if (!pasted) return
    const newDigits = [...digits]
    for (let i = 0; i < pasted.length; i++) newDigits[i] = pasted[i]
    setDigits(newDigits)
    inputRefs.current[pasted.length < 6 ? pasted.length : 5]?.focus()
    if (newDigits.every(Boolean)) onComplete(newDigits.join(''))
  }

  const handleKeyDown = (index: number, e: React.KeyboardEvent) => {
    if (e.key === 'Backspace' && !digits[index] && index > 0) {
      inputRefs.current[index - 1]?.focus()
    }
  }

  const reset = () => {
    setDigits(['', '', '', '', '', ''])
    inputRefs.current[0]?.focus()
  }

  return { digits, inputRefs, handleDigitChange, handlePaste, handleKeyDown, reset }
}

export function TOTPDigitInput({
  digits,
  inputRefs,
  handleDigitChange,
  handlePaste,
  handleKeyDown,
  loading,
}: Readonly<{
  digits: string[]
  inputRefs: React.MutableRefObject<(HTMLInputElement | null)[]>
  handleDigitChange: (index: number, value: string) => void
  handlePaste: (e: React.ClipboardEvent) => void
  handleKeyDown: (index: number, e: React.KeyboardEvent) => void
  loading: boolean
}>) {
  return (
    <div className="flex gap-2 justify-center mb-4">
      {digits.map((digit, i) => (
        <input
          key={`digit-${i}`}
          ref={el => { inputRefs.current[i] = el }}
          type="text"
          inputMode="numeric"
          maxLength={1}
          value={digit}
          onChange={(e) => handleDigitChange(i, e.target.value)}
          onKeyDown={(e) => handleKeyDown(i, e)}
          onPaste={handlePaste}
          className="w-10 h-12 bg-elevated border border-border rounded text-center text-lg font-mono text-text-primary focus:outline-none focus:border-accent"
          autoFocus={i === 0}
          disabled={loading}
        />
      ))}
    </div>
  )
}
```

- [ ] **Step 2: Refactor login-totp.tsx to use shared component**

Remove local digit state/handlers and import from the shared component.

- [ ] **Step 3: Refactor setup-totp.tsx to use shared component**

Same treatment as login-totp.tsx.

- [ ] **Step 4: Run TypeScript check**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add web/src/components/totp-digit-input.tsx web/src/pages/login-totp.tsx web/src/pages/setup-totp.tsx
git commit -m "refactor: extract shared TOTP digit input component to reduce duplication"
```

### Task 5.2: Extract Shared Agent Request Pattern in Hub Handlers

**Context:** `handlers_files.go` (36.3% dup), `handlers_upload.go` (7.2%), `handlers_logs.go` (23.7%), and `handlers_audit.go` (27.8%) share repeated agent connection check + request/response patterns. Extract a helper.

**Files:**
- Create: `hub/internal/server/agent_request.go`
- Modify: `hub/internal/server/handlers_files.go`
- Modify: `hub/internal/server/handlers_upload.go`
- Modify: `hub/internal/server/handlers_logs.go`

- [ ] **Step 1: Create agent_request.go helper**

```go
// hub/internal/server/agent_request.go
package server

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

// requireAgent checks agent connectivity and returns the server ID.
// Returns ("", false) if the response has been written with an error.
func (s *Server) requireAgent(w http.ResponseWriter, r *http.Request) (string, bool) {
	serverID := r.PathValue("id")
	if s.connMgr == nil {
		respondError(w, http.StatusBadGateway, errAgentNotConnected)
		return "", false
	}
	if s.connMgr.GetConn(serverID) == nil {
		respondError(w, http.StatusBadGateway, errAgentNotConnected)
		return "", false
	}
	return serverID, true
}

// sendAndWait sends a message to an agent and waits for a response.
// Returns the raw response message, or writes an error and returns nil.
func (s *Server) sendAndWait(w http.ResponseWriter, serverID string, msg *pb.HubMessage, timeout time.Duration) interface{} {
	requestID := uuid.NewString()
	ch := s.pending.Register(requestID)
	defer s.pending.Remove(requestID)

	if err := s.connMgr.SendToAgent(serverID, msg); err != nil {
		respondError(w, http.StatusBadGateway, errAgentNotConnected)
		return nil
	}

	select {
	case resp := <-ch:
		return resp
	case <-time.After(timeout):
		respondError(w, http.StatusGatewayTimeout, "agent timeout")
		return nil
	}
}
```

- [ ] **Step 2: Refactor handlers_files.go to use helpers**

Replace the duplicated connection-check + send-and-wait pattern with `requireAgent()` and `sendAndWait()`.

- [ ] **Step 3: Refactor handlers_upload.go (deleteDropzone, listDropzone)**

Replace duplicated patterns (keep `handleUploadFile`'s streaming logic as-is since it's different).

- [ ] **Step 4: Refactor handlers_logs.go**

Replace the agent connection check in `handleTailLog()` with `requireAgent()`. Note: `handleTailLog` uses `logSessions` (not `pending`) and SSE streaming, so only the connection check pattern can be extracted — the rest is unique.

- [ ] **Step 5: Run all handler tests**

Run: `cd hub && go test ./internal/server/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add hub/internal/server/agent_request.go hub/internal/server/handlers_files.go hub/internal/server/handlers_upload.go hub/internal/server/handlers_logs.go
git commit -m "refactor: extract agent request helpers to reduce handler duplication"
```

### Task 5.3: Reduce Dashboard and Settings Duplication

**Context:** `dashboard.tsx` (6.1%) and `settings.tsx` (1.7%) have duplicated loading/empty state patterns and `setup.tsx` (9.7%) shares form patterns.

**Files:**
- Modify: `web/src/pages/dashboard.tsx`
- Modify: `web/src/pages/settings.tsx`
- Modify: `web/src/pages/setup.tsx`

- [ ] **Step 1: Extract shared error banner pattern**

If duplicated error banners exist across files, extract into a shared component or ensure each is unique enough to not trigger duplication.

- [ ] **Step 2: Review remaining duplication**

After the TOTP and handler refactoring, re-check if dashboard/settings duplication is still above threshold. Minor duplications in JSX patterns (like loading spinners) may already be below the threshold.

- [ ] **Step 3: Commit if changes needed**

```bash
git add web/src/
git commit -m "refactor: reduce template duplication in dashboard and settings pages"
```

---

## Phase 6: Go Test Coverage Hardening (34% → 99%)

### Task 6.1: Add Tests for handlers_files.go (0% → ~95%)

**Files:**
- Create: `hub/internal/server/handlers_files_test.go`

- [ ] **Step 1: Write tests for handleListFiles, handleReadFile, isPathAllowed, validateRequestPath**

Need to mock the connMgr and pending request infrastructure. Create a mock connMgr that returns predetermined responses. Test cases:
- Missing path parameter → 400
- Path traversal (`..`) → 400
- No agent connected → 502
- Successful file list
- Successful file read
- File too large
- Agent timeout
- Path permission denied (non-admin)
- Path permission allowed (admin)

- [ ] **Step 2: Run tests**

Run: `cd hub && go test ./internal/server/ -run TestHandleListFiles -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add hub/internal/server/handlers_files_test.go
git commit -m "test: add comprehensive tests for file handler endpoints"
```

### Task 6.2: Add Tests for handlers_upload.go (0% → ~95%)

**Files:**
- Create: `hub/internal/server/handlers_upload_test.go`

- [ ] **Step 1: Write tests for handleUploadFile, handleDeleteDropzoneFile, handleListDropzone**

Test cases:
- Upload with no file → 400
- Upload too large → 413
- No agent connected → 502
- Successful upload
- Upload agent timeout
- Delete dropzone file success/failure
- List dropzone empty/populated

- [ ] **Step 2: Run and commit**

### Task 6.3: Add Tests for handlers_logs.go (0% → ~90%)

**Files:**
- Create: `hub/internal/server/handlers_logs_test.go`

- [ ] **Step 1: Write tests for handleTailLog**

Test SSE streaming behavior. Test cases:
- Missing path → 400
- Path traversal → 400
- No agent → 502
- Permission denied
- Successful SSE connection + data streaming

- [ ] **Step 2: Run and commit**

### Task 6.4: Add Tests for handlers_audit.go (0% → 100%)

**Files:**
- Create: `hub/internal/server/handlers_audit_test.go`

- [ ] **Step 1: Write tests for handleListAuditLogs**

Test filter parsing, pagination, default values.

- [ ] **Step 2: Run and commit**

### Task 6.5: Add Tests for handlers_paths.go (0% → ~95%)

**Files:**
- Create: `hub/internal/server/handlers_paths_test.go`

- [ ] **Step 1: Write tests for path handling endpoints**

- [ ] **Step 2: Run and commit**

### Task 6.6: Add Tests for handlers_unregister.go (0% → ~95%)

**Files:**
- Create: `hub/internal/server/handlers_unregister_test.go`

- [ ] **Step 1: Write tests for server unregistration**

- [ ] **Step 2: Run and commit**

### Task 6.7: Improve handler.go Coverage (14.7% → ~90%)

**Files:**
- Modify: `hub/internal/grpcserver/handler_test.go`

- [ ] **Step 1: Add test cases for all message types in the Connect handler**

Cover: heartbeat routing, file list response, file read response, file upload ack, log stream data, file delete response, unregister.

- [ ] **Step 2: Run and commit**

### Task 6.8: Improve client.go Coverage (7.8% → ~85%)

**Files:**
- Modify: `agent/internal/client/client_test.go`

- [ ] **Step 1: Add tests for message handling, reconnection logic, self-cleanup**

Need mock gRPC stream. Test each message handler method after the cognitive complexity refactor.

- [ ] **Step 2: Run and commit**

### Task 6.9: Improve handlers_auth.go Coverage (40.9% → ~95%)

**Files:**
- Modify: `hub/internal/server/handlers_auth_test.go`

- [ ] **Step 1: Add test cases for TOTP setup, TOTP enable, TOTP login, token refresh, edge cases**

Currently only tests registration and auth status. Need:
- Login success/failure
- TOTP flow
- Token refresh
- Invalid body handling
- Password validation

- [ ] **Step 2: Run and commit**

### Task 6.10: Add Tests for respond.go and router.go

**Files:**
- Create: `hub/internal/server/respond_test.go`
- Create: `hub/internal/server/router_test.go`

**Note:** `router.go` already has 100% coverage and `respond.go` has 100% coverage. These may not need new tests but should be verified. If coverage is already complete, skip.

- [ ] **Step 1: Verify existing coverage, add edge case tests if gaps exist**

### Task 6.11: Improve Remaining Go File Coverage

**Files:**
- `hub/cmd/aerodocs/admin.go` (0% → ~90%) — **note: this is in cmd/, not internal/server/**
- `hub/internal/server/server.go` (0% → ~80%)
- `hub/internal/grpcserver/server.go` (0% → ~80%)
- `agent/cmd/aerodocs-agent/main.go` (0% → ~70%)
- `hub/cmd/aerodocs/main.go` (0% → ~70%)
- All other files below 85% — bring up to ≥95%

**Coverage math note:** With 8256 ncloc and a 99% target, we need ~8173 lines covered. The `main.go` files (214 lines combined) contain process-level code (signal handling, `log.Fatal`, `syscall.Exec`) that's inherently hard to unit test. To compensate, every other file must be at or near 100%. `hub/embed.go` (3 lines, embed directive) cannot be unit tested — exclude it from coverage via sonar exclusions if needed.

- [ ] **Step 1: Add tests for each untested file**
- [ ] **Step 2: Run full test suite with coverage**

Run: `cd hub && go test ./internal/... -coverprofile=../coverage-hub.out -covermode=atomic && cd ../agent && go test ./... -coverprofile=../coverage-agent.out -covermode=atomic`

- [ ] **Step 3: Verify coverage**

Run: `go tool cover -func=coverage-hub.out | tail -1` — should show ≥95% total

- [ ] **Step 4: Commit all new Go tests**

```bash
git add hub/ agent/
git commit -m "test: comprehensive Go test coverage to reach 99% target"
```

---

## Phase 7: Frontend Test Infrastructure + Coverage (0% → 99%)

### Task 7.1: Set Up Vitest + React Testing Library

**Files:**
- Modify: `web/package.json`
- Create: `web/vitest.config.ts`
- Create: `web/src/test/setup.ts`
- Modify: `web/tsconfig.app.json`
- Modify: `sonar-project.properties` (already configured for `web/coverage/lcov.info`)
- Modify: `.github/workflows/ci.yml` (add frontend coverage step)

- [ ] **Step 1: Install test dependencies**

```bash
cd web && npm install -D vitest @testing-library/react @testing-library/jest-dom @testing-library/user-event jsdom @vitest/coverage-v8
```

- [ ] **Step 2: Create vitest.config.ts**

```ts
// web/vitest.config.ts
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { '@': path.resolve(__dirname, './src') },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
      reportsDirectory: './coverage',
      include: ['src/**/*.{ts,tsx}'],
      exclude: ['src/main.tsx', 'src/test/**'],
    },
  },
})
```

- [ ] **Step 3: Create test setup file**

```ts
// web/src/test/setup.ts
import '@testing-library/jest-dom/vitest'
```

- [ ] **Step 4: Add test script to package.json**

```json
"scripts": {
  "test": "vitest run",
  "test:watch": "vitest",
  "test:coverage": "vitest run --coverage"
}
```

- [ ] **Step 5: Add frontend test + coverage to CI**

Add to `.github/workflows/ci.yml` after TypeScript check:
```yaml
      - name: Frontend tests with coverage
        run: cd web && npx vitest run --coverage
```

- [ ] **Step 6: Commit**

```bash
git add web/package.json web/package-lock.json web/vitest.config.ts web/src/test/setup.ts web/tsconfig.app.json .github/workflows/ci.yml sonar-project.properties
git commit -m "test: set up Vitest + React Testing Library for frontend coverage"
```

### Task 7.2: Test Utility Files (lib/, hooks/)

**Files:**
- Create: `web/src/lib/__tests__/avatar.test.ts`
- Create: `web/src/lib/__tests__/api.test.ts`
- Create: `web/src/lib/__tests__/auth.test.ts`
- Create: `web/src/lib/__tests__/query-client.test.ts`
- Create: `web/src/hooks/__tests__/use-auth.test.tsx`

- [ ] **Step 1: Write tests for avatar.ts**

Test color generation, hash function, deterministic output for same input.

- [ ] **Step 2: Write tests for api.ts**

Mock fetch, test apiFetch with success/error, token refresh, 401 handling.

- [ ] **Step 3: Write tests for auth.ts**

Test token get/set/clear in localStorage.

- [ ] **Step 4: Write tests for query-client.ts**

Test QueryClient config.

- [ ] **Step 5: Write tests for use-auth.tsx**

Test auth context: login, logout, token storage, initial state.

- [ ] **Step 6: Run and commit**

### Task 7.3: Test Layout Components

**Files:**
- Create: `web/src/layouts/__tests__/auth-layout.test.tsx`
- Create: `web/src/layouts/__tests__/app-shell.test.tsx`

- [ ] **Step 1: Write rendering tests**

Mock router context, test correct rendering of children, navigation links.

- [ ] **Step 2: Run and commit**

### Task 7.4: Test Page Components — Auth Pages

**Files:**
- Create: `web/src/pages/__tests__/login.test.tsx`
- Create: `web/src/pages/__tests__/login-totp.test.tsx`
- Create: `web/src/pages/__tests__/setup.test.tsx`
- Create: `web/src/pages/__tests__/setup-totp.test.tsx`

- [ ] **Step 1: Write tests for login.tsx**

Test form rendering, validation, submit, error handling, redirect on success.

- [ ] **Step 2: Write tests for login-totp.tsx**

Test digit input, paste handling, auto-submit, redirect without token.

- [ ] **Step 3: Write tests for setup.tsx**

Test registration form, password validation, error handling.

- [ ] **Step 4: Write tests for setup-totp.tsx**

Test QR code display, digit input, secret copy, enable flow.

- [ ] **Step 5: Run and commit**

### Task 7.5: Test Page Components — Main App Pages

**Files:**
- Create: `web/src/pages/__tests__/dashboard.test.tsx`
- Create: `web/src/pages/__tests__/server-detail.test.tsx`
- Create: `web/src/pages/__tests__/settings.test.tsx`
- Create: `web/src/pages/__tests__/audit-logs.test.tsx`

- [ ] **Step 1: Write tests for dashboard.tsx**

Test loading state, empty state, server list rendering, search/filter, status indicators.

- [ ] **Step 2: Write tests for server-detail.tsx**

This is the largest component (1634 lines). Test each sub-component:
- MermaidDiagram rendering
- File tree navigation
- Dropzone upload (drag/drop, file selection)
- Log viewer
- Server info display
- Permission handling

- [ ] **Step 3: Write tests for settings.tsx**

Test profile display, user management table, avatar display.

- [ ] **Step 4: Write tests for audit-logs.tsx**

Test log table rendering, filters, pagination.

- [ ] **Step 5: Run and commit**

### Task 7.6: Test Modal Components

**Files:**
- Create: `web/src/pages/__tests__/add-server-modal.test.tsx`
- Create: `web/src/pages/__tests__/create-user-modal.test.tsx`

- [ ] **Step 1: Write tests for add-server-modal.tsx**

Test form, token generation display, close behavior.

- [ ] **Step 2: Write tests for create-user-modal.tsx**

Test user creation form, validation, close behavior.

- [ ] **Step 3: Run and commit**

### Task 7.7: Test Remaining Components

**Files:**
- Create: `web/src/components/__tests__/logo.test.tsx`
- Create: `web/src/components/__tests__/totp-digit-input.test.tsx`
- Create: `web/src/__tests__/App.test.tsx`

- [ ] **Step 1: Write tests for logo.tsx, totp-digit-input.tsx, App.tsx**

- [ ] **Step 2: Run full frontend coverage**

Run: `cd web && npx vitest run --coverage`
Expected: ≥99% coverage

- [ ] **Step 3: Commit**

```bash
git add web/src/
git commit -m "test: comprehensive frontend test suite achieving 99% coverage"
```

---

## Phase 8: Final Verification

### Task 8.1: Full CI Run and SonarQube Verification

- [ ] **Step 1: Run full test suite locally**

```bash
cd hub && go test ./internal/... -coverprofile=../coverage-hub.out -covermode=atomic
cd ../agent && go test ./... -coverprofile=../coverage-agent.out -covermode=atomic
cd ../web && npx vitest run --coverage
```

- [ ] **Step 2: Push to main and verify CI passes**

- [ ] **Step 3: Check SonarQube dashboard**

Verify:
- Vulnerabilities: 0
- Security Hotspots: 0 (or all reviewed)
- Bugs: 0
- Code Smells: 0
- Duplications: < 1%
- Coverage: ≥ 99%

- [ ] **Step 4: Fix any remaining issues**

If SonarQube reports new issues from the added code, fix them incrementally.
