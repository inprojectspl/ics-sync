# Changelog

All notable changes to this project are documented in this file. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the
project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- First public release: deterministic tar.gz bundle, Ed25519-signed manifest
  and a streamed multipart upload to `POST /api/bindings/{id}/sync`, shipped
  as `ghcr.io/inprojectspl/ics-sync`.
- Documentation site at <https://inprojectspl.github.io/ics-sync/>, built with
  MkDocs from `docs/`.

### Fixed

- An unreadable `--private-key @...` value is no longer repeated in the error message. With a
  text CI variable passed with `@`, the message used to print the private key into the job log.
