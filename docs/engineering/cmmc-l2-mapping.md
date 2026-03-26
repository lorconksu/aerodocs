# CMMC Level 2 Security Mapping

> **TL;DR**
> - **What:** Mapping of AeroDocs security controls to CMMC Level 2 (Advanced) practices
> - **Who:** Compliance officers, security auditors, and organizations evaluating AeroDocs for CUI-handling environments
> - **Why:** Demonstrates how AeroDocs addresses NIST SP 800-171 controls required for CMMC L2 certification
> - **Where:** Controls are implemented across Hub, Agent, and operational procedures
> - **When:** Reference during security assessments, audits, or procurement decisions
> - **How:** Each CMMC domain maps to specific AeroDocs features with implementation evidence

---

## About This Document

CMMC Level 2 (Advanced) requires implementation of 110 practices derived from NIST SP 800-171 Rev 2 across 14 security domains. This document maps each practice to AeroDocs features where applicable, with honest assessments of coverage gaps.

AeroDocs is an **application-layer tool** (infrastructure observability platform), not a full IT environment. Many CMMC practices address organizational policies, physical security, or network infrastructure that fall outside the scope of any single application. These are marked as Not Applicable with rationale.

**Status legend:**
- **Fully Addressed** -AeroDocs implements this control directly
- **Partially Addressed** -AeroDocs contributes to this control but does not fully satisfy it alone
- **Not Addressed** -AeroDocs does not implement this control; the deploying organization must address it
- **Not Applicable** -This practice targets organizational/physical/HR controls outside AeroDocs' scope

---

## Summary Scorecard

| Domain | Practices | Fully Addressed | Partially Addressed | Not Applicable | Not Addressed |
|--------|-----------|-----------------|---------------------|----------------|---------------|
| Access Control (AC) | 22 | 10 | 6 | 0 | 6 |
| Audit & Accountability (AU) | 9 | 5 | 2 | 0 | 2 |
| Awareness & Training (AT) | 3 | 0 | 0 | 3 | 0 |
| Configuration Management (CM) | 9 | 3 | 3 | 0 | 3 |
| Identification & Authentication (IA) | 11 | 7 | 2 | 0 | 2 |
| Incident Response (IR) | 3 | 0 | 2 | 0 | 1 |
| Maintenance (MA) | 6 | 0 | 2 | 3 | 1 |
| Media Protection (MP) | 8 | 1 | 2 | 3 | 2 |
| Personnel Security (PS) | 2 | 0 | 0 | 2 | 0 |
| Physical Protection (PE) | 6 | 0 | 0 | 6 | 0 |
| Risk Assessment (RA) | 3 | 1 | 1 | 0 | 1 |
| Security Assessment (CA) | 4 | 1 | 2 | 0 | 1 |
| System & Communications Protection (SC) | 16 | 5 | 4 | 2 | 5 |
| System & Information Integrity (SI) | 7 | 2 | 3 | 0 | 2 |
| **Totals** | **109** | **35** | **29** | **19** | **26** |

> **Note:** The official NIST SP 800-171 Rev 2 contains 110 security requirements. Practice 3.12.4 (System Security Plan) is counted under Security Assessment. Some numbering schemes vary by source; totals here reflect the practices enumerated per domain below.

---

## 1. Access Control (AC) -22 Practices

