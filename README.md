# cal-slotter

[English](README.md) | [日本語](README.ja.md)

A CLI tool for finding shared availability across multiple Google Calendars, creating temporary hold events, and cleaning up unselected holds after a meeting time is confirmed.

## Setup

### 1. Create an OAuth client in Google Cloud

Use the shared Google account that can access the target calendars.

1. Enable the Google Calendar API.
2. Create an OAuth client ID.
3. Choose "Desktop app" as the application type.
4. Download the OAuth client JSON.

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
go run ./cmd/schedule auth
```

Open the printed URL in a browser and authorize with the shared Google account. On success, `~/.config/cal-slotter/token.json` is created.

## Usage

```bash
go run ./cmd/schedule find --duration 60 \
  --range "2026-07-07 10:00-18:00" \
  --range "2026-07-08 10:00-18:00" \
  --count 5

go run ./cmd/schedule hold \
  --title "Customer kickoff" \
  --range "2026-07-07 10:00-18:00" \
  --range "2026-07-08 10:00-18:00" \
  --count 5

go run ./cmd/schedule confirm \
  --title "Customer kickoff" \
  --keep "2026-07-08 10:30"
```

When no slot is available, stdout stays empty and stderr prints `no available slots found`. Use `--json` to inspect the machine-readable result.

`find` uses `--strategy balanced` by default. Selected slots do not overlap, and the tool tries to spread them across dates and morning/afternoon periods. Use `--strategy early` to select the earliest non-overlapping slots.

`hold` can accept explicit `--slot` values. When `--slot` is omitted and `--range` is provided, it runs the same availability search as `find` and creates holds for the selected slots.

Calendar invitation and deletion emails are not sent by default. Add `--send-updates` only when you want Google Calendar to send update emails.

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

Hold creation and deletion do not send email updates by default. Add `--send-updates` to `hold` or `confirm` when email updates should be sent.

## License

[MIT](LICENSE)
