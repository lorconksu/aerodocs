# SSDF v1.1 Assessment

> **TL;DR**
> - **What:** Assessment of AeroDocs development practices against NIST SSDF v1.1 (SP 800-218)
> - **Who:** Security auditors, procurement reviewers, and contributors evaluating development security
> - **Why:** Demonstrates adherence to federal secure development standards as required by EO 14028
> - **Where:** Practices span development environment, CI/CD pipeline, code review, and deployment
> - **When:** Reference during vendor assessments, supply chain security reviews, or compliance audits
> - **How:** Each SSDF practice group maps to specific AeroDocs development processes

---

## Summary

| Practice Group | Implemented | Partial | Gap | Total |
|----------------|:-----------:|:-------:|:---:|:-----:|
| **PO** — Prepare the Organization | 4 | 1 | 0 | 5 |
| **PS** — Protect the Software | 1 | 1 | 1 | 3 |
| **PW** — Produce Well-Secured Software | 7 | 1 | 1 | 9 |
| **RV** — Respond to Vulnerabilities | 1 | 1 | 1 | 3 |
| **Totals** | **13** | **4** | **3** | **20** |

---

## PO: Prepare the Organization

### PO.1 — Define Security Requirements for Software Development

**Status:** ✅ Implemented

Security requirements are defined in the PRD (Section 4.1 — Authentication) and codified as enforceable controls throughout the codebase:

- **Mandatory 2FA** — TOTP enrollment is required for all users; the system cannot be used without it (`hub/internal/auth/totp.go`)
- **RBAC** — Two-role model (admin/viewer) enforced at the database level via `CHECK(role IN ('admin', 'viewer'))` constraint
- **Password policy** — Minimum 12 characters with uppercase, lowercase, digit, and special character requirements (`hub/internal/auth/password.go`)
- **Security model documentation** — Dedicated security model doc at `docs/engineering/security-model.md` covering authentication, authorization, transport, data protection, audit, and threat model

**Evidence:**
- `docs/PRD.md` — Section 4.1 (Authentication), Section 4.9 (Security)
- `docs/engineering/security-model.md` — Full security model
- `hub/internal/auth/password.go:26-58` — `ValidatePasswordPolicy()` function

---

### PO.2 — Implement Roles and Responsibilities

**Status:** ✅ Implemented

The codebase implements clear separation of roles with distinct privilege boundaries:

- **Admin role** — Full access to all servers, users, path permissions, and audit logs. 17 admin-only endpoints enforced via `adminOnly` middleware
- **Viewer role** — Read-only access scoped to explicitly granted server/path combinations via the `permissions` table
- **Separation of concerns** — Hub handles auth/authz/audit; Agent handles file I/O with path validation; Frontend enforces route guards. No component crosses its trust boundary

**Evidence:**
- `hub/internal/server/middleware.go:72-80` — `adminOnly` middleware
- `hub/internal/server/middleware.go:38-68` — `authMiddleware` with token-type enforcement
- `docs/engineering/security-model.md` — Section 2 (Authorization) lists all admin-only endpoints

---

### PO.3 — Implement Supporting Toolchains

**Status:** ✅ Implemented

Development toolchains enforce code quality and consistency:

| Tool | Purpose | Configuration |
|------|---------|---------------|
| Go toolchain (1.26) | Backend compilation, `go vet`, `go test` | `hub/go.mod`, `agent/go.mod` |
| Vite + TypeScript | Frontend build with strict type checking | `web/tsconfig.app.json` (`"strict": true`) |
| protoc + protobuf | gRPC protocol definition and code generation | `Makefile` `proto` target, `proto/aerodocs/v1/agent.proto` |
| gofmt | Go code formatting (standard toolchain) | Enforced by Go toolchain |
| Vitest | Frontend unit testing with coverage | `web/package.json` |
| Playwright | End-to-end browser testing | `tests/e2e/playwright.config.ts` |
| SonarCloud | Static analysis and coverage reporting | `.github/workflows/ci.yml` — `sonarcloud` job |
| Snyk | SAST and dependency vulnerability scanning | `.github/workflows/ci.yml` — `snyk` job |

