# AutoDL CLI

[![CI](https://github.com/saurlax/autodl-cli/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/saurlax/autodl-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/saurlax/autodl-cli)](https://github.com/saurlax/autodl-cli/releases)
[![Go](https://img.shields.io/badge/go-1.26%2B-00ADD8)](go.mod)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

**Manage AutoDL GPU instances from your terminal.**

AutoDL CLI is an open-source, unofficial client for AutoDL's Common and Container
Instance Pro APIs. It brings account billing, instance lifecycle operations,
private images, and storage switching into a single `autodl` command, with readable
terminal output and JSON responses for scripts.

> **Project scope:** Supports the documented Common and Container Instance Pro
> endpoints listed below. Application instances, elastic deployments, SSH
> execution, and a TUI are outside the current scope. This project is independent
> of AutoDL and is not affiliated with or endorsed by the platform.

## What you can do

- **Inspect your account.** Read balances, vouchers, and lifetime spending.
- **Manage Pro instances.** List, inspect, create, start, stop, and release GPU instances.
- **Reuse environments.** Save private images, check their preparation status, and switch storage types.
- **Automate workflows.** Use JSON output, shell completion, explicit exit codes, and request previews without credentials or network access.

## Install

Download an archive from [GitHub Releases](https://github.com/saurlax/autodl-cli/releases),
verify it against `checksums.txt`, extract it, and put `autodl` (`autodl.exe` on
Windows) on your `PATH`. Archives contain a statically built binary, the README,
and the license.

| Operating system | Architectures |
| --- | --- |
| Linux | amd64, arm64 |
| macOS | amd64, arm64 |
| Windows | amd64, arm64 |

With **Go 1.26 or later**, you can also install from source:

```sh
go install github.com/saurlax/autodl-cli/cmd/autodl@latest
```

Ensure your Go binary directory is on `PATH`, then run `autodl --version`.

## Quick start

Get a developer token from the AutoDL console under **Account → Settings →
Developer Token**. Container Instance Pro APIs require personal or enterprise
identity verification.

```sh
autodl config set-token
# Paste the developer token at the hidden prompt and press Enter.
autodl account balance
autodl instance list
autodl image list
```

The token is stored in the system keyring: Windows Credential Manager, macOS
Keychain, or Linux Secret Service. Headless environments can use `AUTODL_TOKEN`:

```sh
export AUTODL_TOKEN='your-developer-token'
autodl instance list --json
```

In PowerShell, use `$env:AUTODL_TOKEN = 'your-developer-token'`. Token precedence is
`--token` → `AUTODL_TOKEN` → system keyring. An explicit `--config PATH` selects a
plaintext credential file instead of the keyring; keyring errors never silently
fall back to plaintext. See the [authentication guide](docs/authentication.md) for
storage requirements, file permissions, and migration from v0.1.0.

## Use

Run `autodl`, a command group such as `autodl instance`, or any command with
`--help` to see its options. Help and input validation require no credentials.
Replace `INSTANCE_ID` below with an ID from `instance list`.

```sh
autodl instance list --page 1 --page-size 20
autodl instance status INSTANCE_ID
autodl instance get INSTANCE_ID
```

`instance get` includes SSH passwords and Jupyter tokens. Treat its output,
including JSON snapshots, as sensitive.

### Create and manage an instance

Preview a creation request before sending it:

```sh
autodl instance create \
  --gpu-spec pro6000-p \
  --image base-image-l2t43iu6uk \
  --cuda-min 118 \
  --gpus 1 \
  --region westDC3,beijingDC2 \
  --name training \
  --dry-run
```

GPU, image, and region values are examples; select current IDs from the
[official API appendix](https://www.autodl.com/docs/instance_pro_api/).
Creation requires `--gpu-spec`, `--image`, and `--cuda-min`. CUDA uses integer
encoding: `113` means 11.3 and `118` means 11.8. GPU count defaults to 1 (range
1–4); `--disk-gb` defaults to 0 (range 0–500). Omit regions to let AutoDL choose.
IDs are passed through so new offerings do not require a CLI update.

Removing `--dry-run` **creates and boots a pay-as-you-go instance**. Other API
commands also send requests immediately unless `--dry-run` is supplied:

```sh
autodl instance start INSTANCE_ID --start-command 'nvidia-smi'
autodl instance stop INSTANCE_ID
autodl image save INSTANCE_ID --name training-env
autodl image list
autodl storage mount --data-center westDC2 --type exclusive
autodl storage mount --data-center westDC2 --type ordinary
```

Only pay-as-you-go creation and GPU-mode startup are supported. A startup command
failure does not automatically stop billing. To permanently delete an instance
and its data, stop it first, then run `autodl instance release INSTANCE_ID --yes`.

### Command reference

| Command | Purpose |
| --- | --- |
| `account balance` | Balance, vouchers, and lifetime spending |
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
| `config set-token` | Save a developer token |
| `completion bash\|zsh\|fish\|powershell` | Generate shell completion |

Missing arguments, required flags, invalid values, and unknown flags print the
relevant command's complete help.

## Automation

```sh
autodl instance list --json
autodl instance status INSTANCE_ID --json
autodl instance status INSTANCE_ID --timeout 2m
autodl completion bash > autodl-completion.bash
```

Global flags are `--token`, `--config`, `--base-url`, `--timeout` (default `30s`),
`--json`, `--dry-run`, `--help`, and `--version`.

`--json` writes the complete successful API envelope (`code`, `data`, `msg`, and
optional `request_id`) as one JSON object to stdout. Errors go to stderr. Default
output uses tables for lists, exact thousandths of CNY for balances, scalar values
for status and created IDs, and indented JSON for snapshots. Successful operations
without data print `OK`.

| Exit code | Meaning |
| --- | --- |
| `0` | Success or help |
| `1` | Configuration, network, API, or output error |
| `2` | Command usage error |

`--dry-run` works on API commands and prints `{method,url,body}` without loading
credentials or sending a request. It excludes the Authorization header; body
values such as startup commands are printed as supplied.

Requests are never retried automatically. After an ambiguous network failure,
check the console before repeating a creation or destructive operation. The HTTP
client respects proxy environment variables, applies a per-request timeout,
propagates interrupt cancellation, limits response size, and refuses redirects.
Custom base URLs must be HTTPS origins; loopback HTTP is accepted for local testing.

## Develop

```sh
git clone https://github.com/saurlax/autodl-cli.git
cd autodl-cli
go build -o autodl ./cmd/autodl
go vet ./...
go test -race ./...
go mod verify
```

Tests use local HTTP servers, fake credential stores, and temporary files; no
real token, desktop keyring session, or GPU is needed. CI runs on Linux, macOS,
and Windows and packages all six release targets. See the
[development and release guide](docs/development.md) for packaging, validation
coverage, and read-only acceptance checks.

## Contribute

Bug reports, documentation improvements, and focused API additions are welcome.
Search [GitHub Issues](https://github.com/saurlax/autodl-cli/issues) before opening
a report or proposal. Include your CLI version, operating system, command, and
sanitized error output; remove developer tokens and instance connection credentials.
Discuss new API coverage before implementing it, and keep changes aligned with
the official contract. Run the development checks above before submitting a pull request.

## API sources and acknowledgments

The API contract follows AutoDL's [Common API](https://www.autodl.com/docs/common_api/)
and [Container Instance Pro API](https://www.autodl.com/docs/instance_pro_api/),
reviewed on 2026-10-04. Snapshot and status endpoints deliberately send a JSON body
with GET, following that contract. Common API's standalone “save image” heading
has no callable endpoint; private-image saving uses the documented Pro endpoint.

Command design was informed by [Minato-Aqukin/AutoDL-cli](https://github.com/Minato-Aqukin/AutoDL-cli)
and [Raymond1800/autodl-cli](https://github.com/Raymond1800/autodl-cli). This is an
independent Go implementation, built with [Cobra](https://github.com/spf13/cobra).

## License

AutoDL CLI is licensed under the [MIT License](LICENSE).