| Practice ID | Practice Title | AeroDocs Implementation | Status |
|-------------|---------------|------------------------|--------|
| 3.1.1 | Limit system access to authorized users, processes, and devices | JWT-based authentication required for all API endpoints; auth middleware validates token on every request. No anonymous access to protected resources. | Fully Addressed |
| 3.1.2 | Limit system access to authorized transaction types | 4-type JWT system enforces token-type scoping -access, refresh, setup, and TOTP tokens are each accepted only at their designated endpoints. `adminOnly` middleware restricts 17+ endpoints to admin role. | Fully Addressed |
| 3.1.3 | Control CUI flow in accordance with approved authorizations | Per-server, per-path permissions restrict viewer access to explicitly granted filesystem paths. Admins bypass permission checks. File content flows only through Hub (never direct agent access). | Partially Addressed |
| 3.1.4 | Separate duties of individuals to reduce risk | Two-role model (admin/viewer) separates administrative and read-only functions. However, no separation between admin sub-duties (user management vs. server management vs. audit review). | Partially Addressed |
| 3.1.5 | Employ principle of least privilege | Viewers default to zero access -explicit path grants required. Admins have full access (no sub-scoping). Rate limiting on auth endpoints (5/min per IP). | Partially Addressed |
| 3.1.6 | Use non-privileged accounts for non-security functions | Hub runs as dedicated `aerodocs` system user with `nologin` shell. Viewer role provides non-privileged access for browsing. | Fully Addressed |
| 3.1.7 | Prevent non-privileged users from executing privileged functions | `adminOnly` middleware enforces role checks before handler execution. Viewers cannot create users, manage servers, view audit logs, or upload files. Returns 403 Forbidden. | Fully Addressed |
| 3.1.8 | Limit unsuccessful logon attempts | Rate limiter: 5 attempts per IP per 60-second sliding window on login, TOTP, and registration endpoints. Returns 429 with `Retry-After: 60` header. | Fully Addressed |
| 3.1.9 | Provide privacy and security notices | Not implemented. AeroDocs does not display login banners or privacy notices. | Not Addressed |
| 3.1.10 | Use session lock with pattern-hiding displays | Not implemented at application level. Access tokens expire after 15 minutes, providing implicit session timeout. No active session lock or screen blanking. | Partially Addressed |
| 3.1.11 | Terminate (automatically) user sessions after defined conditions | Access tokens expire after 15 minutes; refresh tokens after 7 days. Token refresh issues a new pair, invalidating the old. No server-side session revocation list. | Partially Addressed |
| 3.1.12 | Monitor and control remote access sessions | All access is remote (web-based). Every API call passes through auth middleware. Audit log records login, login failures, and TOTP failures with IP addresses. | Fully Addressed |
| 3.1.13 | Employ cryptographic mechanisms to protect remote access | TLS termination via Traefik for all browser-to-Hub traffic. gRPC uses TLS auto-detection for Hub-to-Agent connections over WAN. | Fully Addressed |
| 3.1.14 | Route remote access via managed access control points | All traffic routes through Traefik (TLS termination) then Hub (auth/authz). Agents connect outbound to Hub -no direct user-to-agent access. | Fully Addressed |
| 3.1.15 | Authorize remote execution of privileged commands | Admin role required for all privileged operations (user management, server management, file upload). Break-glass TOTP reset requires shell access to Hub server. | Fully Addressed |
| 3.1.16 | Authorize wireless access | Not applicable at application layer; however, AeroDocs enforces TLS regardless of transport. Organization must control wireless infrastructure. | Not Addressed |
| 3.1.17 | Protect wireless access using authentication and encryption | Not applicable at application layer. Organization responsibility. | Not Addressed |
| 3.1.18 | Control connection of mobile devices | Not implemented. AeroDocs does not differentiate between device types. | Not Addressed |
| 3.1.19 | Encrypt CUI on mobile devices | Not implemented. AeroDocs does not manage device-level encryption. File content is transmitted over TLS but not encrypted at rest on client devices. | Not Addressed |
| 3.1.20 | Verify and control/limit connections to external systems | Agent connections use single-use registration tokens with expiry. Hub tracks all connected agents. No outbound connections from Hub to arbitrary external systems. | Partially Addressed |
| 3.1.21 | Limit use of portable storage devices | Not implemented. AeroDocs does not control USB or portable media. | Not Addressed |
| 3.1.22 | Control CUI posted or processed on publicly accessible systems | Hub binds to localhost (127.0.0.1:8080); Traefik handles external exposure. No public API endpoints expose CUI -authentication required. Agent install endpoints are public but serve only the binary. | Fully Addressed |

---

## 2. Audit & Accountability (AU) -9 Practices