**Evidence:**
- `Makefile` — Build orchestration with `proto`, `build-web`, `build-hub`, `build-agent` targets
- `web/tsconfig.app.json:24` — `"strict": true`
- `.github/workflows/ci.yml:102-155` — SonarCloud and Snyk CI jobs

---

### PO.4 — Define and Use Criteria for Software Security Checks

**Status:** ✅ Implemented

Software security checks are automated in the CI pipeline and enforced on every push and pull request:

- **TypeScript type checking** — `npx tsc --noEmit` in CI catches type errors before merge
- **Go unit tests** — `go test ./internal/...` with coverage for Hub; `go test ./...` for Agent
- **Frontend tests** — `vitest run --coverage` for React component tests
- **Integration tests** — `hub/internal/integration/` with 6 test files covering agent connect, file list, file read, upload, unregister, and auth flows
- **Smoke tests** — `tests/smoke/smoke_test.sh` validates Docker build and basic functionality
- **E2E tests** — 4 Playwright specs (`01-setup-flow`, `02-login`, `03-dashboard`, `04-settings`)
- **SAST** — Snyk Code Test with `--severity-threshold=high`
- **Dependency scanning** — Snyk Open Source with `--all-projects --severity-threshold=high`
- **Container scanning** — Grype and Snyk container scans on release images

**Evidence:**
- `.github/workflows/ci.yml` — `test`, `integration`, `smoke`, `e2e`, `sonarcloud`, `snyk` jobs
- `.github/workflows/release.yml:64-91` — `scan-container` job with Grype and Snyk
- `hub/internal/integration/` — 6 integration test files
- `tests/e2e/tests/` — 4 E2E test specs

---

### PO.5 — Implement and Maintain Secure Environments for Software Development

**Status:** ✅ Implemented

Development and deployment environments are hardened:

- **Systemd hardening** — `NoNewPrivileges=true`, `ProtectSystem=strict`, `PrivateTmp=true`, dedicated `aerodocs` system user with no login shell
- **TLS termination** — Traefik reverse proxy handles TLS certificates; Hub binds to localhost only
- **Docker containerization** — Multi-stage Dockerfile with minimal `debian:trixie-slim` runtime image, non-root `aerodocs` user, explicit `USER aerodocs` directive
- **Build isolation** — Frontend built in separate Docker stage; Go binaries built with `-ldflags="-s -w"` to strip debug symbols
- **DHI hardened base images** — Default `FROM` images are `dhi.io/node:25-debian13-dev` and `dhi.io/golang:1-debian13-dev`

**Evidence:**
- `Dockerfile:37` — `useradd -r -s /bin/false aerodocs`
- `Dockerfile:48` — `USER aerodocs`
- `Dockerfile:25` — `-ldflags="-s -w"` on Hub build
- `docs/engineering/security-model.md` — Section 6 (Operational Security) documents systemd hardening

---

## PS: Protect the Software

### PS.1 — Protect All Forms of Code from Unauthorized Access and Tampering

**Status:** ✅ Implemented

Source code is protected through standard version control practices:

- **Git version control** — All source code is tracked in Git with full commit history
- **GitHub hosting** — Repository hosted on GitHub with branch protection available
- **CI gating** — GitHub Actions CI runs on every push to `main` and all pull requests; tests must pass before merge

**Evidence:**
- `.github/workflows/ci.yml:3-6` — Triggers on `push` to `main` and `pull_request` to `main`
- Git repository structure with `.gitignore` excluding build artifacts and sensitive files

---

### PS.2 — Provide a Mechanism for Verifying Software Release Integrity

**Status:** ⚠️ Partial

AeroDocs provides some integrity mechanisms but lacks cryptographic release signing:

- **Single binary with embedded frontend** — The Hub binary embeds the compiled frontend via `go:embed`, producing a single artifact whose integrity can be verified as a unit (`hub/embed.go`)
- **Docker image tags** — Release images are tagged with semver versions via `docker/metadata-action` (e.g., `v1.0.0`, `1.0`, `latest`)
- **Container scanning** — Released images are scanned with Grype and Snyk before distribution

