# Product Requirements Document (PRD): AeroDocs
**Version:** 1.1 | **Target Audience:** Project Managers, Junior Engineers, and Platform Leaders

## 1. Executive Summary
**What it is:** AeroDocs is a web-based dashboard that acts as a secure control center for our server infrastructure. Instead of engineers typing terminal commands to read logs or check documentation on remote computers, they can use this clean web interface. 

**Why we are doing it:** To save time, reduce the risk of human error, and improve security. It allows the team to troubleshoot issues and read system documents without needing direct, highly privileged SSH (Secure Shell) access to every machine.

## 2. Core User Workflows & UI Behavior

### 2.1 Fleet Dashboard (The Home Screen)
* **What it is:** The main landing page showing a list or grid of all the servers connected to AeroDocs.
    * **Telemetry:** Displays basic health stats like the server's name, IP address, operating system, and a green/red dot showing if it is currently online.
    * **Mass Actions:** Checkboxes next to each server allow users to select multiple machines at once to delete or manage them.
* **Why we are doing it:** A Project Manager or Engineer needs to know the overall health of the system at a single glance. By grouping actions (like mass delete) behind checkboxes, we keep the screen from looking cluttered with buttons.

### 2.2 Server Onboarding & Agent Deployment
* **What it is:** The process of connecting a brand new server to the AeroDocs dashboard. 
    * **The "Agent":** A tiny piece of software installed on the target server that listens for instructions from the main AeroDocs Hub.
    * **The Flow:** The UI gives the user a single line of code (a `curl` command) to copy and paste into the new server. 
* **Why we are doing it:** Adding servers should be painless. Instead of a 10-step manual setup, providing a single copy-paste command means even a junior engineer can securely attach a new server to the fleet in under 30 seconds.

### 2.3 The "Honest" File Tree & Reading Stage
* **What it is:** A visual folder browser on the left side of the screen, just like Finder on a Mac or File Explorer on Windows. 
    * **Markdown Auto-Render:** If the user clicks a `.md` documentation file, it displays beautifully formatted text. If they click a `.log` file, it displays as raw code.
    * **Honest Display:** It shows *everything* in the folder. If a file is a binary (like an unreadable compiled program) or a shortcut pointing to a forbidden folder, it is greyed out and unclickable.
* **Why we are doing it:** We call it "Honest" because we never want to hide files from our engineers—hiding files causes confusion during an outage. However, greying out forbidden files physically prevents users from breaking the system or accessing sensitive areas.

### 2.4 Live Log Tailing & Built-in Grep (Search)
* **What it is:** "Tailing" means watching a log file update in real-time as the server runs. "Grep" is a search bar that instantly filters those lines.
    * **Reverse Infinite Scroll:** Just like scrolling up on social media loads older posts, scrolling up in our log reader seamlessly fetches older log lines.
    * **Log Rotation Resilience:** Servers automatically archive old logs and start new ones to save space. Our tool detects this and seamlessly switches to the new file without the user doing anything.
* **Why we are doing it:** Server logs can be gigabytes in size. If a web browser tries to load a 5GB file all at once, the computer will crash. The infinite scroll and built-in search ensure the tool stays lightning-fast, only loading exactly what the engineer is looking at.

### 2.5 The Quarantined Dropzone (File Uploads)
* **What it is:** A drag-and-drop menu to upload files to a remote server. However, files are *only* allowed to go into a specific, isolated folder called the "Dropzone" (e.g., `/tmp/aerodocs_uploads/`).
* **Why we are doing it:** If users could upload files anywhere, someone could accidentally overwrite a critical database configuration and take down the company. The Dropzone ensures files arrive safely, but an engineer still must intentionally move them to their final destination.
