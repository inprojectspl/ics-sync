# Security policy

## Supported versions

Security fixes are released for the latest `1.x` version only. The `:1`
image tag always points at it.

## Reporting a vulnerability

Please do **not** open a public issue or pull request for a security
problem. Report it privately through
[GitHub private vulnerability reporting](https://github.com/inprojectspl/ics-sync/security/advisories/new)
(the **Report a vulnerability** button on the repository's Security tab).

Include the version (`ics-sync --version`) or image tag, a description of the
issue and its impact, and steps to reproduce it.

## Scope

The CLI runs in CI with the binding's private key. Of particular interest:

- anything that makes the CLI read or upload files outside `publish.source`,
  or leak the private key (logs, error messages, the bundle);
- weaknesses in the signed manifest or the bundle hash;
- the build and release pipeline of the container image.

Problems in the ICS server itself (signature verification, replay protection,
bundle extraction) belong to the ICS project.