**Gap:** No cryptographic signing of release artifacts (no cosign, Sigstore, or GPG signatures on binaries or Docker images). Consumers cannot independently verify that artifacts were produced by the project's CI pipeline.

**Remediation:**
1. Add cosign signing to the Docker image push step in `.github/workflows/release.yml`
2. Generate and publish SHA-256 checksums for cross-compiled agent binaries
3. Consider Sigstore keyless signing for GitHub Actions provenance

**Evidence:**
- `hub/embed.go:5` — `//go:embed all:web/dist`
- `.github/workflows/release.yml:48-62` — Docker metadata and push with semver tags

---

### PS.3 — Archive and Protect Each Software Release

**Status:** ❌ Gap

Release artifacts are built and pushed to Docker Hub, but there is no formal SBOM (Software Bill of Materials) generation or provenance attestation:

- **Agent cross-compilation** — Binaries built for `linux/amd64` and `linux/arm64` (`Makefile`, `Dockerfile:29-30`)
- **Docker image versioning** — Semver tags applied to images via CI

**Gap:** No SBOM generation (CycloneDX or SPDX), no build provenance attestation (SLSA), and no formal release archive process beyond Docker Hub push. Dependency lists exist only in `go.mod`/`go.sum` and `package-lock.json`.

**Remediation:**
1. Add `anchore/sbom-action` to the release workflow to generate CycloneDX SBOMs
2. Enable Docker BuildKit provenance attestations (`--provenance=true`)
3. Publish agent binaries as GitHub Release assets with checksums
4. Consider SLSA Level 2 build provenance via `slsa-framework/slsa-github-generator`

**Evidence:**
- `Makefile:31-32` — Agent cross-compilation targets
- `.github/workflows/release.yml` — No SBOM or provenance steps present

---

## PW: Produce Well-Secured Software

### PW.1 — Design Software to Meet Security Requirements and Mitigate Security Risks

**Status:** ✅ Implemented

The architecture enforces security by design:

- **Hub-and-Spoke isolation** — All user interactions flow through the Hub, which enforces auth, authz, and audit before proxying to agents. Agents have no web interface and accept commands only from authenticated Hub connections
- **Agent has no inbound ports** — Agents initiate outbound gRPC connections to the Hub; no listening services are exposed on managed servers
- **Path sanitization** — Agent validates all file paths against traversal attacks before any filesystem operation. Paths containing `..` are rejected; symlinks are resolved via `filepath.EvalSymlinks`
- **Dropzone isolation** — File deletion is restricted to `/tmp/aerodocs-dropzone/` prefix only
- **Single-use registration tokens** — Server registration tokens are consumed on first use and cannot be reused

**Evidence:**
- `docs/SDD.md` — Architecture overview
- `docs/engineering/architecture.md` — Hub-and-Spoke design
- `docs/engineering/security-model.md` — Section 7 (Input Validation) documents path traversal prevention

---

### PW.2 — Review the Software Design to Verify Compliance with Security Requirements

**Status:** ✅ Implemented

Multiple design documents establish and review security posture:

- **PRD** — `docs/PRD.md` defines security requirements in Section 4.1 (Authentication) and Section 4.9 (Security)
- **SDD** — `docs/SDD.md` maps requirements to architectural decisions
- **Architecture doc** — `docs/engineering/architecture.md` documents trust boundaries
- **Security model** — `docs/engineering/security-model.md` provides threat model and trust boundary analysis
- **Design specs** — Individual feature designs in `docs/superpowers/specs/` (foundation-and-auth, fleet-dashboard, dropzone, file-tree, log-tailing, audit-logs-settings, agent)

**Evidence:**
- `docs/superpowers/specs/2026-03-23-foundation-and-auth-design.md` — Auth design decisions
- `docs/superpowers/specs/2026-03-23-agent-design.md` — Agent security design
- `docs/engineering/security-model.md` — Section 8 (Threat Model Summary)

---

### PW.4 — Reuse Existing, Well-Secured Software Components When Feasible

**Status:** ✅ Implemented

The project relies on established, well-maintained libraries rather than custom crypto:

