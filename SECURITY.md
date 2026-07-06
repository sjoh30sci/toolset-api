# Security Policy

## Supported Versions

| Version | Supported |
| ------- | --------- |
| 0.1.x   | ✅        |

## Reporting a Vulnerability

Please **do not** open public issues for security vulnerabilities.

Report privately via [GitHub Security Advisories](https://github.com/yourusername/toolset-api/security/advisories/new)
or email the maintainer. Include:

- A description of the vulnerability and its impact.
- Steps to reproduce (proof-of-concept if possible).
- Affected version(s).

We aim to acknowledge reports within 72 hours and to provide a remediation
timeline after triage.

## Security Model

The Toolset API executes untrusted code and automates browsers. Deploy with the
following in mind:

- **Auth**: run in `auth.mode=token` for any non-loopback exposure. In
  `auth.mode=none` the gateway only accepts `127.0.0.1`/`::1`.
- **Sandboxing**: code execution runs inside sandboxed containers (nsjail).
  Never expose the exec service directly to untrusted networks.
- **File sandbox**: file operations are confined to `files.sandbox_root`. Paths
  are validated against traversal.
- **Rate limiting**: per-token/IP rate limits are enforced; tune `auth.rate_limit`.
- **Network isolation**: keep tool containers on the internal Docker network;
  only the gateway is intended to be reachable.
