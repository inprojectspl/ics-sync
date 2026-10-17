# Changelog

All notable changes to this project are documented in this file. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the
project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [1.1.1] - 2026-10-17

### Fixed

- A sync counts as done only when ICS answers `201` with the new version: the CLI no longer
  follows redirects (a proxy login page answered `200` and the job passed), and fails on a
  body it cannot read in full or that is not a version.
- Unknown keys in the binding file and malformed exclude patterns are errors, so a typo such as
  `soruce` no longer publishes the whole working directory.
- The archive being written is never packed into itself when `TMPDIR` lies under
  `publish.source`.

## [1.1.0] - 2026-10-17

### Added

- First public release: deterministic tar.gz bundle, Ed25519-signed manifest
  and a streamed multipart upload to `POST /api/bindings/{id}/sync`, shipped
  as `ghcr.io/inprojectspl/ics-sync`.
- Documentation site at <https://inprojectspl.github.io/ics-sync/>, built with
  MkDocs from `docs/`.

### Fixed

- An unreadable `--private-key @...` value is no longer repeated in the error message. With a
  text CI variable passed with `@`, the message used to print the private key into the job log.
