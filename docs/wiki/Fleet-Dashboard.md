# Fleet Dashboard

The Fleet Dashboard is the main screen of AeroDocs. It shows all the servers that have been registered with your Hub.

![Fleet Dashboard](../screenshots/05-fleet-dashboard.png)

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
6. AeroDocs generates a one-time install command. It will look something like:

   ```
   curl -fsSL https://aerodocs.example.com/install.sh | bash -s -- --token <token>
   ```

7. Copy the command and run it on the target server as root (or with sudo).
8. The agent installs, registers with the Hub using the token, and the server card will change from amber to green within a few seconds.

The registration token is single-use and expires after a short period. If you do not run the command in time, delete the server and add it again to get a fresh token.

---

## Filtering and Searching

Use the search bar at the top of the dashboard to filter servers by name. You can also filter by status using the status dropdown (All / Online / Offline / Pending).

---

## Selecting Multiple Servers

Click the checkbox on any server card to select it. A toolbar will appear at the bottom of the screen showing how many servers are selected and offering bulk actions.

---

## Deleting Servers

**Single server:** Click the three-dot menu on a server card and choose **Delete**. You will be asked to confirm.

**Multiple servers:** Select the servers you want to remove using the checkboxes, then click **Delete Selected** in the bulk action toolbar.

Deleting a server removes it from the Hub's database. It does not uninstall the agent from the remote machine — you will need to do that manually if required.
