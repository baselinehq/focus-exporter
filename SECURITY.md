# Security Policy

## Reporting a vulnerability

Please do not report security vulnerabilities through public GitHub issues,
discussions, or pull requests.

Report privately using GitHub's
[private vulnerability reporting](https://github.com/baselinehq/focus-exporter/security/advisories/new),
or email **support@costgraph.ai**.

Include as much of the following as you can:

- The affected version or commit.
- A description of the issue and its impact.
- Steps to reproduce, or a proof of concept.
- Any suggested mitigation.

We will acknowledge your report within 3 business days and keep you informed as
we work on a fix.

## Scope

focus-exporter is a client that reads provider billing APIs using credentials
you supply. Of particular interest:

- Handling of provider credentials (service tokens, API keys) passed via
  environment variables.
- Any path that could leak a credential into logs, output, or error messages.
- Parsing of untrusted provider responses.

## Credentials

Provider credentials are read from environment variables and sent only to the
provider's own API over HTTPS. They are never written to the FOCUS output or to
logs. If you find a case where a credential could be exposed, treat it as a
security issue and report it privately as above.