| Practice ID | Practice Title | AeroDocs Implementation | Status |
|-------------|---------------|------------------------|--------|
| 3.3.1 | Create and retain system audit logs and records | Immutable audit log with 25 event types across user, server, file, and log operations. Append-only design -no UPDATE or DELETE operations exist in the codebase. | Fully Addressed |
| 3.3.2 | Ensure actions can be traced to individual users | Every audit entry records `user_id`, `action`, `target`, `detail`, `ip_address`, and `created_at`. Indexed on user_id for efficient per-user queries. | Fully Addressed |
| 3.3.3 | Review and update logged events | 25 event types defined as Go constants in the codebase. Adding new events requires code changes and redeployment. No runtime configuration of audit scope. | Partially Addressed |
| 3.3.4 | Alert on audit logging process failures | Not implemented. No alerting mechanism if audit log writes fail. SQLite write failures would surface as HTTP 500 errors but no dedicated monitoring. | Not Addressed |
| 3.3.5 | Correlate audit review, analysis, and reporting | Audit logs are queryable by user, action type, and date range via the admin API (`GET /api/audit-logs`). IP tracking enables cross-referencing. No built-in SIEM integration. | Partially Addressed |
| 3.3.6 | Provide audit record reduction and report generation | Filtered queries supported via API (user, action, date range). Three database indexes support efficient queries. No built-in report generation or export. | Fully Addressed |
| 3.3.7 | Provide a system capability for comparing time stamps | All timestamps use ISO 8601 format via SQLite `datetime('now')`. Single Hub server eliminates clock synchronization issues. `created_at` index supports time-ordered queries. | Fully Addressed |
| 3.3.8 | Protect audit information and tools from unauthorized access | Audit log endpoint (`GET /api/audit-logs`) restricted to admin role via `adminOnly` middleware. Database file protected by OS-level `aerodocs` user permissions. No API exists to modify or delete audit entries. | Fully Addressed |
| 3.3.9 | Limit management of audit logging to a subset of users | Only admins can view audit logs. No user can modify or delete audit entries (no such API exists). Audit log schema changes require code deployment. | Not Addressed |

> **Note on 3.3.9:** While AeroDocs protects audit log integrity (no modification API exists), there is no granular "audit administrator" role separate from the general admin role. The practice asks for limiting *management* of audit functionality to specific privileged users, which is partially met by admin-only access but lacks a dedicated audit management role.

---

## 3. Awareness & Training (AT) -3 Practices

| Practice ID | Practice Title | AeroDocs Implementation | Status |
|-------------|---------------|------------------------|--------|
| 3.2.1 | Ensure personnel are aware of security risks | Not Applicable -organizational training responsibility. AeroDocs provides security documentation (this document, security model doc) but does not deliver training. | Not Applicable |
| 3.2.2 | Ensure personnel are trained in their security roles | Not Applicable -organizational training responsibility. | Not Applicable |
| 3.2.3 | Provide security awareness training on recognizing and reporting threats | Not Applicable -organizational training responsibility. | Not Applicable |

---

## 4. Configuration Management (CM) -9 Practices

