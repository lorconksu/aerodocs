# Audit Logs

## What Are Audit Logs?

Every action taken in AeroDocs is recorded in the audit log. Think of it as an activity history — a permanent record of who did what and when.

Audit log entries cannot be edited or deleted, even by admins. This makes the log trustworthy: if something changed, there will be a record of it.

The audit log is only visible to admins.

---

## Viewing the Audit Log

Navigate to **Audit Logs** in the sidebar.

![Audit Logs](../screenshots/07-audit-logs.png)

Each entry shows:

- **When** — the date and time of the action
- **Who** — the username of the person who performed the action (or "System" for automated actions)
- **Action** — what was done (see action types below)
- **Target** — the resource that was affected (e.g. a username or server name)
- **Details** — any additional context
- **IP Address** — the IP address the request came from

---

## Filtering the Log

Use the filters at the top of the page to narrow down the log:

- **Date range** — Show entries between a start and end date/time
- **User** — Filter to actions by a specific user
- **Action type** — Filter to a specific category of action

You can combine filters. Click **Clear** to reset them.

---

## Understanding Action Types

Actions follow a `resource.action` naming pattern.

### User actions

| Action | What it means |
|--------|--------------|
| `user.login` | A user successfully logged in |
| `user.login_failed` | A login attempt failed (wrong password) |
| `user.login_totp_failed` | Password was correct but the TOTP code was wrong |
| `user.registered` | The initial admin account was created |
| `user.totp_setup` | A user started the TOTP setup process |
| `user.totp_enabled` | A user successfully enabled TOTP (confirmed their code) |
| `user.totp_disabled` | An admin disabled TOTP for a user |
| `user.totp_reset` | TOTP was reset via the CLI break-glass command |
| `user.created` | An admin created a new user account |
| `user.password_changed` | A user changed their password |
| `user.role_updated` | An admin changed a user's role |
| `user.deleted` | An admin deleted a user account |

### Server actions

| Action | What it means |
|--------|--------------|
| `server.created` | An admin added a new server record |
| `server.updated` | An admin edited a server's name or labels |
| `server.deleted` | An admin deleted a server |
| `server.batch_deleted` | An admin deleted multiple servers at once |
| `server.registered` | An agent ran the install command and registered with the Hub |
| `server.connected` | An agent established a live WebSocket connection to the Hub |
| `server.disconnected` | An agent's WebSocket connection to the Hub dropped |

### File actions

| Action | What it means |
|--------|--------------|
| `file.read` | A user viewed or downloaded a file via the file browser |
| `file.uploaded` | A user uploaded a file to a server via the Dropzone |

### Path access actions

| Action | What it means |
|--------|--------------|
| `path.granted` | An admin granted a user access to a filesystem path on a server |
| `path.revoked` | An admin revoked a user's access to a filesystem path on a server |

### Log actions

| Action | What it means |
|--------|--------------|
| `log.tail_started` | A user started a live log tail session on a server |