| Component | Library | Purpose |
|-----------|---------|---------|
| Password hashing | `golang.org/x/crypto/bcrypt` | Standard Go crypto library |
| JWT | `github.com/golang-jwt/jwt/v5` | Most popular Go JWT library |
| TOTP | `github.com/pquerna/otp` | Standard OTP implementation |
| SQLite | `modernc.org/sqlite` | Pure Go SQLite (no CGO required for builds) |
| gRPC | `google.golang.org/grpc` | Google's official gRPC implementation |
| UUID | `github.com/google/uuid` | Google's UUID library |

No custom cryptographic implementations exist in the codebase. All crypto operations delegate to standard libraries.

**Evidence:**
- `hub/go.mod:6-10` — Direct dependency declarations
- `hub/internal/auth/password.go:9` — `golang.org/x/crypto/bcrypt` import
- `hub/internal/auth/jwt.go:7` — `github.com/golang-jwt/jwt/v5` import

---

### PW.5 — Create Source Code by Adhering to Secure Coding Practices

**Status:** ✅ Implemented

Secure coding practices are evident throughout the codebase:

- **Input validation** — Password policy enforcement (`ValidatePasswordPolicy`), username validation (3-32 chars, alphanumeric + underscore), path traversal prevention
- **Password hashing** — bcrypt at cost 12 (`hub/internal/auth/password.go:12`)
- **JWT type enforcement** — 4 distinct token types with strict endpoint matching; a valid access token cannot be used on a setup endpoint (`hub/internal/server/middleware.go:59-62`)
- **Rate limiting** — 5 attempts per IP per 60-second window on auth endpoints (`hub/internal/server/middleware.go:109-163`)
- **Path traversal prevention** — `..` rejection, `filepath.Clean`, `filepath.EvalSymlinks`, absolute path requirement
- **Cryptographic randomness** — `crypto/rand` used for temporary password generation (`hub/internal/auth/password.go:96-98`)
- **Short token lifetimes** — Access: 15 min, TOTP: 60 sec, Setup: 10 min (`hub/internal/auth/jwt.go:16-19`)
- **Structured error handling** — Errors wrapped with `fmt.Errorf("context: %w", err)` throughout

**Evidence:**
- `hub/internal/auth/password.go:12` — `const bcryptCost = 12`
- `hub/internal/auth/jwt.go:16-19` — Token expiry constants
- `hub/internal/auth/jwt.go:76` — Signing method validation rejects non-HMAC algorithms
- `hub/internal/server/middleware.go:116-122` — Rate limiter initialization

---

### PW.6 — Configure the Compilation, Interpreter, and Build Processes to Improve Executable Security

**Status:** ✅ Implemented

Build processes are configured for security:

- **Vite production build** — Frontend compiled with production optimizations and tree-shaking via `npm run build`
- **Go build flags** — `-ldflags="-s -w"` strips symbol table and debug info from production binaries (`Dockerfile:25`)
- **CGO control** — Hub built with `CGO_ENABLED=1` for SQLite; Agent built with `CGO_ENABLED=0` for static linking (`Dockerfile:25-30`)
- **Cross-compilation** — Agent compiled for `linux/amd64` and `linux/arm64` targets
- **TypeScript strict mode** — `"strict": true` in `web/tsconfig.app.json` and `web/tsconfig.node.json` enables all strict type-checking options
- **Multi-stage Docker build** — Build tools excluded from runtime image; final stage uses `debian:trixie-slim`

**Evidence:**
- `Dockerfile:25` — `CGO_ENABLED=1 go build -ldflags="-s -w"`
- `Dockerfile:29-30` — `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` / `arm64`
- `web/tsconfig.app.json:24` — `"strict": true`

---

### PW.7 — Review and/or Analyze Human-Readable Code to Identify Vulnerabilities and Verify Compliance with Security Requirements

**Status:** ⚠️ Partial

Code review and static analysis are partially implemented:

