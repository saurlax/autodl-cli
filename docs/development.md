# Development and releases

```sh
gofmt -w cmd internal scripts
go vet ./...
go test -race ./...
go mod verify
go run ./scripts/package -version dev
```

Tests cover documented HTTP methods, paths, JSON payloads (including JSON bodies on GET), credential-source precedence, keyring save/error behavior without secret echo, required-input help, creation bounds, release confirmation, exact billing output, response errors, credential redaction, redirects, cancellation, and config replacement. They use local HTTP servers, fake credential stores, and temporary files. No real token, desktop keyring session, or GPU is needed.

GitHub Actions:

- **CI** runs on branch pushes, pull requests, and manual dispatch. It tests on Linux, macOS, and Windows, then cross-builds six targets and uploads archives with SHA-256 checksums.
- **Reusable test and build** contains the shared checks and packaging. Both CI and Release call it.
- **Release** runs on `v*` tag pushes or manual dispatch on a version tag. It calls the shared workflow, downloads that run's `release-assets`, verifies checksums, and uploads those exact archives to GitHub Releases. The publish job never builds again. Tags with a suffix such as `v0.2.0-rc.1` create prereleases. Re-running publishing replaces existing assets.

Publish a version (replace `vMAJOR.MINOR.PATCH` with a new version tag):

```sh
git tag vMAJOR.MINOR.PATCH
git push origin vMAJOR.MINOR.PATCH
# Manual CI or a retry on an existing release tag:
gh workflow run ci.yml
gh workflow run release.yml --ref vMAJOR.MINOR.PATCH
```

Workflows use read-only permissions except the release publishing job, which needs `contents: write`. The build implementation is `scripts/package/main.go`; it produces tar.gz archives for Linux/macOS, zip archives for Windows, embeds the version with linker flags, and requires no GoReleaser installation.

## Read-only acceptance checks

In a local PowerShell terminal:

```powershell
Set-Location path/to/autodl-cli
go run ./cmd/autodl config set-token
# Paste your developer token at the hidden prompt and press Enter.
go run ./cmd/autodl account balance
go run ./cmd/autodl instance list
go run ./cmd/autodl image list
go run ./cmd/autodl instance list --json
# If the list includes a Pro instance:
go run ./cmd/autodl instance status YOUR_INSTANCE_ID
```

These acceptance calls only read existing account state. Compare balance and instance status with the AutoDL console. `go run` reports application failures as `exit status N`; the Go launcher itself may exit with 1 even when the CLI's exit code is 2. To check the CLI exit code directly, run the compiled binary.

[Back to the README](../README.md).