| Practice ID | Practice Title | AeroDocs Implementation | Status |
|-------------|---------------|------------------------|--------|
| 3.4.1 | Establish and maintain baseline configurations | Single binary deployment with embedded frontend. SQLite auto-migrations enforce schema consistency. Agent config stored in `/etc/aerodocs/agent.conf`. No external dependencies (no Redis, no Postgres, no message queue). | Fully Addressed |
| 3.4.2 | Establish and enforce security configuration settings | Systemd hardening (`NoNewPrivileges`, `ProtectSystem=strict`, `PrivateTmp`). bcrypt cost fixed at 12. Rate limiting hardcoded (5/60s). Password policy enforced in code (12+ chars, complexity requirements). | Fully Addressed |
| 3.4.3 | Track, review, approve, and log changes to systems | Audit log captures server configuration changes (`server.created`, `server.updated`, `server.deleted`). User role changes logged. No built-in change approval workflow. | Partially Addressed |
| 3.4.4 | Analyze security impact of changes prior to implementation | Not implemented. No built-in change impact analysis. Organization must implement change management processes. | Not Addressed |
| 3.4.5 | Define, document, approve, and enforce access restrictions for changes | Admin role required for all configuration changes. Break-glass operations require shell access. No multi-admin approval workflow. | Partially Addressed |
| 3.4.6 | Employ the principle of least functionality | Hub serves only its intended functions (auth, file browsing, log tailing, audit). No unnecessary services. Agent executes only Hub-instructed operations (file list, file read, log tail, dropzone). CORS disabled in production. | Fully Addressed |
| 3.4.7 | Restrict, disable, or prevent the use of nonessential programs | Not implemented at application level. AeroDocs does not control what other software runs on the host. Systemd `ProtectSystem=strict` limits filesystem access. | Partially Addressed |
| 3.4.8 | Apply deny-by-exception (blacklisting) policy for unauthorized software | Not implemented. AeroDocs does not manage software inventory or execution policies on host systems. | Not Addressed |
| 3.4.9 | Control and monitor user-installed software | Not implemented. Outside AeroDocs' scope as an application. | Not Addressed |

---

## 5. Identification & Authentication (IA) -11 Practices

| Practice ID | Practice Title | AeroDocs Implementation | Status |
|-------------|---------------|------------------------|--------|
| 3.5.1 | Identify system users, processes, and devices | Users identified by unique UUID (`users.id`) and unique username. Servers identified by UUID (`servers.id`) with hostname and IP recorded on registration. JWT claims carry user ID and role. | Fully Addressed |
| 3.5.2 | Authenticate users, processes, and devices | Users: bcrypt password + mandatory TOTP 2FA. Agents: single-use registration token with expiry, then persistent gRPC stream with heartbeat monitoring. All API access requires valid JWT. | Fully Addressed |
| 3.5.3 | Use multifactor authentication for local and network access | Mandatory TOTP 2FA for all users -no opt-out path. Password (knowledge factor) + TOTP (possession factor). Setup token flow enforces enrollment before access token issuance. | Fully Addressed |
| 3.5.4 | Employ replay-resistant authentication mechanisms | TOTP codes are time-based (30-second window). JWT tokens include expiry claims enforced by middleware. Registration tokens are single-use (consumed on first registration). | Fully Addressed |
| 3.5.5 | Prevent reuse of identifiers | UUIDs generated for all users, servers, permissions, and audit entries. Username and email have UNIQUE constraints. Server registration tokens have UNIQUE constraint. | Fully Addressed |
| 3.5.6 | Disable identifiers after a defined period of inactivity | Not implemented. No account inactivity timeout or automatic disabling. Admin must manually delete inactive users. | Not Addressed |
| 3.5.7 | Enforce minimum password complexity | Password policy enforced in code: minimum 12 characters, requires uppercase, lowercase, digit, and special character. Validated in `hub/internal/auth/password.go`. | Fully Addressed |
| 3.5.8 | Prohibit password reuse for a specified number of generations | Not implemented. No password history tracking. Users can reuse previous passwords. | Not Addressed |
| 3.5.9 | Allow temporary password use for system logons with an immediate change | Admin-created users receive a temporary password and must complete TOTP enrollment on first login (which involves the setup token flow). However, no forced password change mechanism exists. | Partially Addressed |
| 3.5.10 | Store and transmit only cryptographically-protected passwords | Passwords hashed with bcrypt (cost 12) before storage. Plaintext never stored or logged. Password transmitted over TLS (Traefik) during login. `password_hash` excluded from all API responses. | Fully Addressed |
| 3.5.11 | Obscure feedback of authentication information | TOTP secret shown only once during setup (QR code). Password hash never returned in API responses. Login failures return generic error messages (no enumeration of valid usernames). | Partially Addressed |

---

## 6. Incident Response (IR) -3 Practices

