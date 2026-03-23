# AeroDocs

A self-hosted infrastructure observability and documentation platform. Full-stack: Go backend (Hub), Go agent, React frontend, SQLite database.

## Architecture

Hub-and-Spoke model:
- **Hub**: Central Go server — serves the React frontend, exposes REST/gRPC APIs, manages SQLite database, enforces all auth and permissions.
- **Agent**: Lightweight Go binary installed on each remote server — takes orders from the Hub only. Agents never talk to each other, users never talk to Agents directly.
- **Frontend**: React SPA served by the Hub.

## Product Domain

Server fleet management, remote log tailing, markdown documentation rendering, and secure file transfer.

## Build Order (Sub-projects)

Each sub-project gets its own spec → plan → implementation cycle:

1. **Foundation & Auth** — Project scaffolding, SQLite schema, Go server skeleton, user auth (login/register/2FA), React app shell with routing
2. **Fleet Dashboard & Server Management** — Server CRUD API, agent registration protocol, fleet dashboard UI with mass actions
3. **Agent** — Lightweight Go binary on remote servers, Hub↔Agent communication protocol
4. **File Tree & Document Viewer** — "Honest" file tree browsing, markdown rendering, file reading via agent
5. **Live Log Tailing & Grep** — Real-time log streaming, grep/filter, reverse infinite scroll, log rotation resilience
6. **Quarantined Dropzone** — Drag-and-drop file upload to sandboxed directory
7. **Audit Logs & Settings** — Global audit log viewer, permissions management, user settings

## Tech Stack

### Backend
- **Language**: Go (Golang)
- **Database**: SQLite (single-file, no external DB server)
- **API**: REST + gRPC (Hub↔Agent communication)

### Frontend
- **Framework**: React with TypeScript
- **Styling**: Tailwind CSS with shadcn/ui (heavily customized — must feel fully bespoke, not a starter kit)
- **Routing**: React Router
- **Server state**: TanStack Query
- **Icons**: lucide-react
- **Layouts**: react-resizable-panels for multi-pane reading layouts

### Database Schema (SQLite)
- `users` — login info, 2FA/TOTP secrets
- `servers` — inventory of connected machines
- `permissions` — per-user, per-server folder access maps
- `audit_logs` — immutable history of all actions

## Implementation Requirements

- DRY coding principles throughout
- Modular, clean codebase
- Strong type-safety end to end (strict TypeScript on frontend, strong typing in Go)
- Prefer reusable primitives, shared helpers, shared layout shells, and domain-specific modules over duplicated logic
- Break large views into focused components, hooks, and utility files
- Centralize design tokens, route state helpers, and reusable panel patterns
- Avoid giant monolithic files
- Avoid `any` in TypeScript except as an absolute last resort
- Explicit interfaces for API contracts (Server Node, Log Line, File Node, etc.)
- Strongly typed UI state, filter state, and connection status (Online, Offline, Reconnecting)

## Frontend Design

### Design Goal
Serious, premium, high-trust interface — enterprise platform engineering command center meets advanced terminal.

### Visual Direction
- Dark, near-black UI with layered charcoal and graphite surfaces
- Dense but organized information layout
- Sharp, technical, operator-focused presentation
- Desktop-first app shell with workspace-style navigation

### Layout
- Top global telemetry bar: context-aware actions (Mass Delete, Add Server), global connection health, user profile (with 2FA settings dropdown)
- Left vertical nav: Fleet Dashboard, Global Audit Logs, Settings
- Server view: left sidebar becomes the "Honest" File Tree (directories, text files, greyed-out binaries/prohibited symlinks)
- Main content: split panes with breadcrumbs, file metadata, real-time Grep/Filter input. Main stage toggles between rendered Markdown view and dense monospaced terminal view for log tailing

### Typography
- Strong, technical heading style
- Uppercase micro-labels with generous tracking for metadata (file sizes, line counts)
- Monospace heavily: log streams, IPs, file paths, terminal outputs

### Color System
- Base: black, charcoal, slate, off-white
- Green: server online, live-tail connected, success
- Amber: network blip/buffering, warning
- Red: server offline, stream disconnected, errors
- No purple-heavy or trendy SaaS palettes

### Component Style
- Thin borders, subtle separators, faint grid structure
- Compact status pills and badges
- Dense data tables with mass-select checkboxes
- Log reader: native terminal feel, no excess padding
- Modals: "Add Server" flow, "Quarantined Dropzone" file uploader

### Interaction Style
- Fast, restrained, precise
- Grep bar filters visible lines immediately
- Subtle hover states on file tree
- No flashy animation or glassmorphism

### UX Principles
- High information density with strong hierarchy
- Prioritize readability of massive log walls
- Explicit error states (inline break on log rotation, connection drop indicators)

## Critical Edge Cases
- **Network Breakaway**: 15-second timeout → cut live-tail, show red error
- **CLI Break-Glass**: Terminal command to reset admin 2FA if phone is lost
- **Path Sanitization**: Aggressively scrub file paths to prevent traversal attacks (e.g., `../../passwords`)

## Reusable Components to Build
- Fleet status cards / table rows
- "Honest" File Tree node (active, directory, disabled/greyed-out states)
- Log Reading Stage (grep bar + line count telemetry)
- 2FA/TOTP setup modals
