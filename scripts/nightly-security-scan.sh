#!/bin/bash
set -euo pipefail

# AeroDocs Nightly Security Scan
# Runs at 3am EST via cron. Uses Claude Code CLI to:
# 1. Run security review, code review, and CTF penetration test
# 2. Fix all findings
# 3. Re-scan to verify fixes
# 4. Push, tag, and deploy if changes were made
#
# Cron entry (3am EST = 8am UTC):
#   0 8 * * * /home/wyiu/personal/aerodocs/scripts/nightly-security-scan.sh >> /var/log/aerodocs-nightly.log 2>&1

PROJECT_DIR="/home/wyiu/personal/aerodocs"
LOG_DIR="/var/log"
TIMESTAMP=$(date +%Y-%m-%d)
LOG_FILE="${LOG_DIR}/aerodocs-nightly-${TIMESTAMP}.log"

# Ensure we're in the project directory
cd "$PROJECT_DIR"

# Pull latest code
git pull --rebase origin main 2>&1 || true

echo "=== AeroDocs Nightly Security Scan — ${TIMESTAMP} ==="
echo "Started at: $(date)"

# Run Claude Code in non-interactive mode with the full scan prompt
/home/wyiu/.local/bin/claude --dangerously-skip-permissions -p "$(cat <<'PROMPT'
You are running as a nightly automated security scan for the AeroDocs project.
Your working directory is /home/wyiu/personal/aerodocs.

## Your Mission

Perform a complete security review cycle:

### Phase 1: Scan (parallel)
Run these 3 scans in parallel using agents:

1. **Security Review** — Full codebase audit (OWASP Top 10, auth, crypto, input validation, injection, XSS, CSRF). Check hub/, agent/, web/. Write report to /home/wyiu/personal/aerodocs-internal/vulnerabilities/YYYY-MM-DD-security-review.md (use today's date). Compare against previous reports in that directory to only report NEW findings.

2. **Code Review** — Review all changes since the last release tag. Check for correctness, race conditions, error handling, security issues. Write report to /home/wyiu/personal/aerodocs-internal/engineering/YYYY-MM-DD-code-review.md.

3. **CTF Penetration Test** — White-box penetration test against https://aerodocs.yiucloud.com. You have full source code access. Test auth bypass, injection, path traversal, API abuse, CSRF, header security, TLS, cookie security. Use curl for all tests. Write report to /home/wyiu/personal/aerodocs-internal/vulnerabilities/YYYY-MM-DD-ctf-report.md.

Use vertical tables for metadata in reports (severity, CVSS, type, location).

### Phase 2: Fix
If any findings were discovered:
- Fix all findings that can be safely auto-fixed (vulnerabilities, code issues)
- Run `go test ./...` (from hub/ and agent/) and `npx vitest run` (from web/) to verify
- For findings that can't be auto-fixed (architectural, breaking changes), open GitHub issues on lorconksu/aerodocs with the finding details

### Phase 3: Re-scan
If fixes were made, re-run the security review on the fixed code to verify:
- All previously found issues are resolved
- No new issues were introduced by the fixes
- Update the report with FIXED status for resolved findings

### Phase 4: SonarCloud
Check SonarCloud quality gate using the API:
  curl -s -u "$SONAR_TOKEN:" "https://sonarcloud.io/api/qualitygates/project_status?projectKey=lorconksu_aerodocs"

If there are any failures (vulnerabilities, code smells, coverage < 90%, unreviewed hotspots):
- Fix all code issues
- Mark hotspots as reviewed via the API
- Run tests to verify

### Phase 5: Release (only if changes were made)
If any code was changed:
1. Commit all changes with message: "security: nightly scan fixes — YYYY-MM-DD"
2. Push to main
3. Create next version tag (check `git tag --sort=-v:refname | head -1` for current version, increment patch)
4. Push the tag (triggers release pipeline)
5. Wait for the release pipeline to complete: `gh run watch <id> --exit-status`
6. Deploy to production:
   - `ssh -i ~/.ssh/yiucloud cloud-user@10.10.1.96 "sudo docker pull yiucloud/aerodocs:<version> && sudo sed -i 's|yiucloud/aerodocs:[0-9.]*|yiucloud/aerodocs:<version>|' /opt/aerodocs/docker-compose.yml && cd /opt/aerodocs && sudo docker compose up -d"`
7. Verify: `curl -s -o /dev/null -w "%{http_code}" https://aerodocs.yiucloud.com/login` should return 200

### Phase 6: Report
Commit and push all reports to lorconksu/aerodocs-internal:
  cd /home/wyiu/personal/aerodocs-internal && git add -A && git commit -m "docs: nightly scan reports — YYYY-MM-DD" && git push

## Environment
- SONAR_TOKEN is in ~/.bashrc
- SSH key for deployment: ~/.ssh/yiucloud
- GitHub CLI (gh) is authenticated
- Project: /home/wyiu/personal/aerodocs
- Internal docs: /home/wyiu/personal/aerodocs-internal
- Deploy target: LXC 110 at 10.10.1.96, user cloud-user
- Domain: aerodocs.yiucloud.com
- Docker image: yiucloud/aerodocs

## Rules
- If no findings and no SonarCloud issues: just push clean reports, no release needed
- Never leave the site broken — verify deployment works before finishing
- All reports use vertical tables for metadata blocks
- Open GitHub issues for findings you can't auto-fix
PROMPT
)" 2>&1 | tee -a "$LOG_FILE"

echo ""
echo "Completed at: $(date)"
echo "=== End of nightly scan ==="