| Practice ID | Practice Title | AeroDocs Implementation | Status |
|-------------|---------------|------------------------|--------|
| 3.6.1 | Establish an operational incident-handling capability | Not implemented as a feature. AeroDocs provides forensic data (audit log with 25 event types, IP tracking, timestamps) but no incident response workflow, alerting, or escalation. | Partially Addressed |
| 3.6.2 | Track, document, and report incidents | Audit log provides an immutable record of all security-relevant events (login failures, TOTP failures, user changes, server changes). Queryable by user, action, and date range. No automated reporting or notification. | Partially Addressed |
| 3.6.3 | Test the organizational incident response capability | Not implemented. No built-in incident response testing or simulation capability. Organization must conduct tabletop exercises independently. | Not Addressed |

---

## 7. Maintenance (MA) -6 Practices

| Practice ID | Practice Title | AeroDocs Implementation | Status |
|-------------|---------------|------------------------|--------|
| 3.7.1 | Perform maintenance on organizational systems | Automated SQLite schema migrations on startup. Agent auto-reconnect with exponential backoff (1s to 60s cap). Systemd service management for Hub and Agent. | Partially Addressed |
| 3.7.2 | Provide controls on tools, techniques, and personnel for maintenance | Break-glass TOTP reset requires shell access to Hub server (not accessible via web UI or API). Admin role required for all management operations. | Partially Addressed |
| 3.7.3 | Ensure equipment removed for off-site maintenance is sanitized | Not Applicable -AeroDocs is software, not physical equipment. | Not Applicable |
| 3.7.4 | Check media containing diagnostic and test programs for malicious code | Not implemented. AeroDocs does not scan uploaded files (dropzone) for malware. Organization must implement file scanning. | Not Addressed |
| 3.7.5 | Require multifactor authentication for remote maintenance sessions | Not Applicable -maintenance is performed via shell access (outside AeroDocs) or via AeroDocs admin UI which requires mandatory 2FA. | Not Applicable |
| 3.7.6 | Supervise maintenance activities of personnel without required access | Not Applicable -organizational process, not software-enforced. All AeroDocs admin actions are audit-logged. | Not Applicable |

---

## 8. Media Protection (MP) -8 Practices

| Practice ID | Practice Title | AeroDocs Implementation | Status |
|-------------|---------------|------------------------|--------|
| 3.8.1 | Protect (control access to) system media containing CUI | File access controlled via per-path permissions. Admin-only upload to dropzone. Agent restricts file deletion to dropzone directory only. Database file protected by OS-level `aerodocs` user permissions. | Partially Addressed |
| 3.8.2 | Limit access to CUI on system media to authorized users | Per-server, per-path permission grants for viewers. Admin-only access to dropzone and upload operations. Hub auth middleware enforces access on every file request. | Fully Addressed |
| 3.8.3 | Sanitize or destroy system media before disposal or reuse | Not Applicable -AeroDocs is software. Agent self-cleanup on unregister removes binary, config, and dropzone files, but does not perform secure erasure. | Not Applicable |
| 3.8.4 | Mark media with necessary CUI markings and distribution limitations | Not implemented. AeroDocs does not apply or track CUI markings on files. | Not Addressed |
| 3.8.5 | Control access to media containing CUI and maintain accountability | Audit log tracks all file read operations (`file.read`) and file uploads (`file.uploaded`) with user ID and IP address. Path grants and revocations are logged. | Partially Addressed |
| 3.8.6 | Implement cryptographic mechanisms to protect CUI during transport | File content transferred over TLS (browser to Hub) and gRPC with TLS (Hub to Agent over WAN). Base64 encoding for API responses (encoding, not encryption). | Not Addressed |
| 3.8.7 | Control the use of removable media on system components | Not Applicable -outside application scope. AeroDocs does not interact with removable media. | Not Applicable |
| 3.8.8 | Prohibit the use of portable storage devices when owner is unidentified | Not Applicable -outside application scope. | Not Applicable |
| 3.8.9 | Protect the confidentiality of backup CUI at storage locations | Not implemented. AeroDocs does not manage backups. SQLite database backups are the organization's responsibility. | Not Addressed |