- **Go unit tests** — Hub and Agent tested with `go test` including coverage profiling
- **httptest-based handler tests** — Server middleware tests in `hub/internal/server/middleware_test.go`
- **Auth module tests** — `hub/internal/auth/password_test.go`, `jwt_test.go`, `jwt_extra_test.go`, `totp_test.go`
- **SonarCloud** — Static analysis integrated in CI via `SonarSource/sonarqube-scan-action`
- **Snyk SAST** — `snyk code test` runs in CI with `--severity-threshold=high`

**Gap:** No formal code review process is documented (e.g., required reviewers, review checklists). GitHub branch protection rules requiring PR reviews are not configured in the repository settings files (this may be configured in GitHub's UI but is not codified).

**Remediation:**
1. Enable branch protection rules requiring at least one approving review before merge
2. Add a `CODEOWNERS` file to assign security-sensitive paths to designated reviewers
3. Document code review expectations in a `CONTRIBUTING.md`

**Evidence:**
- `hub/internal/auth/password_test.go`, `jwt_test.go`, `totp_test.go` — Auth module tests
- `.github/workflows/ci.yml:102-155` — SonarCloud and Snyk SAST in CI

---

### PW.8 — Test Executable Code to Identify Vulnerabilities and Verify Compliance with Security Requirements

**Status:** ✅ Implemented

Multi-layered testing is automated in the CI pipeline:

| Layer | Tool | Scope | CI Job |
|-------|------|-------|--------|
| Unit | `go test`, Vitest | Hub, Agent, Frontend | `test` |
| Integration | `go test ./internal/integration/` | Agent connect, file ops, auth flows | `integration` |
| Smoke | `smoke_test.sh` | Docker build + basic validation | `smoke` |
| E2E | Playwright | Setup flow, login, dashboard, settings | `e2e` |
| SAST | Snyk Code | Source code vulnerability detection | `snyk` |
| SCA | Snyk Open Source | Dependency vulnerability scanning | `snyk` |
| Container | Grype + Snyk Docker | Runtime image vulnerability scanning | `scan-container` |

All test layers run automatically on push to `main` and on pull requests.

**Evidence:**
- `.github/workflows/ci.yml` — All 6 CI jobs
- `.github/workflows/release.yml:64-91` — Container scanning on release
- `tests/e2e/tests/` — 4 Playwright test specs
- `hub/internal/integration/` — 6 integration test files

---

### PW.9 — Configure Software to Have Secure Settings by Default

**Status:** ✅ Implemented

Security-relevant defaults are hardened out of the box:

| Setting | Default | Location |
|---------|---------|----------|
| 2FA | Mandatory (no opt-out) | `hub/internal/server/handlers_auth.go` |
| bcrypt cost | 12 | `hub/internal/auth/password.go:12` |
| Access token expiry | 15 minutes | `hub/internal/auth/jwt.go:16` |
| Refresh token expiry | 7 days | `hub/internal/auth/jwt.go:17` |
| TOTP token expiry | 60 seconds | `hub/internal/auth/jwt.go:19` |
| Setup token expiry | 10 minutes | `hub/internal/auth/jwt.go:18` |
| SQLite journal mode | WAL | `hub/internal/store/store.go` |
| Foreign keys | Enabled | `hub/internal/store/store.go` |
| Password minimum length | 12 characters | `hub/internal/auth/password.go:27` |
| Rate limit | 5 per IP per 60s | `hub/internal/server/middleware.go` |
| Docker user | Non-root `aerodocs` | `Dockerfile:48` |
| CORS | Disabled in production | `hub/internal/server/middleware.go:93-106` |

No security feature requires explicit opt-in. The platform ships in its most secure configuration.

**Evidence:**
- `hub/internal/auth/jwt.go:16-19` — Short token lifetimes as constants
- `hub/internal/auth/password.go:12` — `const bcryptCost = 12`
- `Dockerfile:48` — `USER aerodocs`

---

## RV: Respond to Vulnerabilities

### RV.1 — Identify and Confirm Vulnerabilities on an Ongoing Basis

**Status:** ⚠️ Partial

Vulnerability identification is partially automated:

- **Snyk dependency scanning** — Runs in CI on every push and PR; monitors dependencies on `main` via `snyk monitor`
- **Grype container scanning** — Scans released Docker images for known CVEs
- **SonarCloud** — Continuous static analysis for code quality and security issues
- **Audit log** — 25 event types track security-relevant actions (failed logins, TOTP failures, file access) for detection
- **Heartbeat monitoring** — Agent heartbeats every 10 seconds; offline detection within 30 seconds

**Gap:** No automated dependency update mechanism (Dependabot or Renovate) to proactively surface new vulnerabilities. No formal vulnerability disclosure process (no `SECURITY.md` file).

**Remediation:**
1. Add a `.github/dependabot.yml` configuration for Go modules and npm packages
2. Create a `SECURITY.md` with vulnerability reporting instructions and expected response timelines
3. Consider enabling GitHub security advisories

**Evidence:**
- `.github/workflows/ci.yml:118-155` — Snyk SAST and SCA
- `.github/workflows/release.yml:64-91` — Grype and Snyk container scans

---

### RV.2 — Assess, Prioritize, and Remediate Vulnerabilities

**Status:** ✅ Implemented

The platform provides tools for vulnerability assessment and remediation:

- **Immutable audit trail** — Append-only audit log with no `UPDATE` or `DELETE` operations; supports forensic analysis with user, action, target, detail, IP, and timestamp fields
- **Snyk severity thresholds** — CI configured with `--severity-threshold=high` to focus on critical issues
- **Grype severity cutoff** — Container scan uses `severity-cutoff: high`
- **Snyk dashboard monitoring** — `snyk monitor` uploads dependency graph on `main` merges for ongoing tracking

**Evidence:**
- `docs/engineering/security-model.md` — Section 5 (Audit and Accountability)
- `.github/workflows/ci.yml:135` — `--severity-threshold=high`
- `.github/workflows/release.yml:79` — `severity-cutoff: high`

---

### RV.3 — Analyze Vulnerabilities to Identify Their Root Causes

**Status:** ❌ Gap

No formal root cause analysis process is documented:

- **Structured error handling** — Errors are wrapped with context via `fmt.Errorf("context: %w", err)` throughout the codebase, aiding debugging
- **Audit log context** — Each audit entry includes user ID, action, target resource, detail text, and IP address for incident correlation

**Gap:** No documented incident response process, no post-mortem template, and no process for feeding vulnerability root causes back into development practices. This is expected for a project at this maturity level but represents a gap against SSDF requirements.

**Remediation:**
1. Create an incident response playbook documenting triage, investigation, and resolution steps
2. Establish a post-mortem template for security incidents
3. Define a process to update security requirements (`PO.1`) based on root cause findings

**Evidence:**
- Structured error wrapping present throughout `hub/internal/auth/`, `hub/internal/server/`, `hub/internal/store/`

---

## Gap Summary and Remediation Roadmap

| # | Gap | SSDF Practice | Priority | Effort |
|---|-----|---------------|----------|--------|
| 1 | No cryptographic release signing | PS.2 | High | Medium — Add cosign to release workflow |
| 2 | No SBOM generation | PS.3 | High | Low — Add `anchore/sbom-action` to release |
| 3 | No `SECURITY.md` or vulnerability disclosure process | RV.1 | High | Low — Create file with reporting instructions |
| 4 | No Dependabot/Renovate for dependency updates | RV.1 | Medium | Low — Add `.github/dependabot.yml` |
| 5 | No `CODEOWNERS` or documented review requirements | PW.7 | Medium | Low — Create file, enable branch protection |
| 6 | No incident response process | RV.3 | Medium | Medium — Write playbook and post-mortem template |
| 7 | No SLSA provenance attestation | PS.3 | Low | Medium — Integrate `slsa-github-generator` |

---

## References

- [NIST SP 800-218: Secure Software Development Framework (SSDF) v1.1](https://csrc.nist.gov/publications/detail/sp/800-218/final)
- [Executive Order 14028: Improving the Nation's Cybersecurity](https://www.whitehouse.gov/briefing-room/presidential-actions/2021/05/12/executive-order-on-improving-the-nations-cybersecurity/)
- [AeroDocs Security Model](security-model.md)
- [AeroDocs CMMC L2 Mapping](cmmc-l2-mapping.md)
- [AeroDocs Architecture](architecture.md)
