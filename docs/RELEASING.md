# Releasing breathe-tui

Releases are prepared manually and published through a reviewed draft. The release workflow reacts to an existing semantic-version tag; it never creates or publishes a tag.

## Release checklist

1. Start from an updated `main` branch.
2. Confirm the latest CI run is green.
3. Run `gofmt`, `go test ./...`, `go vet ./...`, `go build ./...`, and `git diff --check` locally.
4. Smoke-test an interactive practice on Windows, including pause, resize, `q`, and Ctrl+C restoration.
5. Smoke-test an interactive practice on macOS, including pause, resize, `q`, and Ctrl+C restoration.
6. Replace `Pending` in the changelog release heading with the release date if it has not already been set.
7. Review and commit the release-preparation changes.
8. Create an annotated tag:

   ```sh
   git tag -a v0.1.0 -m "breathe-tui v0.1.0"
   ```

9. Push that tag:

   ```sh
   git push origin v0.1.0
   ```

10. Wait for the Release workflow to complete.
11. Inspect the resulting draft GitHub Release; do not publish it yet.
12. Confirm that the draft contains all five platform archives and `SHA256SUMS`.
13. Download at least the Windows archive and the archive for the macOS test machine.
14. Verify both downloaded archives against `SHA256SUMS`.
15. Run `breathe version` from each extracted archive and confirm it reports the tagged version.
16. Smoke-test an actual practice from each extracted archive.
17. Review and edit the generated release notes as needed.
18. Publish the draft release manually.
19. Verify the public release page and its downloadable assets.

## Recovering from a failed release run

First inspect the failed job. If the failure was transient, use GitHub Actions' **Re-run jobs** action. The workflow deliberately fails if a release already exists for the tag, so inspect any existing draft before retrying. A partial draft may be removed through the GitHub interface only after confirming it was never published, then the same workflow run can be retried.

Do not delete or force-move a published tag. If the tagged source or workflow is wrong, or a published release needs correction, make the fix on `main` and issue a new patch release (for example, `v0.1.1`).
