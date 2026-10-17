# CI job recipes

Ready-to-adapt GitLab CI jobs for ics-sync: syncing from the default branch, publishing the
output of a build job, pinning the image, picking runners by tag, and more. Every recipe uses
only what the CLI supports: `--config`, `--endpoint`, `--private-key`, `--commit-sha`,
`--commit-ts`, `-v` and `--version`, plus the environment variables `ICS_ENDPOINT`,
`ICS_PRIVATE_KEY`, `CI_COMMIT_SHA` and `CI_COMMIT_TIMESTAMP`.

All recipes assume the private key is a **File** variable named `ICS_PRIVATE_KEY` and that
`ICS_ENDPOINT` is set either in the job or as a CI/CD variable. See
[Set up a repository](setup.md) for both.

## The basic job: sync the default branch

```yaml
ics:sync:
  stage: deploy
  image:
    name: ghcr.io/inprojectspl/ics-sync:1
    entrypoint: [""]
    pull_policy: always
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
  variables:
    ICS_ENDPOINT: "https://ics.example.com"
  script:
    - ics-sync --config .inprojects.ics --private-key "@$ICS_PRIVATE_KEY"
```

| Line | Why |
|---|---|
| `entrypoint: [""]` | The image's entrypoint is the CLI itself. Resetting it lets GitLab Runner run the `script:` lines in the image's shell. |
| `pull_policy: always` | `:1` moves to every new `1.x.y` release; this makes the runner pull the current one instead of a cached copy. |
| `rules` | Only pushes to the default branch publish. Merge request pipelines and other branches skip the job. |
| `"@$ICS_PRIVATE_KEY"` | A File variable holds a path. The `@` tells the CLI to read the key from that path. |

The CLI takes the commit SHA and timestamp from GitLab's `CI_COMMIT_SHA` and
`CI_COMMIT_TIMESTAMP`, so the job needs no extra variables.

