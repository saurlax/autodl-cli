# Authentication and credential storage

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

[Back to the README](../README.md).

