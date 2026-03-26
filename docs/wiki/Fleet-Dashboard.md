# Fleet Dashboard

The Fleet Dashboard is the main screen of AeroDocs. It shows all the servers that have been registered with your Hub.

![Fleet Dashboard](../screenshots/05-fleet-dashboard.png)

> **Admin vs Viewer:** All users can view the Fleet Dashboard, search/filter servers, and click into server details. Only admins can add, edit, or unregister servers.

---

## Overview

Each server is displayed as a card showing:

- **Server name** — the label you gave it when adding it
- **Status indicator** — a coloured dot showing whether the server is reachable
- **Hostname and IP address** — populated automatically when the agent registers
- **Operating system** — detected by the agent
- **Agent version** — the version of the AeroDocs agent running on that server
- **Labels** — any tags you assigned when creating the server
- **Last seen** — when the Hub last received a heartbeat from the agent

---

## Server Status Indicators

| Colour | Meaning |
|--------|---------|
| Green | Online — the agent is connected and responding |
| Red | Offline — the agent was previously connected but is no longer reachable |
| Amber / Yellow | Pending — the server record has been created but the agent has not registered yet |

A server stays in "pending" state until you run the install command on that machine. Once the agent runs and calls home, the status moves to "online."

---

## Adding a Server

Only admins can add servers.

1. Click the **Add Server** button in the top-right corner of the dashboard.
2. The Add Server modal will appear.

![Add Server Modal](../screenshots/06-add-server-modal.png)

3. Enter a **Name** for the server (this is just a label — it does not have to match the actual hostname).
4. Optionally add **Labels** to help you organise servers (e.g. `env:production`, `region:eu-west`).
5. Click **Add Server**.
6. AeroDocs creates the server record and generates a one-time install command. It will look something like:

   ```
   curl -fsSL https://aerodocs.example.com/install.sh | bash -s -- --token <token>
   ```

7. Copy the command and run it on the target server as root (or with `sudo`). The script downloads and installs the AeroDocs agent, then registers the server with the Hub using the embedded token.
8. Once the agent connects, the server card changes from amber (Pending) to green (Online) automatically — no page refresh needed.

The registration token is single-use and expires after a short period. If you do not run the command in time, delete the server and add it again to get a fresh token.

---

## Auto-Refresh

The Fleet Dashboard polls the Hub every **10 seconds** and updates all server cards in place. You do not need to reload the page to see status changes.

---

## Opening a Server

Click a **server name** (or anywhere on the server card body) to open the [[Server Detail]] page for that server. From there you can browse the remote filesystem, tail logs, and upload files.

---

## Filtering and Searching

Use the search bar at the top of the dashboard to filter servers by name. You can also filter by status using the status dropdown (All / Online / Offline / Pending).

---

## Selecting Multiple Servers

Click the checkbox on any server card to select it. A toolbar will appear at the bottom of the screen showing how many servers are selected and offering bulk actions.

---

## Unregistering Servers

Only admins can unregister servers.

> **Warning:** Unregistering a server is permanent. The server record is deleted from the Hub database and cannot be recovered. If the agent is online, it will be automatically uninstalled from the remote machine. To re-add the server later, you must go through the full add-and-install process again.

**Single server:** Click the three-dot menu on a server card and choose **Unregister**. You will be asked to confirm.

**Multiple servers:** Select the servers you want to remove using the checkboxes, then click **Unregister Selected** in the bulk action toolbar.

### What Unregister does

When you unregister a server, the Hub sends a cleanup command to the agent on that machine. The agent will:

1. Stop the `aerodocs-agent` systemd service
2. Remove the agent binary
3. Remove the configuration file (`/etc/aerodocs/agent.conf`)
4. Remove the dropzone staging directory

Once cleanup is confirmed, the server record is deleted from the Hub database.

**If the agent is offline** (unreachable at the time of unregistration), the Hub skips the remote cleanup step and deletes the server record from the database only. You will need to clean up the agent files on that machine manually if desired.

The unregister action is recorded in the [[Audit Logs]] as `server.unregistered`.
