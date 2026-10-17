# Troubleshooting

When a sync fails, the last line of the CI job log says why. This page lists the messages the
CLI prints and the responses ICS returns, each with its cause and fix.

## Read the error

Every CLI error is one line starting with `ics-sync:`, and the job fails with exit code 1.
There are two kinds:

- **The CLI stopped before uploading** (a problem with the binding file, the key, the source
  folder or the network). The message comes from the CLI itself, for example
  `ics-sync: config: binding_id missing`.
- **ICS refused the upload.** The CLI prints the URL, the HTTP status and the body ICS sent:

  ```
  ics-sync: https://ics.example.com/api/bindings/1eb8ec80-927a-4087-9c1f-262db49441bb/sync returned 409: {"detail":"replay or out-of-order push","code":"replay_rejected"}
  ```

  The `code` value is stable; look it up in [Responses from ICS](#responses-from-ics). The
  `detail` text is for people and may change.

An upload ICS refused (any error with a `code`) changes nothing: the document keeps its current
version. If the connection broke after ICS stored the version, the job fails although the version
exists; running the job again then gets `409 replay_rejected`, which means the content is already
in ICS.

## Errors from the binding file

| Message | Cause | Fix |
|---|---|---|
| `config: read .inprojects.ics: open .inprojects.ics: no such file or directory` | The binding file is not where `--config` points, relative to the job's directory. | Commit the file to the repository root, or pass the right path with `--config`. |
| `config: parse .inprojects.ics: yaml: …` | The file is not valid YAML, often because of broken indentation in `public_key` or an unquoted pattern starting with `*`. | Copy the file again from the binding dialog or fix the line the message names. Quote exclude patterns. |
| `config: unsupported version: 0 (want 1)` | `version:` is missing or not `1`. | Set `version: 1`. |
| `config: binding_id missing` | `binding_id:` is missing or empty. | Copy the ID from the binding dialog (step 2 of the setup screen). |
| `config: parse .inprojects.ics: yaml: unmarshal errors: … field soruce not found …` | A key the CLI does not know, usually a typo. | Fix the key the message names. The keys are listed in [The binding file](binding-file.md#fields). |
| `config: exclude "docs/[draft": syntax error in pattern` | A malformed exclude pattern. | Fix the pattern; see [Exclude files](binding-file.md#exclude-files). |

## Errors from the private key

### The key file cannot be opened

```
ics-sync: private key: cannot read the key file: no such file or directory
```

The command passes `--private-key "@$ICS_PRIVATE_KEY"`, but the variable is empty in this
pipeline. Usually the variable is **protected** and the pipeline runs on a branch or tag that is
not protected, or the environment scope does not match the job.

Fix: run the sync only on the protected default branch (see the `rules:` in
[CI job recipes](ci-recipes.md)), protect the branch or tag, or widen the variable's environment
scope.

```
ics-sync: private key: the value after @ is a PEM key, not a file path; pass it without @
```

The variable is a plain **Variable** (text), but the command still has the `@`, so the CLI was
given the key itself where it expected a path. The CLI does not print the key.

Fix: store the key as a **File** variable (the recommended setup), or keep it plain and remove
`--private-key` from the command so that the CLI reads `$ICS_PRIVATE_KEY` directly.

### Other key errors

| Message | Cause | Fix |
|---|---|---|
| `private key: missing key (--private-key or $ICS_PRIVATE_KEY)` | No `--private-key` flag and no `ICS_PRIVATE_KEY` variable in the job. | Add the variable in **Settings → CI/CD → Variables**, check its name, flags and scope. |
| `private key parse: not a PEM block` | The value is not a PEM key. Typically a **File** variable passed without `@` (the CLI gets the file's path instead of its content), or a value pasted without the `BEGIN` and `END` lines. | Use `--private-key "@$ICS_PRIVATE_KEY"` for a File variable. Paste the whole block, both marker lines included. |
| `private key parse: unexpected PEM type: PUBLIC KEY` | The public key was stored instead of the private one. | Store the block from **ICS_PRIVATE_KEY (variable value)**. If you no longer have it, rotate the key. |
| `private key parse: unexpected PEM type: …` (another type) | The PEM is not an unencrypted PKCS #8 private key. | Use the key ICS generated, unchanged. |
| `private key parse: expected Ed25519, got …` | A PKCS #8 key of another algorithm, not one from ICS. | Use the key ICS generated. |

If ICS accepts no key at all (`403 bad_signature`), see [below](#403-bad_signature).

## Other errors before the upload

| Message | Cause | Fix |
|---|---|---|
| `missing endpoint (--endpoint or $ICS_ENDPOINT)` | No ICS address. | Set `ICS_ENDPOINT` in the job's `variables:` or as a CI/CD variable, or pass `--endpoint`. |
| `commit-ts: parsing time "…"` | `--commit-ts` or `CI_COMMIT_TIMESTAMP` is not an RFC 3339 time. | GitLab's own value is valid; for manual runs use `git log -1 --format=%cI`. |
| `bundle: stat …: no such file or directory` | `publish.source` does not exist in the job, often because the build artifact was not downloaded. | Check the path (it is relative to the job's directory) and the `needs:` or `dependencies:` of the sync job. See [Publish the output of a build job](ci-recipes.md#publish-the-output-of-a-build-job). |
| `bundle: … is not a directory` | `publish.source` points at a file. | Point it at the folder that contains the file. |

## Network errors

These start with `POST <sync URL>:` and come from the connection, not from ICS.

| Message contains | Cause | Fix |
|---|---|---|
| `no such host`, `connection refused`, `i/o timeout` | The runner cannot reach ICS. | Check `ICS_ENDPOINT` and that this runner can reach the ICS host. Use a runner inside your network if ICS is internal (see [runner tags](ci-recipes.md#run-on-runners-with-tags)). |
| `x509: certificate signed by unknown authority` | ICS uses a certificate from a private CA, which the image does not trust. | Store your CA certificate as a File variable and set `SSL_CERT_FILE` to it in the job; the CLI then trusts the certificates in that file. |
| `Client.Timeout exceeded` | No response within 5 minutes, the CLI's limit for the whole request. | Check proxy timeouts in front of ICS and the bundle size. |
| `reading the response: …` | The connection broke while ICS was answering. | Check the document's history in ICS: the version may have been stored. Running the job again is safe; if it was stored, the retry gets `409 replay_rejected`. |
| `returned 302` (or another `3xx`) | Something in front of ICS redirects the request, often a login page of a proxy. The CLI does not follow redirects. | Point `ICS_ENDPOINT` at the ICS address itself, with `https://`, and let `/api/bindings/` through the proxy without a login. |
| `unexpected response, not a new version from ICS` | A `201` whose body is not the answer ICS gives for a new version, for example a page from a proxy. | Check that `ICS_ENDPOINT` reaches ICS and not another service. |

The CLI honours `HTTPS_PROXY`, `HTTP_PROXY` and `NO_PROXY` if your runners need a proxy.

## Responses from ICS

### 403 bad_signature

`{"detail":"bad signature","code":"bad_signature"}`

The private key in GitLab does not belong to this binding. Most often the key was rotated in ICS
and the variable still holds the old one; also check that the variable is the one for this
binding if the project has several.

Fix: rotate the key in ICS and put the new one into `ICS_PRIVATE_KEY`
([Rotate the key](keys-and-security.md#rotate-the-key)).

### 404 binding_not_found

`{"detail":"binding not found","code":"binding_not_found"}`

ICS has no binding with this ID. Either the binding was deleted, or `binding_id` in the binding
file has a typo.

Fix: open **Connect a GitLab repo** on the document and compare the IDs. If the binding is gone,
create a new one and update both the binding file and the variable.

### 404 not_found

`{"detail":"Not Found","code":"not_found"}`

The URL is not the sync endpoint, usually because `ICS_ENDPOINT` includes a path such as `/api`
or points at another site.

Fix: set `ICS_ENDPOINT` to the base address you open ICS at, for example
`https://ics.example.com`. A trailing `/` does not matter.

### 404 asset_not_found

`{"detail":"asset not found","code":"asset_not_found"}`

The binding exists, but its document has been deleted.

Fix: restore the document in ICS, or create a binding on another document and update the
binding file and the variable.

### 409 replay_rejected

`{"detail":"replay or out-of-order push","code":"replay_rejected"}`

The commit timestamp is not later than the one of the last accepted sync
([why](how-it-works.md#replay-protection)). Typical causes:

| Situation | Fix |
|---|---|
| You re-ran the job for a commit that was already synced, including a retry after a run that stored the version but lost the answer. | Nothing to fix: the content is already in ICS. Push a new commit to sync again. |
| A force push, amend or rebase left the branch tip with an older timestamp. | Push a new commit, or rotate the key to reset the clock. |
| An older pipeline finished after a newer one. | Nothing to fix: the newer content is already in ICS. |
| Two commits were pushed within the same second. | Push another commit. |
| A tag pipeline ran for a commit already synced from the branch. | Publish from one of the two only (see [Publish on tags](ci-recipes.md#publish-on-tags-as-well)). |
| A manual run with the current time moved the clock ahead of the CI commits. | Rotate the key, or wait for a newer commit. |

Rotating the key resets the clock, but it also means updating the variable in GitLab.

### 400 commit_from_future

`{"detail":"commit from the future","code":"commit_from_future"}`

The commit timestamp is more than 5 minutes ahead of the ICS server's clock. The commit was made
on a machine with a wrong clock, or `--commit-ts` was set by hand.

Fix: wait until the timestamp is no longer in the future, or push a commit made with a correct
clock. Also check the clock of the ICS server.

### 400 manifest_invalid or manifest_mismatch

`{"detail":"manifest invalid: …","code":"manifest_invalid"}` or
`{"detail":"binding_id mismatch","code":"manifest_mismatch"}`

The signed manifest does not have the form ICS expects, or names another binding than the URL.
With an unmodified CLI this points at a CLI and server that do not match, or a commit SHA longer
than 64 characters passed with `--commit-sha`.

Fix: use the current `:1` image (or a current release) and do not override `--commit-sha` with
anything but a commit SHA.

### 400 bundle_hash_mismatch

`{"detail":"bundle hash mismatch","code":"bundle_hash_mismatch"}`

The bundle ICS received is not the one the CLI hashed and signed: something between the runner
and ICS changed or cut the upload.

Fix: check proxies between the runner and ICS. Re-run the job.

### 400 content does not fit the document type

ICS checks the bundle against the type of the document. See
[What the content must look like](binding-file.md#what-the-content-must-look-like).

| Code | Detail | Fix |
|---|---|---|
| `no_files` | `the bundle contains no files` | `publish.source` is empty after excludes. Check the source path and the patterns. |
| `md_files_required` | `md_collection expects .md files` | The collection's bundle has no `.md` file. Point `publish.source` at the Markdown. |
| `md_single_requires_one_file` | `md_single expects exactly one .md file` | A Markdown document takes exactly one `.md` file. Narrow `publish.source` or exclude the others, or publish into a collection instead. |
| `html_entry_missing` | `an HTML bundle must contain at least one .html file` | Point `publish.source` at the built site, not at its sources. |
| `bundle_invalid_archive` | `bundle is not a tar.gz: …` | ICS could not unpack the bundle. The CLI always sends a valid archive, so check proxies as for `bundle_hash_mismatch`. |
| `bundle_entry_rejected` | `rejected entry (link/device): …` or `rejected entry (absolute path): …` | The archive has a link, a device or an absolute path. The CLI never writes these; the bundle was not made by ics-sync. |

### 413 limits

| Code | Detail | Fix |
|---|---|---|
| `bundle_too_many_files` | `too many files` | More files than the instance allows (500 by default). Exclude what the document does not need. |
| `bundle_too_large` | `bundle too large`, or `bundle too large: … B, limit … B` | The unpacked content is over the instance limit (100 MiB by default), or an HTML bundle is over the owner's HTML bundle limit (5 MB by default, set per user by an administrator). Exclude large files or ask an administrator. |
| `storage_quota_exceeded` | `storage quota exceeded: using … B of … B (short by … B)` | The document owner's storage is full. The new version counts against the **owner's** quota, even when someone else created the binding. Free space or ask an administrator for a larger quota. |
| `request_too_large` | `request_too_large` | The compressed upload is over the server's request limit. Make the bundle smaller. |

A `413` with an HTML page instead of JSON comes from a proxy in front of ICS (for example
nginx's `client_max_body_size`). Ask the administrator to raise it.

### 429 rate_limited

`{"detail":"rate_limited","code":"rate_limited"}`

ICS accepts at most 30 sync requests a minute from one address. Runners behind one NAT address
share that budget.

Fix: re-run the job after a minute. If it happens often, sync fewer documents per pipeline or
add `changes:` rules so that unchanged content does not sync.

## The job succeeded, but

| Symptom | Explanation |
|---|---|
| `"security_status":"critical"` in the output, and the document cannot be shared | The HTML bundle scan found a critical issue. An administrator must review the version in the review queue. Each sync is scanned again. |
| No chat card after the sync | Cards are sent only when the binding has a webhook and the document has an active organisation or public share link. |
| Files in ICS that you did not mean to publish | Everything under `publish.source` is uploaded, including dotfiles. Add [excludes](binding-file.md#exclude-files). |
| A file deleted in the repository is still in the document | It is in an older version only, not in the new one. Open the current version. |

## Problems with the image

| Symptom | Fix |
|---|---|
| The runner refuses the job because of the pull policy | The runner allows only some policies (for example `if-not-present`). Remove `pull_policy: always` and pin a release tag ([Pin the image](ci-recipes.md#pin-the-image)). |
| `permission denied`, `401` or `403` when pulling the image | The image `ghcr.io/inprojectspl/ics-sync` is meant to be pulled without credentials. If `docker pull ghcr.io/inprojectspl/ics-sync:1` fails on a machine without registry logins too, the package is not public yet: [open an issue](https://github.com/inprojectspl/ics-sync/issues), or build the image from this repository and push it to your own registry. Otherwise check the runner for registry credentials that force authentication (a custom `~/.docker/config.json`). |
| The job fails without running the `script:` lines | Check that `entrypoint: [""]` is under `image:`. The image's entrypoint is the CLI itself, so without the reset GitLab Runner cannot start the job's shell. |
| The snippet in ICS shows `<registry>/ics-sync:<tag>` | The ICS administrator has not set `ICS_SYNC_IMAGE`. Use `ghcr.io/inprojectspl/ics-sync:1`. |

## Related

- [Set up a repository](setup.md)
- [The binding file](binding-file.md)
- [CI job recipes](ci-recipes.md)
- [Keys and security](keys-and-security.md)
- [How it works](how-it-works.md)
