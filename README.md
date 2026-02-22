# PaperlessLink

A lightweight, cross-platform daemon written in Go that watches a directory for new files and automatically uploads them to [Paperless-ngx](https://docs.paperless-ngx.com/) via its REST API.

## Features

- 🔍 **Directory watching** using native OS events (`fsnotify`) – works on Linux, Windows, and macOS
- 🗂 **Extension filtering** – only process files with specific extensions
- 🔑 **Token authentication** – `Authorization: Token …` header
- 🆔 **UUID renaming** – optionally rename files to a UUID before upload (original name used as document title)
- 🗑 **Post-upload action** – delete the file or move it to a backup directory
- 📝 **Structured JSON logging** – to stdout and/or a log file
- 🛑 **Graceful shutdown** – handles `SIGINT` / `SIGTERM`

## Installation

### Build from source (requires Go 1.21+)

```bash
git clone https://github.com/youruser/paperlesslink.git
cd paperlesslink
make build          # native platform
make all            # Linux + Windows + macOS cross-compile
```

Binaries are placed in `bin/`.

## Usage

```
paperlesslink [flags]

Flags:
  -dir          string    Directory to watch (required)
  -url          string    Paperless-ngx base URL (required)
  -token        string    API token (required)
  -ext          string    Comma-separated extensions, e.g. pdf,png (default: all)
  -rename-uuid           Rename file to UUID before upload
  -after-upload string   Action after upload: delete | backup (default: delete)
  -backup-dir   string   Backup directory (required when -after-upload=backup)
  -log-file     string   Log file path (default: stdout only)
  -poll-interval duration Fallback poll interval (default: 5s)
  -version               Print version and exit
```

### Examples

**Minimal – watch /scans, upload PDFs, delete after upload:**
```bash
paperlesslink \
  -dir /scans \
  -url https://paperless.example.com \
  -token abc123 \
  -ext pdf
```

**Keep originals in a backup folder, rename with UUID:**
```bash
paperlesslink \
  -dir /scans \
  -url https://paperless.example.com \
  -token abc123 \
  -ext pdf,png,jpg \
  -rename-uuid \
  -after-upload backup \
  -backup-dir /scans/backup \
  -log-file /var/log/paperlesslink.log
```

## Running as a service

### systemd (Linux)

Create `/etc/systemd/system/paperlesslink.service`:

```ini
[Unit]
Description=PaperlessLink – automatic document uploader
After=network.target

[Service]
Type=simple
User=paperless
ExecStart=/usr/local/bin/paperlesslink \
    -dir /srv/scans \
    -url https://paperless.example.com \
    -token YOUR_TOKEN \
    -ext pdf,png \
    -after-upload backup \
    -backup-dir /srv/scans/backup \
    -log-file /var/log/paperlesslink.log
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now paperlesslink
```

### Windows Task Scheduler / Service

Use [NSSM](https://nssm.cc/) to wrap the `.exe` as a Windows service:

```cmd
nssm install PaperlessLink "C:\PaperlessLink\paperlesslink-windows-amd64.exe"
nssm set PaperlessLink AppParameters "-dir C:\Scans -url https://paperless.example.com -token YOUR_TOKEN -ext pdf -log-file C:\Logs\paperlesslink.log"
nssm start PaperlessLink
```

## API

PaperlessLink posts to:

```
POST {url}/api/documents/post_document/
Authorization: Token {token}
Content-Type: multipart/form-data

document=<file binary>
title=<filename stem>
```

This matches the official Paperless-ngx API documented at  
<https://docs.paperless-ngx.com/api/#post-/api/documents/post_document/>.

## License

MIT