> [!NOTE]
> If your runner allows only the `if-not-present` pull policy, the job fails with
> `pull_policy: always`. Remove the line and pin a release tag instead (see
> [Pin the image](#pin-the-image)), because a cached `:1` would never update.

## Sync only when the content changed

Every successful sync creates a new version, even when no file changed. To skip pipelines that
touch only code or CI files, add `changes:` to the rule:

```yaml
ics:sync:
  stage: deploy
  image:
    name: ghcr.io/inprojectspl/ics-sync:1
    entrypoint: [""]
    pull_policy: always
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
      changes:
        - docs/**/*
        - .inprojects.ics
  script:
    - ics-sync --config .inprojects.ics --private-key "@$ICS_PRIVATE_KEY"
```

Here `ICS_ENDPOINT` comes from a project or group CI/CD variable.

## Publish the output of a build job

When the content is generated (a static site, an HTML report), build it in one job and publish
it from the next. The build job saves the folder as an artifact; `needs:` makes the sync job
start as soon as the build finishes and download that artifact into the same path.

```yaml
stages:
  - build
  - deploy

site:build:
  stage: build
  image: node:22-alpine
  script:
    - npm ci
    - npm run build          # writes the site to public/
  artifacts:
    paths:
      - public/
    expire_in: 1 day
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH

ics:sync:
  stage: deploy
  needs: ["site:build"]
  image:
    name: ghcr.io/inprojectspl/ics-sync:1
    entrypoint: [""]
    pull_policy: always
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
  script:
    - ics-sync --config .inprojects.ics --private-key "@$ICS_PRIVATE_KEY"
```

With this binding file:

```yaml
version: 1
binding_id: 1eb8ec80-927a-4087-9c1f-262db49441bb
publish:
  source: public
  kind: html_bundle
```

Give both jobs the same `rules:`. If the build job does not run, `needs:` makes the pipeline
invalid, and if the artifact is missing the CLI stops with `bundle: stat …/public: no such file
or directory`.

## Pin the image

`:1` follows every `1.x.y` release. For a job that must not change without a commit, pin a
release or an exact build:

```yaml
ics:sync:
  stage: deploy
  image:
    name: ghcr.io/inprojectspl/ics-sync:<x.y.z>
    entrypoint: [""]
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
  script:
    - ics-sync --version
    - ics-sync --config .inprojects.ics --private-key "@$ICS_PRIVATE_KEY"
```

| Tag | Published for | Changes |
|---|---|---|
| `:1` | every `v1.x.y` release | moves to each new 1.x release |
| `:<x.y.z>` | the release `v<x.y.z>` | fixed |
| `:<commit sha>` | every commit on `main` of ics-sync (full 40-character SHA) | fixed |
| `:latest` | every commit on `main` of ics-sync | moves on every commit; not for production jobs |

Replace `<x.y.z>` with a release: one of the `v1.x.y`
[tags of ics-sync](https://github.com/inprojectspl/ics-sync/tags), without the `v`. A pinned tag does not need
`pull_policy: always`. `ics-sync --version` prints the version in the job log, which helps when
you compare runs.

## Run on runners with tags

If your GitLab instance routes jobs by runner tags, add `tags:`. The ICS dialog suggests
`docker`; use whatever tag your Docker runners have:

```yaml
ics:sync:
  stage: deploy
  tags:
    - docker
    - internal-network
  image:
    name: ghcr.io/inprojectspl/ics-sync:1
    entrypoint: [""]
    pull_policy: always
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
  script:
    - ics-sync --config .inprojects.ics --private-key "@$ICS_PRIVATE_KEY"
```

The runner needs a Docker executor (or Docker in Docker) and a network path to ICS. If ICS is
reachable only from inside your network, pick a runner that is inside it.

## Use a plain variable instead of a File variable

If the key is stored as a plain **Variable**, drop `--private-key`. The CLI reads the PEM from
`$ICS_PRIVATE_KEY` on its own, and the key does not appear on the command line:

```yaml
  script:
    - ics-sync --config .inprojects.ics
```

## Publish several documents from one repository

With one binding file and one key variable per document (see
[the binding file recipe](binding-file.md#publish-two-folders-into-two-documents)):

```yaml
ics:sync:
  stage: deploy
  image:
    name: ghcr.io/inprojectspl/ics-sync:1
    entrypoint: [""]
    pull_policy: always
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
  script:
    - ics-sync --config docs/handbook.ics --private-key "@$ICS_KEY_HANDBOOK"

ics:sync:runbooks:
  extends: ics:sync
  script:
    - ics-sync --config docs/runbooks.ics --private-key "@$ICS_KEY_RUNBOOKS"
```

Use one job per document. In a single job the lines run in order and the job stops at the first
failure; when the first document is stored and the second fails, a retry of the whole job stops
at the first one again with `409 replay_rejected` and never reaches the second. With separate
jobs each one is retried on its own.

## Publish on tags as well

To publish when you tag a release, add a rule for tags:

```yaml
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
    - if: $CI_COMMIT_TAG
```

With **Protect variable** on, the tags must be protected too, or the job gets no key. Mind the
[replay rule](how-it-works.md#replay-protection): a tag pipeline for a commit that was already
synced from the branch carries the same commit timestamp and is refused with `409`. Tag-only
publishing (only the `$CI_COMMIT_TAG` rule) avoids that.

## Run the CLI outside GitLab

To test a binding from your machine, pass the values GitLab would provide:

```sh
ics-sync \
  --config .inprojects.ics \
  --endpoint https://ics.example.com \
  --private-key @/path/to/private.pem \
  --commit-sha "$(git rev-parse HEAD)" \
  --commit-ts "$(git log -1 --format=%cI)" \
  -v
```

Keep in mind that a successful manual run moves the binding's timestamp forward: the next CI run
for an older commit is then refused.

## Related

- [Set up a repository](setup.md)
- [The binding file](binding-file.md)
- [How it works](how-it-works.md)
- [Troubleshooting](troubleshooting.md)
