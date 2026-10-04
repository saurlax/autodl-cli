# autodl-cli

A small, unofficial Go CLI for AutoDL's Common and Container Instance Pro APIs. Built with [Cobra](https://github.com/spf13/cobra), with nested commands, descriptive English help, shell completion, readable output, and JSON for automation.

## Install

Download an archive for your OS and CPU from [GitHub Releases](https://github.com/saurlax/autodl-cli/releases), verify it against `checksums.txt`, extract it, and put `autodl` (`autodl.exe` on Windows) on your `PATH`. Release archives include a statically built binary, this README, and the license. Supported targets: Linux, macOS, and Windows, each on amd64 and arm64.

Build from source with Go 1.26 or later:

```sh
go install github.com/saurlax/autodl-cli/cmd/autodl@latest
# Or from a checkout:
go build -o autodl ./cmd/autodl
```

## Authentication

Get a developer token from the AutoDL console → Account → Settings → Developer Token. Container Instance Pro APIs require personal or enterprise identity verification.

```sh
export AUTODL_TOKEN='your-developer-token'
# PowerShell:
# $env:AUTODL_TOKEN = 'your-developer-token'
autodl account balance
```

For local persistence, run:

```sh
autodl config set-token
```

In a terminal this prompts for a token with input hidden; paste it and press Enter. With redirected stdin it reads until EOF. It saves the token without printing or verifying it.

The default store is the **system keyring**: Windows Credential Manager, macOS Keychain, or Linux Secret Service (D-Bus, with an unlocked `login` collection). Credentials use service `autodl-cli` and account `developer-token`. No token is written to a config file. Keyring errors never silently fall back to plaintext; on headless servers use `AUTODL_TOKEN` or explicitly opt into a credential file with `--config PATH`.

Token precedence: `--token` → `AUTODL_TOKEN` → system keyring. An explicit `--config PATH` selects a plaintext file instead of the keyring. `autodl config set-token --config PATH` writes that file; new files have mode `0600` on Unix and inherit the user's directory ACLs on Windows.

Upgrading from v0.1.0: the old default plaintext file is no longer read automatically. Run `config set-token` again to save your token in the keyring, then remove the old file after verifying access. Alternatively, explicitly select the old file with `--config`: Linux `$XDG_CONFIG_HOME/autodl/config.json` or `~/.config/autodl/config.json`, macOS `~/Library/Application Support/autodl/config.json`, Windows `%AppData%\autodl\config.json`.

### Run from source and accept the CLI

In a local PowerShell terminal:

```powershell
Set-Location D:\Projects\autodl-cli
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

## Usage

```sh
autodl                       # Root command help
autodl instance              # All instance subcommands
autodl instance create       # Missing required options: full create help and exit 2
autodl instance create --help
autodl instance list --page 1 --page-size 20
autodl instance status pro-76419909953e
autodl instance get pro-76419909953e
```

`instance get` includes SSH passwords and Jupyter tokens in its output. Treat snapshots as sensitive. Lists are printed as tables; balance uses exact thousandths of CNY; scalar results such as status and created instance IDs are printed directly; snapshots are indented JSON. Successful operations with no data print `OK`.

```sh
# Inspect the exact creation request without credentials, network access, or billing.
autodl instance create \
  --gpu-spec pro6000-p \
  --image base-image-l2t43iu6uk \
  --cuda-min 118 \
  --gpus 1 \
  --region westDC3,beijingDC2 \
  --name training \
  --dry-run

# Remove --dry-run to create and boot a pay-as-you-go instance.
autodl instance start pro-76419909953e --start-command 'nvidia-smi'
autodl instance stop pro-76419909953e
autodl image save pro-76419909953e --name training-env
autodl image list

# Release permanently deletes instance data. Stop the instance first.
autodl instance release pro-76419909953e --yes

autodl storage mount --data-center westDC2 --type exclusive
autodl storage mount --data-center westDC2 --type ordinary
```

Creation requires `--gpu-spec`, `--image`, and `--cuda-min`. GPU count defaults to 1 (range 1–4); `--disk-gb` defaults to 0 (range 0–500). CUDA uses the API's integer encoding: 113 means 11.3 and 118 means 11.8. Omit regions to let AutoDL choose. GPU and image IDs are passed through so new platform offerings do not require a CLI update; find current IDs in the official appendix. Startup command failures do not automatically stop billing. The API currently supports pay-as-you-go creation and GPU-mode startup.

### Command reference

| Command | Purpose |
| --- | --- |
| `account balance` | Balance, vouchers, lifetime spend |
| `instance list` | Paginated Pro instance list |
| `instance create` | Create and boot a GPU instance |
| `instance get INSTANCE_ID` | Snapshot and connection credentials |
| `instance status INSTANCE_ID` | Current lifecycle status |
| `instance start INSTANCE_ID` | GPU-mode power on |
| `instance stop INSTANCE_ID` | Power off |
| `instance release INSTANCE_ID --yes` | Permanent release |
| `image save INSTANCE_ID --name NAME` | Save a private image |
| `image list` | Paginated private images and save status |
| `storage mount --data-center CODE --type TYPE` | Switch exclusive NFS / ordinary storage |
| `config set-token` | Save token in system keyring (hidden terminal input) |
| `completion bash\|zsh\|fish\|powershell` | Generate shell completion |

Run any group without a subcommand to show its help. Missing arguments, missing required flags, invalid values, and unknown flags print the relevant command's complete help. Help and validation require no credentials.

### Automation

```sh
autodl instance list --json
autodl instance status pro-76419909953e --json
autodl instance stop pro-76419909953e --timeout 2m
autodl completion bash > autodl-completion.bash
```

Global flags: `--token`, `--config`, `--base-url`, `--timeout` (default 30s), `--json`, `--dry-run`, `--help`, and `--version`. `--json` outputs the complete successful API envelope (`code`, `data`, `msg`, optional `request_id`) as one JSON object to stdout. Errors go to stderr. Exit codes: 0 success/help, 2 command usage errors, 1 config/network/API/output errors. Dry-run prints `{method,url,body}` and never includes the Authorization header; other body values, including startup commands, are printed as supplied. It works for API commands only.

There are no automatic retries: repeating a creation or destructive request after an ambiguous network failure could perform the operation twice. Check the console before retrying. The HTTP client respects proxy environment variables, has a per-request timeout, propagates interrupt cancellation, limits response size, and refuses redirects. A custom base URL must be an HTTPS origin; loopback HTTP is accepted for local testing.

## Development and releases

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

Publish a version:

```sh
git tag v0.1.0
git push origin v0.1.0
# Manual CI or a retry on an existing release tag:
gh workflow run ci.yml
gh workflow run release.yml --ref v0.1.0
```

Workflows use read-only permissions except the release publishing job, which needs `contents: write`. The build implementation is `scripts/package/main.go`; it produces tar.gz archives for Linux/macOS, zip archives for Windows, embeds the version with linker flags, and requires no GoReleaser installation.

## API sources and references

The API contract follows [Common API](https://www.autodl.com/docs/common_api/) and [Container Instance Pro API](https://www.autodl.com/docs/instance_pro_api/) (reviewed 2026-10-04). The snapshot/status endpoints deliberately send a JSON body with GET because that is the official documented contract. Common API's standalone “save image” heading has no callable endpoint; private-image saving uses the documented Pro endpoint. Application-instance APIs, elastic deployments, SSH execution, and TUI features are outside this CLI's scope.

Command design was informed by [Minato-Aqukin/AutoDL-cli](https://github.com/Minato-Aqukin/AutoDL-cli) and [Raymond1800/autodl-cli](https://github.com/Raymond1800/autodl-cli). This is an independent Go implementation using the specified official API, not a port of their source. Not affiliated with or endorsed by AutoDL.

MIT license.
