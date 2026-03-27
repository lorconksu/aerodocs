# Integration, Smoke & E2E Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add integration tests (agent↔hub gRPC), smoke tests (container health), and E2E tests (Playwright browser) to achieve comprehensive test coverage beyond unit tests.

**Architecture:** Three test levels. Integration tests live inside `hub/internal/integration/` (to bypass Go's `internal` package import restriction) and use an in-process gRPC server + SQLite :memory:. Smoke tests in `tests/smoke/` build and run the Docker container, then curl health endpoints. E2E tests in `tests/e2e/` run Playwright against the containerized app. All run in CI. The Dockerfile uses build args so CI can override DHI private images with public ones.

**Tech Stack:** Go 1.26.1, gRPC, SQLite, Docker, Playwright, GitHub Actions

---

## File Structure

```
hub/internal/integration/         # Inside hub module — can import hub/internal/*
├── testhelpers.go                # Start hub+gRPC in-process, create servers, get tokens
├── agent_connect_test.go         # Agent registration + heartbeat
├── agent_filelist_test.go        # Full gRPC round-trip: HTTP → hub → agent → response
├── agent_fileread_test.go        # File read through gRPC
├── agent_upload_test.go          # Upload + dropzone through gRPC
├── agent_unregister_test.go      # Unregister flow
└── auth_flow_test.go             # Register → TOTP → login → refresh (HTTP only)

tests/smoke/
└── smoke_test.sh                 # Docker build + run + curl health checks

tests/e2e/
├── package.json
├── playwright.config.ts
├── global-setup.ts               # Start Docker container
├── global-teardown.ts            # Stop Docker container
├── tsconfig.json
└── tests/
    ├── 01-setup-flow.spec.ts     # Create admin + TOTP (must run first)
    ├── 02-login.spec.ts          # Login + TOTP + logout
    ├── 03-dashboard.spec.ts      # Dashboard + add server modal
    └── 04-settings.spec.ts       # Profile + user management
```

**Key design decisions:**
- Integration tests in `hub/internal/integration/` — Go's `internal` restriction prevents external modules from importing internal packages. By placing tests inside the hub module tree, they can freely import `store`, `connmgr`, `grpcserver`, `server`, etc.
- Agent is added as a dependency to `hub/go.mod` with a `replace` directive pointing to `../agent`.
- Dockerfile uses `ARG` for base images — CI overrides with `--build-arg` instead of maintaining a separate `Dockerfile.ci`.
- E2E specs prefixed with numbers to enforce execution order.

---

## Phase 1: Dockerfile Build Args (prerequisite for smoke/E2E)

### Task 1: Add build args to Dockerfile for CI compatibility

**Context:** The Dockerfile uses `dhi.io` private registry images. CI can't pull from there. Instead of a separate `Dockerfile.ci`, add `ARG` directives so CI can override base images.

**Files:**
- Modify: `Dockerfile`

- [ ] **Step 1: Add ARG directives at the top of each stage**

Change the FROM lines:
```dockerfile
ARG NODE_IMAGE=dhi.io/node:25-debian13-dev
ARG GO_IMAGE=dhi.io/golang:1-debian13-dev

FROM ${NODE_IMAGE} AS frontend
...
FROM ${GO_IMAGE} AS backend
...
```

The rest of the Dockerfile stays identical. In CI, pass:
```bash
docker build --build-arg NODE_IMAGE=node:25-slim --build-arg GO_IMAGE=golang:1.26-bookworm .
```

- [ ] **Step 2: Verify production build still works (no args = DHI images)**

```bash
docker build -t aerodocs:test .
```

- [ ] **Step 3: Verify CI build works (with args = public images)**

```bash
docker build --build-arg NODE_IMAGE=node:25-slim --build-arg GO_IMAGE=golang:1.26-bookworm -t aerodocs:ci-test .
```

Note: `golang:1.26-bookworm` needs `gcc libc6-dev` installed for CGO (SQLite). Add to the backend stage if not present:
```dockerfile
FROM ${GO_IMAGE} AS backend
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev && rm -rf /var/lib/apt/lists/* || true
```

- [ ] **Step 4: Commit**

```bash
git add Dockerfile
git commit -m "ci: add build args to Dockerfile for CI-compatible base images"
```

---

## Phase 2: Integration Tests

### Task 2: Set up integration test package inside hub module

**Files:**
- Modify: `hub/go.mod` (add agent dependency)
- Create: `hub/internal/integration/testhelpers.go`

- [ ] **Step 1: Add agent module as dependency to hub**

```bash
cd hub
go mod edit -require github.com/wyiu/aerodocs/agent@v0.0.0
go mod edit -replace github.com/wyiu/aerodocs/agent=../agent
go mod tidy
```

- [ ] **Step 2: Create testhelpers.go**

Provides `StartHarness(t)` that starts real gRPC + HTTP servers in-process with SQLite :memory:. Captures server startup errors. Cleans up both servers on test completion.

Key points:
- `gs.Start()` returns `error` — wrap in goroutine that sends to errCh
- `hs.Start()` returns `error` — same treatment
- Cleanup: `gs.Stop()` + `hs.Shutdown(ctx)` + `st.Close()`
- Uses `freePort(t)` to find available TCP ports
- Uses `waitForPort(t, port)` with 5s timeout
- `server.Config` should set `IsDev: true` (no embedded frontend needed for API tests)

- [ ] **Step 3: Verify it compiles**

```bash
cd hub && go build ./internal/integration/...
```

- [ ] **Step 4: Commit**

```bash
git add hub/go.mod hub/go.sum hub/internal/integration/testhelpers.go
git commit -m "test: add integration test harness inside hub module"
```

### Task 3: Agent connect + registration integration test

**Files:**
- Create: `hub/internal/integration/agent_connect_test.go`

- [ ] **Step 1: Write the test**

Test flow:
1. Start harness
2. Create a server via the store with a registration token (use HTTP API: register admin → create server)
3. Create an agent `client.Client` pointing to the harness's gRPC address with the registration token
4. Run `client.Run(ctx)` in a goroutine with cancellable context
5. Poll `connMgr.ActiveServerIDs()` until agent appears (timeout 5s)
6. Verify server status is "online" in store
7. Wait 12s for a heartbeat, verify `last_seen_at` updated
8. Cancel context → verify agent disconnects
9. Run heartbeat sweep → verify server goes "offline"

- [ ] **Step 2: Run**

```bash
cd hub && go test ./internal/integration/ -run TestAgentConnect -v -timeout 30s
```

- [ ] **Step 3: Commit**

```bash
git add hub/internal/integration/agent_connect_test.go
git commit -m "test: add agent connection + registration integration test"
```

### Task 4: Agent file list integration test

**Files:**
- Create: `hub/internal/integration/agent_filelist_test.go`

- [ ] **Step 1: Write the test**

Full round-trip: HTTP → hub → gRPC → agent → filebrowser → response → HTTP:
1. Start harness, register agent, wait for connection
2. Create a temp dir with a known file
3. HTTP `GET /api/servers/{id}/files?path=<tempdir>` with admin auth token
4. Verify response JSON contains the file listing
5. This exercises: HTTP handler → `sendAgentRequest` → pending → connMgr → gRPC stream → agent `handleFileListRequest` → `filebrowser.ListDir` → response back through gRPC → `pending.Deliver` → HTTP response

- [ ] **Step 2: Run and commit**

### Task 5: Agent file read integration test

**Files:**
- Create: `hub/internal/integration/agent_fileread_test.go`

- [ ] **Step 1: Write the test**

1. Create temp file with known content ("hello integration test")
2. HTTP `GET /api/servers/{id}/files/read?path=<tempfile>` with admin auth
3. Verify response contains base64-encoded content matching the file

- [ ] **Step 2: Run and commit**

### Task 6: Agent file upload integration test

**Files:**
- Create: `hub/internal/integration/agent_upload_test.go`

- [ ] **Step 1: Write the test**

1. Start harness, register agent
2. HTTP `POST /api/servers/{id}/upload` with multipart file body
3. Verify file appears in agent's dropzone (`/tmp/aerodocs-dropzone/`)
4. HTTP `GET /api/servers/{id}/dropzone` → verify file listed
5. HTTP `DELETE /api/servers/{id}/dropzone?filename=...` → verify removed

- [ ] **Step 2: Run and commit**

### Task 7: Agent unregister integration test

**Files:**
- Create: `hub/internal/integration/agent_unregister_test.go`

- [ ] **Step 1: Write the test**

1. Start harness, register agent, wait for connection
2. HTTP `DELETE /api/servers/{id}/unregister` (admin auth)
3. Verify hub sends UnregisterRequest → agent receives it
4. Verify agent disconnects (no longer in `connMgr.ActiveServerIDs()`)
5. Verify server deleted from store

- [ ] **Step 2: Run and commit**

### Task 8: Full auth flow integration test

**Files:**
- Create: `hub/internal/integration/auth_flow_test.go`

- [ ] **Step 1: Write the test**

Complete auth lifecycle via real HTTP (no mocks):
1. `GET /api/auth/status` → `{"initialized": false}`
2. `POST /api/auth/register` → get setup_token
3. `POST /api/auth/totp/setup` (with setup_token header) → get secret + qr_url
4. Generate valid TOTP code using `pquerna/otp/totp` from the secret
5. `POST /api/auth/totp/enable` → get access_token + refresh_token
6. `GET /api/auth/status` → `{"initialized": true}`
7. `POST /api/auth/login` → get totp_token (TOTP required)
8. `POST /api/auth/login/totp` → get tokens
9. `POST /api/auth/refresh` → get new tokens
10. `GET /api/auth/me` → get user profile
11. `PUT /api/auth/password` → change password
12. Login again with new password → success

- [ ] **Step 2: Run and commit**

### Task 9: Add integration tests to CI

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Add integration job**

```yaml
  integration:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: hub/go.mod
          cache-dependency-path: |
            hub/go.sum
            agent/go.sum
      - name: Integration tests
        run: cd hub && go test ./internal/integration/ -v -timeout 120s
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add integration test job to pipeline"
```

---

## Phase 3: Smoke Tests

### Task 10: Create smoke test script

**Files:**
- Create: `tests/smoke/smoke_test.sh`

- [ ] **Step 1: Write the script**

Key features:
- `cd "$(dirname "$0")/../.."` at the top to ensure repo-root CWD
- Uses `DOCKERFILE=Dockerfile` (overridable)
- Builds image, starts container on random high ports (18081/19090)
- `trap cleanup EXIT` for reliable teardown
- 7 tests: auth status JSON, not initialized, SPA serves HTML, register user, initialized after register, no container restarts, no panic/fatal in logs
- All assertions use `curl -sf` + `python3 -c` for JSON checks

- [ ] **Step 2: Make executable and commit**

```bash
chmod +x tests/smoke/smoke_test.sh
git add tests/smoke/smoke_test.sh
git commit -m "test: add smoke test script for containerized deployment"
```

### Task 11: Add smoke tests to CI

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Add smoke job**

```yaml
  smoke:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run smoke tests
        run: bash tests/smoke/smoke_test.sh
        env:
          BUILD_ARGS: "--build-arg NODE_IMAGE=node:25-slim --build-arg GO_IMAGE=golang:1.26-bookworm"
```

The smoke script passes `$BUILD_ARGS` to `docker build` if set.

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add smoke test job to pipeline"
```

---

## Phase 4: E2E Tests (Playwright)

### Task 12: Set up Playwright project

**Files:**
- Create: `tests/e2e/package.json`
- Create: `tests/e2e/playwright.config.ts`
- Create: `tests/e2e/global-setup.ts`
- Create: `tests/e2e/global-teardown.ts`
- Create: `tests/e2e/tsconfig.json`

- [ ] **Step 1: Create package.json, config, setup/teardown**

Config: `fullyParallel: false`, `testDir: './tests'`, `baseURL` from env or `http://localhost:18081`.

global-setup.ts: Use `spawnSync` (not `exec`) for Docker commands. Resolve Dockerfile path from `__dirname` (`path.resolve(__dirname, '../../Dockerfile')`). Wait for `/api/auth/status` to respond.

global-teardown.ts: `spawnSync('docker', ['rm', '-f', 'aerodocs-e2e'])`.

- [ ] **Step 2: Install and verify**

```bash
cd tests/e2e && npm install && npx playwright install chromium
```

- [ ] **Step 3: Commit**

```bash
git add tests/e2e/
git commit -m "test: set up Playwright E2E infrastructure"
```

### Task 13: E2E — Setup flow test (`01-setup-flow.spec.ts`)

- [ ] **Step 1: Write test**

First-time setup: navigate to `/` → redirects to `/setup` → fill form → submit → TOTP setup page → read secret from `<code>` element → generate TOTP code with `otplib` → fill 6 digit inputs → verify redirect to dashboard.

- [ ] **Step 2: Commit**

### Task 14: E2E — Login test (`02-login.spec.ts`)

- [ ] **Step 1: Write test**

Login with admin credentials → TOTP → dashboard loads → logout → redirected to `/login`.

- [ ] **Step 2: Commit**

### Task 15: E2E — Dashboard test (`03-dashboard.spec.ts`)

- [ ] **Step 1: Write test**

Authenticated: dashboard loads → empty state → "Add Server" button → modal opens → token visible → close modal.

- [ ] **Step 2: Commit**

### Task 16: E2E — Settings test (`04-settings.spec.ts`)

- [ ] **Step 1: Write test**

Authenticated: navigate to `/settings` → profile tab → username/email visible → users tab → admin in table.

- [ ] **Step 2: Commit**

### Task 17: Add E2E tests to CI

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Add E2E job**

```yaml
  e2e:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 25
      - name: Install Playwright
        run: cd tests/e2e && npm ci && npx playwright install --with-deps chromium
      - name: Build Docker image
        run: docker build --build-arg NODE_IMAGE=node:25-slim --build-arg GO_IMAGE=golang:1.26-bookworm -t aerodocs-e2e:test .
      - name: Run E2E tests
        run: cd tests/e2e && E2E_IMAGE=aerodocs-e2e:test npx playwright test
      - name: Upload artifacts on failure
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: playwright-report
          path: tests/e2e/test-results/
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add E2E test job with Playwright to pipeline"
```