---

## 9. Personnel Security (PS) -2 Practices

| Practice ID | Practice Title | AeroDocs Implementation | Status |
|-------------|---------------|------------------------|--------|
| 3.9.1 | Screen individuals prior to authorizing access | Not Applicable -organizational HR process. AeroDocs provides admin-controlled user creation (no self-registration after initial setup). | Not Applicable |
| 3.9.2 | Ensure CUI is protected during and after personnel actions (transfers, terminations) | Not Applicable -organizational process. AeroDocs supports immediate user deletion and permission revocation by admins, with CASCADE deletes removing all associated permissions. | Not Applicable |

---

## 10. Physical Protection (PE) -6 Practices

| Practice ID | Practice Title | AeroDocs Implementation | Status |
|-------------|---------------|------------------------|--------|
| 3.10.1 | Limit physical access to organizational systems | Not Applicable -physical infrastructure security is outside AeroDocs' scope. | Not Applicable |
| 3.10.2 | Protect and monitor the physical facility | Not Applicable -physical infrastructure. | Not Applicable |
| 3.10.3 | Escort visitors and monitor visitor activity | Not Applicable -physical infrastructure. | Not Applicable |
| 3.10.4 | Maintain audit logs of physical access | Not Applicable -physical infrastructure. | Not Applicable |
| 3.10.5 | Control and manage physical access devices | Not Applicable -physical infrastructure. | Not Applicable |
| 3.10.6 | Enforce safeguarding measures for CUI at alternate work sites | Not Applicable -physical infrastructure and policy. | Not Applicable |

---

## 11. Risk Assessment (RA) -3 Practices

| Practice ID | Practice Title | AeroDocs Implementation | Status |
|-------------|---------------|------------------------|--------|
| 3.11.1 | Periodically assess risk to organizational operations and assets | Threat model documented in [security-model.md](security-model.md) with trust boundary analysis, key assumptions, and out-of-scope threats. No automated risk assessment tooling. | Partially Addressed |
| 3.11.2 | Scan for vulnerabilities periodically and when new vulnerabilities are identified | Not implemented. No built-in vulnerability scanning. Organization must implement Go dependency scanning (`govulncheck`), container scanning, and infrastructure vulnerability assessment. | Not Addressed |
| 3.11.3 | Remediate vulnerabilities in accordance with risk assessments | Go modules enable dependency updates. Single binary simplifies patching (replace binary, restart service). Auto-migrations handle schema updates. No built-in vulnerability tracking or remediation workflow. | Fully Addressed |

---

## 12. Security Assessment (CA) -4 Practices

| Practice ID | Practice Title | AeroDocs Implementation | Status |
|-------------|---------------|------------------------|--------|
| 3.12.1 | Periodically assess security controls to determine effectiveness | This document provides a comprehensive mapping. [security-model.md](security-model.md) documents all security controls. Automated testing (Go unit tests, Playwright E2E) validates control implementation. | Partially Addressed |
| 3.12.2 | Develop and implement plans of action to correct deficiencies | Not implemented as a feature. This document identifies gaps (Not Addressed items) that serve as a starting point for remediation planning. | Not Addressed |
| 3.12.3 | Monitor security controls on an ongoing basis | Audit log provides continuous monitoring of auth events and access patterns. Heartbeat monitoring tracks agent connectivity (15s interval, 30s staleness threshold). No automated security control testing. | Partially Addressed |
| 3.12.4 | Develop, document, and periodically update system security plans | This CMMC mapping document, [security-model.md](security-model.md), and [architecture.md](architecture.md) collectively document the system security posture. Updated with each release. | Fully Addressed |

---

## 13. System & Communications Protection (SC) -16 Practices

