# Security Policy

## Supported Versions

| Version | Supported |
|---|---|
| 0.3.x | Yes |
| < 0.3 | No |

## Reporting a Vulnerability

If you discover a security vulnerability in Evidra, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, email: security@samebits.com

Include:
- Description of the vulnerability
- Steps to reproduce
- Impact assessment
- Suggested fix (if any)

We will acknowledge receipt within 48 hours and provide a timeline for a fix.

## Scope

Security-relevant areas in Evidra include:
- Evidence chain integrity (hash-linking, signatures)
- Ed25519 signing key handling
- File-based locking and concurrent access
- SARIF parser input handling
