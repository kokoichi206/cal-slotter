# cal-slotter

[English](README.md) | [日本語](README.ja.md)

A CLI tool for finding shared availability across multiple Google Calendars, creating temporary hold events, and cleaning up unselected holds after a meeting time is confirmed.

## Features

- Find shared availability across multiple Google Calendars.
- Create temporary hold events in Google Calendar.
- Delete unselected hold events after a meeting time is confirmed.
- Limit target members per command with `--members`.
- Attach Google Meet to hold events and enable automatic recording, transcription, and Gemini notes.

## Install

For macOS or Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/kokoichi206/cal-slotter/main/scripts/install.sh | sh
slotter version
```

The installer downloads the latest binary from [GitHub Releases](https://github.com/kokoichi206/cal-slotter/releases/latest) and installs it to `~/.local/bin`. Set `SLOTTER_INSTALL_DIR` to use another directory.

```bash
curl -fsSL https://raw.githubusercontent.com/kokoichi206/cal-slotter/main/scripts/install.sh | SLOTTER_INSTALL_DIR=/usr/local/bin sh
```

To update an existing install:

```bash
slotter update
```

To check the latest release without installing it:

```bash
slotter update --dry-run
```

You can also download the archive for your OS and CPU from the [latest GitHub Release](https://github.com/kokoichi206/cal-slotter/releases/latest), then place the `slotter` binary somewhere on your `PATH`.

Manual macOS or Linux install:

```bash
tar -xzf slotter_*.tar.gz
chmod +x slotter
mkdir -p ~/.local/bin
mv slotter ~/.local/bin/
slotter version
```

For Windows, download the `.zip` archive from the release page and add `slotter.exe` to your `PATH`.

If you prefer installing from source with Go:

```bash
go install github.com/kokoichi206/cal-slotter/cmd/slotter@latest
slotter version
```

## Setup

### 1. Create an OAuth client in Google Cloud

Use the shared Google account that can access the target calendars.

1. Enable the Google Calendar API.
2. Create an OAuth client ID.
3. Choose "Desktop app" as the application type.
4. Download the OAuth client JSON.

If you use `hold --meet-artifacts` to automatically generate Google Meet recordings, transcripts, and Gemini notes, also enable the Google Meet API in the same project.

### 2. Place the credentials file

Save the downloaded JSON as `~/.config/cal-slotter/credentials.json`.

```bash
mkdir -p ~/.config/cal-slotter
mv ~/Downloads/client_secret_*.json ~/.config/cal-slotter/credentials.json
```

### 3. Create the config file

Create `~/.config/cal-slotter/config.json`.

```json
{
  "timezone": "Asia/Tokyo",
  "calendar_id": "primary",
  "credentials": "/Users/you/.config/cal-slotter/credentials.json",
  "token": "/Users/you/.config/cal-slotter/token.json",
  "members": [
    "member1@example.com",
    "member2@example.com"
  ]
}
```

`members` are the calendar IDs used for availability checks and hold event attendees. In typical usage, they are the participants' email addresses.

### 4. Authenticate once

```bash
slotter auth
```

Open the printed URL in a browser and authorize with the shared Google account. On success, `~/.config/cal-slotter/token.json` is created.

## Release

Releases are built by GoReleaser on GitHub Actions when a `v*` tag is pushed.

Release tags must:

1. Use the `vMAJOR.MINOR.PATCH` format, such as `v0.1.0`.
2. Point to the current `origin/main` commit.
3. Be newer than existing release tags.
4. Not already have a GitHub Release with the same tag.

Before tagging, verify the release artifacts locally:

```bash
go install github.com/goreleaser/goreleaser/v2@latest
goreleaser release --snapshot --clean
```

Then create and push a tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The release workflow uploads archives for macOS, Linux, and Windows on amd64 and arm64, plus a checksums file.

## Usage

```bash
slotter find --duration 60 \
  --range "2026-07-07 10:00-18:00" \
  --range "2026-07-08 10:00-18:00" \
  --count 5

slotter hold \
  --title "Customer kickoff" \
  --range "2026-07-07 10:00-18:00" \
  --range "2026-07-08 10:00-18:00" \
  --count 5 \
  --meet-artifacts

slotter confirm \
  --title "Customer kickoff" \
  --keep "2026-07-08 10:30"
```

When no slot is available, stdout stays empty and stderr prints `no available slots found`. Use `--json` to inspect the machine-readable result.

`find` uses `--strategy balanced` by default. Selected slots do not overlap, and the tool tries to spread them across dates and morning/afternoon periods. Use `--strategy early` to select the earliest non-overlapping slots.

`hold` can accept explicit `--slot` values. When `--slot` is omitted and `--range` is provided, it runs the same availability search as `find` and creates holds for the selected slots.

`confirm` keeps the hold event whose start time matches `--keep` for the same `--title`, renames it back to `--title`, and deletes the other matching hold events.

Calendar invitation and confirmation emails are not sent by default. Add `--send-updates` only when you want Google Calendar to send update emails. `confirm` does not send cancellation emails for deleted unselected holds.

Add `--meet-artifacts` to `hold` to attach Google Meet and enable automatic recording, transcription, and Gemini notes. If your existing `token.json` was created before this scope was added, run `slotter auth` again and grant the new permission.

## Config

The default config file is `~/.config/cal-slotter/config.json`. Use `--config` to load a different file.

```json
{
  "timezone": "Asia/Tokyo",
  "calendar_id": "primary",
  "credentials": "/Users/you/.config/cal-slotter/credentials.json",
  "token": "/Users/you/.config/cal-slotter/token.json",
  "members": [
    "member1@example.com",
    "member2@example.com"
  ]
}
```

`calendar_id` is the calendar on the shared account where hold events are created and deleted. `primary` is usually the right value.

Hold creation and confirmation do not send email updates by default. Add `--send-updates` to `hold` or `confirm` when email updates should be sent. `confirm` does not send cancellation emails for deleted unselected holds.

## License

[MIT](LICENSE)