| Practice ID | Practice Title | AeroDocs Implementation | Status |
|-------------|---------------|------------------------|--------|
| 3.13.1 | Monitor, control, and protect communications at external boundaries | Traefik terminates TLS at the external boundary. Hub binds to localhost only. All external traffic passes through Traefik before reaching the Hub. Agent connections are outbound-only. | Fully Addressed |
| 3.13.2 | Employ architectural designs that promote effective information security | Hub-and-Spoke architecture with clear trust boundaries. Hub is the single control point. Agents have no user interface or direct access. Separation of concerns across packages (auth, store, server, grpcserver). | Fully Addressed |
| 3.13.3 | Separate user functionality from system management functionality | Admin and viewer roles are separate. Admin UI functions are distinct routes. API endpoints are split between user-accessible and admin-only (enforced by middleware). Frontend uses route guards. | Partially Addressed |
| 3.13.4 | Prevent unauthorized and unintended information transfer | Per-path permissions restrict file access. Agent validates all paths (traversal prevention, symlink resolution). File deletion restricted to dropzone only. 1 MB maximum file read size. | Partially Addressed |
| 3.13.5 | Implement subnetworks for publicly accessible system components | Hub architecture supports this -Traefik in DMZ, Hub on internal network, Agents on managed servers. However, network segmentation is the deployer's responsibility. | Partially Addressed |
| 3.13.6 | Deny network communications traffic by default | Not implemented at application level. AeroDocs does not manage firewall rules. Hub accepts connections on its configured port; access control is at the authentication layer, not the network layer. | Not Addressed |
| 3.13.7 | Prevent remote devices from simultaneously establishing non-remote connections | Not Applicable -AeroDocs is a web application and does not manage device network configurations. | Not Applicable |
| 3.13.8 | Implement cryptographic mechanisms to prevent unauthorized disclosure during transmission | TLS via Traefik for browser-to-Hub. gRPC TLS auto-detection for Hub-to-Agent over WAN. JWT tokens signed with HMAC-SHA256. bcrypt for password storage. | Fully Addressed |
| 3.13.9 | Terminate network connections at the end of sessions or after inactivity | Access tokens expire after 15 minutes. Refresh tokens expire after 7 days. gRPC connections have heartbeat monitoring (30s staleness threshold). No persistent HTTP sessions (stateless JWT). | Partially Addressed |
| 3.13.10 | Establish and manage cryptographic keys | JWT signing key generated and stored in SQLite `_config` table at first startup. TOTP secrets generated per-user using standard TOTP library. No key rotation mechanism. | Fully Addressed |
| 3.13.11 | Employ FIPS-validated cryptography when used to protect CUI | Not implemented. AeroDocs uses standard Go crypto libraries (bcrypt, HMAC-SHA256) which are not FIPS 140-2 validated. Would require building with `GOEXPERIMENT=boringcrypto` for FIPS compliance. | Not Addressed |
| 3.13.12 | Prohibit remote activation of collaborative computing devices | Not Applicable -AeroDocs does not interact with collaborative computing devices (cameras, microphones). | Not Applicable |
| 3.13.13 | Control and monitor the use of mobile code | Not implemented. AeroDocs serves a React SPA (JavaScript) but does not restrict or monitor client-side code execution. | Not Addressed |
| 3.13.14 | Control and monitor the use of Voice over Internet Protocol (VoIP) technologies | Not Applicable -AeroDocs does not use VoIP. Listed for completeness. | Not Addressed |
| 3.13.15 | Protect the authenticity of communications sessions | JWT tokens authenticate each API request. Token type enforcement prevents token misuse across endpoints. gRPC stream maintains persistent authenticated session per agent. TOTP provides session origin verification. | Fully Addressed |
| 3.13.16 | Protect the confidentiality of CUI at rest | Not implemented. SQLite database is not encrypted at rest by AeroDocs. Relies on OS-level disk encryption (LUKS, dm-crypt) if required. File content is stored on managed servers, not in the Hub. | Not Addressed |

---

## 14. System & Information Integrity (SI) -7 Practices

