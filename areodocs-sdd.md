# Software Design Document (SDD): AeroDocs

## 1. System Architecture
* **What it is:** The software is split into two halves: The **Hub** and the **Agent**.
    * **The Hub:** The central "brain" and web server. It holds the database, runs the website, and acts as the ultimate security bouncer.
    * **The Agent:** The "hands." A small, dumb program running on each remote server that only takes orders from the Hub.
* **Why we are doing it:** This is called a "Hub-and-Spoke" model. It is highly secure because the Agents never talk to each other, and users never talk directly to the Agents. Everything must pass through the Hub's security checks first.

## 2. Tech Stack Requirements
* **What it is:** The specific programming languages and tools we are using.
    * **Frontend (The Visuals):** React, TypeScript, Tailwind CSS. These build the actual buttons, charts, and text the user clicks.
    * **Backend (The Logic):** Go (Golang) and SQLite. Go is a fast, compiled language that handles the heavy lifting and networking. SQLite is a lightweight database that stores our data in a single file.
* **Why we are doing it:** This stack is the modern standard for fast, reliable infrastructure tools. Go is incredibly lightweight, meaning our Agent won't slow down the servers it is monitoring. SQLite means we don't have to manage complex external database servers.

## 3. SQLite Data Schema (The Source of Truth)
* **What it is:** How information is organized inside the Hub's brain. 
    * **`users` table:** Stores login info and Two-Factor Authentication (2FA) secrets.
    * **`servers` table:** A simple inventory list of every connected machine.
    * **`permissions` table:** The strict map of exactly which folders each user is allowed to view on each server.
    * **`audit_logs` table:** A permanent, un-erasable history of who did what and when.
* **Why we are doing it:** Structuring the data this way guarantees strict access control. If an action isn't explicitly permitted in the `permissions` table, the Hub automatically blocks it. The `audit_logs` ensure that if something breaks, we have a clear history to review.

## 4. API & Data Contracts (How the Hub and Agent Talk)
* **What it is:** "Contracts" are strict rules defining exactly what a message looks like when the Hub asks the Agent for something (like a file stream).
* **Why we are doing it:** By defining these rules clearly up front, the engineer building the frontend (React) and the engineer building the backend (Go) can work completely independently. As long as they both follow the contract, the pieces will snap together perfectly at the end.

## 5. Critical Edge Cases & State Machines
* **What it is:** How the software behaves when things go horribly wrong.
    * **Network Breakaway:** If the internet connection drops for more than 15 seconds, the system instantly cuts the live-tail session and displays a red error. 
    * **The CLI Break-Glass:** If the main Administrator loses their phone and gets locked out of their 2FA, there is a hidden terminal command they can run directly on the server to reset their access.
    * **Path Sanitization:** The code aggressively scrubs any file paths requested by the user to ensure they aren't trying to hack the system (e.g., trying to read `../../passwords`).
* **Why we are doing it:** Software doesn't exist in a perfect vacuum. Wi-Fi drops, phones get lost, and users make typos. Defining exactly how the system handles these failures ensures it fails safely and predictably, rather than crashing or creating a security hole.`
