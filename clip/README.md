# Crush Clipboard Bridge

This helper lets Crush running on a remote VPS read an image from the clipboard of a Windows PC.

The bridge binds only to `127.0.0.1` on Windows. OpenSSH creates a reverse tunnel to a loopback-only port on the VPS, so no clipboard endpoint is exposed publicly.

## Install

Open PowerShell in this folder and run:

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

On a new PC, the installer creates a dedicated Ed25519 key. If the VPS does not already trust that key, SSH asks for the VPS password once and adds the public key to `~/.ssh/authorized_keys`. Future starts use the key without a password.

The installed process starts immediately and whenever the current Windows user signs in. It runs in the interactive user session because a Windows system service cannot reliably access that user's clipboard.

Defaults:

- VPS: `root@157.173.127.84`
- Windows loopback: `127.0.0.1:47831`
- VPS loopback: `127.0.0.1:48731`
- Maximum image size: 5 MB

Override values during installation when needed:

```powershell
.\install.ps1 -VpsHost 157.173.127.84 -VpsUser root -SshPort 22 -LocalPort 47831 -RemotePort 48731
```

Only one PC can own the same VPS reverse port at a time. Assign a different `-RemotePort` if multiple PCs must remain connected simultaneously, and configure Crush to use the desired bridge.

## Control

```powershell
.\start.ps1
.\stop.ps1
.\uninstall.ps1
```

Runtime files and logs are stored in `%LOCALAPPDATA%\CrushClipBridge`. The dedicated private key is stored in `%USERPROFILE%\.ssh\crush_clip_bridge_ed25519` and is not copied into this folder.

## Security

- Both HTTP listener endpoints are loopback-only.
- The image endpoint requires a random bearer token shared over SSH.
- The reverse tunnel uses a dedicated key and key-only authentication after setup.
- The bridge serves only the current clipboard image. It cannot execute commands or read arbitrary files.
- SSH uses `StrictHostKeyChecking=accept-new` on first connection. Verify the VPS host key out of band when deploying on an untrusted network.
