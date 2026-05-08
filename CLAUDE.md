# Project: TCP Tunnel System

Goal:

Build a minimal TCP reverse tunnel system.

Phase-1 Scope:

- TCP only
- No TLS
- No HTTP

Architecture:

- Client connects to server
- Single control connection
- Multiple tunnels via multiplexing

Principles:

- Keep it simple
- Avoid over-engineering
- Log everything

Concurrency:

- Goroutine per connection
- No blocking operations