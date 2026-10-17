# ics-sync user guide

ics-sync publishes the content of a GitLab repository into a document in
[ICS (Internal Content Sharing)](https://github.com/inprojectspl/ics). A job in the repository's
own CI pipeline packs the files, signs the upload and sends it to ICS, which stores it as a new
version of the document. Every push to the default branch then keeps the document in ICS up to
date, with the full version history ICS keeps for any document.

## When to use it

Upload by hand (the **Update content** action in ICS) is fine for a document that changes now
and then. Use ics-sync when:

- the content already lives in Git: a handbook, runbooks, ADRs, a Markdown knowledge base;
- the content is built by a pipeline: a static site, a generated report, an HTML dashboard;
- you want ICS to show what is on the default branch without anyone remembering to upload it;
- you want each published version tied to a commit (ICS records the commit SHA of every sync).

It works with any GitLab instance, including self-managed ones behind a firewall: ICS never
connects to GitLab. The job only needs to reach ICS over HTTP(S).

When not to use it: when people edit the document in ICS itself. Every sync replaces the whole
content with what is in the repository, so edits made in ICS between syncs are kept in the
version history but disappear from the current version at the next push.

## Pages

- [How it works](how-it-works.md): the trust model, what is signed, replay protection and what
  never leaves the CI job.
- [Set up a repository](setup.md): the end-to-end walkthrough, from the binding dialog in ICS to
  the first new version in the document's history.
- [The binding file](binding-file.md): every field of `.inprojects.ics`, with recipes for the
  whole repository, one folder, a built static site and excludes.
- [CI job recipes](ci-recipes.md): job variants for the default branch, a build job with
  artifacts, pinned image tags, runner tags and more.
- [Keys and security](keys-and-security.md): where the private key is kept, how to rotate it,
  how to unlink a repository and what to do when a key leaks.
- [Troubleshooting](troubleshooting.md): every error the CLI and ICS can return, with the fix.

## Elsewhere

- [ics-sync README](https://github.com/inprojectspl/ics-sync/blob/main/README.md): quick start, flags and image tags.
- [ICS user guide](https://github.com/inprojectspl/ics/tree/main/docs/guide): documents,
  versions, sharing and administration in ICS.
- [ICS deployment guide](https://github.com/inprojectspl/ics/blob/main/docs/DEPLOY.md):
  for administrators, including the `ICS_SYNC_IMAGE` setting that fills in the image name in the
  binding dialog.
