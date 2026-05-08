---
name: step-2-register
description: Implement client registration system
---

# Step 2 - Client Registration

You are implementing client registration.

Project:

TCP reverse tunnel system.

---

# Goals

Implement:

- server/session.go
- server/client_manager.go

---

# Requirements

- Support multiple clients
- Unique clientId
- Store active sessions
- Reject duplicate clientId

---

# Constraints

- Thread-safe
- No global state
- Handle disconnect cleanly

---

# Output

## Struct Design
## Go Code
## Session Lifecycle

---

# Definition of Done

Server logs:

client connected: node-1