| Practice ID | Practice Title | AeroDocs Implementation | Status |
|-------------|---------------|------------------------|--------|
| 3.14.1 | Identify, report, and correct system flaws in a timely manner | Go module system enables dependency tracking. Single binary deployment simplifies updates (replace + restart). Auto-migrations handle schema changes. No built-in vulnerability notification. | Partially Addressed |
| 3.14.2 | Provide protection from malicious code | Not implemented. AeroDocs does not scan uploaded files for malware. Dropzone files are quarantined (separate directory) but not scanned. Organization must implement antivirus/malware scanning. | Not Addressed |
| 3.14.3 | Monitor system security alerts and advisories | Not implemented. No built-in security advisory monitoring. Organization must monitor Go vulnerability databases and upstream dependencies. | Not Addressed |
| 3.14.4 | Update malicious code protection mechanisms | Not Applicable -AeroDocs does not include malware scanning; see 3.14.2. Organization responsibility. | Partially Addressed |
| 3.14.5 | Perform periodic scans and real-time monitoring | Heartbeat monitoring provides real-time agent health checks (15s interval). Audit log provides continuous event monitoring. Login attempt tracking with rate limiting detects brute-force attempts. No periodic security scans. | Partially Addressed |
| 3.14.6 | Monitor organizational systems to detect attacks and indicators of compromise | Audit log captures failed login attempts (`user.login_failed`), failed TOTP attempts (`user.login_totp_failed`), and all administrative actions. IP address tracking enables source identification. No automated anomaly detection or alerting. | Partially Addressed |
| 3.14.7 | Identify unauthorized use of organizational systems | Immutable audit log records all access with user identity, IP address, and timestamps. Admin can query by user, action, and date range to identify unauthorized access. No automated detection -requires manual review. | Fully Addressed |

---

## Coverage Summary

**Out of 109 practices assessed:**

| Category | Count | Percentage |
|----------|-------|------------|
| Fully Addressed | 35 | 32% |
| Partially Addressed | 29 | 27% |
| Not Applicable | 19 | 17% |
| Not Addressed | 26 | 24% |

**Excluding Not Applicable (90 applicable practices):**

| Category | Count | Percentage |
|----------|-------|------------|
| Fully Addressed | 35 | 39% |
| Partially Addressed | 29 | 32% |
| Not Addressed | 26 | 29% |

### Key Strengths

- **Authentication and Identity (IA):** 7 of 11 practices fully addressed. Mandatory 2FA, bcrypt, JWT token scoping, and password policy provide strong coverage.
- **Access Control (AC):** 10 of 22 fully addressed. RBAC, per-path permissions, and token-type scoping cover core access control needs.
- **Audit (AU):** 5 of 9 fully addressed. Immutable, append-only audit log with 25 event types and IP tracking is a standout feature.

### Key Gaps

- **No FIPS-validated cryptography (3.13.11):** Would require Go BoringCrypto build.
- **No encryption at rest (3.13.16):** Relies on OS-level disk encryption.
- **No malware scanning (3.14.2):** Dropzone files are quarantined but not scanned.
- **No account inactivity timeout (3.5.6):** Inactive accounts remain active until manually deleted.
- **No password history (3.5.8):** Users can reuse previous passwords.
- **No automated alerting (3.3.4, 3.14.3):** Security events are logged but no notification mechanism exists.
- **No SIEM integration:** Audit logs are queryable via API but no export to external security tools.

### Recommendations for Organizations Pursuing CMMC L2

1. **Implement OS-level disk encryption** (LUKS/dm-crypt) on Hub and Agent servers to address 3.13.16.
2. **Build Hub with BoringCrypto** (`GOEXPERIMENT=boringcrypto`) if FIPS validation is required (3.13.11).
3. **Deploy file scanning** on Agent servers to scan dropzone uploads (3.14.2).
4. **Implement network segmentation** placing Hub behind a firewall with deny-by-default rules (3.13.6).
5. **Integrate audit log export** with organizational SIEM for automated alerting (3.3.4, 3.14.3).
6. **Establish organizational policies** for Awareness & Training (AT), Personnel Security (PS), and Physical Protection (PE) domains -these are inherently outside any application's scope.
