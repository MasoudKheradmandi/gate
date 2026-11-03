# Security Policy

Corvus sits in front of your services, so security reports are handled with priority.

## Reporting a vulnerability

Please **do not open a public issue** for a vulnerability.

Report it privately through GitHub:
**[Security → Report a vulnerability](https://github.com/corvus-gateway/corvus/security/advisories/new)**.

Include, if you can:

- the affected version or commit,
- the component (gateway / Lua, control plane / API, admin console, deployment),
- steps to reproduce or a proof of concept,
- the impact you expect (e.g. rule bypass, authentication bypass, information disclosure).

You will get an acknowledgement within 7 days. Once a fix is ready, it is released together with
a GitHub security advisory, and reporters are credited unless they prefer otherwise.

## Supported versions

Security fixes are released for the latest minor version of 1.x. Upgrade with
`docker compose pull && docker compose up -d`.

## Scope notes

- Detection signatures and virtual-patch templates reduce risk but cannot catch every attack. A
  bypass of a signature is welcome as a report, but it is treated as a rule improvement rather than
  a vulnerability unless it bypasses an explicit guarantee (for example, a *Block* security rule).
- Managed rule bundles are accepted only when signed with the project key. A way to make the
  gateway accept an unsigned or downgraded bundle **is** a vulnerability.
