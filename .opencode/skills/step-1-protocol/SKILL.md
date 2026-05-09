---
name: step-1-protocol
description: Implement binary protocol for TCP tunnel system
---

# Step 1 - Protocol System

You are implementing a binary protocol.

Project:

TCP reverse tunnel system.

---

# Goals

Implement:

- protocol/message.go
- protocol/encoder.go
- protocol/decoder.go

---

# Requirements

Binary format:

| Magic | Type | Length | Payload |

- Magic: uint32
- Type: uint8
- Length: uint32

Support:

- REGISTER
- OPEN
- DATA
- CLOSE
- PING
- PONG

---

# Constraints

- Must handle TCP sticky packets
- Use BigEndian
- Validate magic number
- Return clear errors

---

# Output

- Full compilable Go code
- Encode()
- Decode()

---

# Definition of Done

- Encode/Decode roundtrip works
- Invalid packets rejected