# cal-slotter

[English](README.md) | 日本語

Google Calendar の freebusy から複数人の空き時間を探し、仮押さえイベントの作成と確定後の削除を行う CLI。

## インストール

macOS / Linux の場合:

```bash
curl -fsSL https://raw.githubusercontent.com/kokoichi206/cal-slotter/main/install.sh | sh
slotter version
```

installer は [GitHub Releases](https://github.com/kokoichi206/cal-slotter/releases/latest) の最新 binary をダウンロードし、`~/.local/bin` に配置する。別の場所に入れる場合は `SLOTTER_INSTALL_DIR` を指定する。

```bash
curl -fsSL https://raw.githubusercontent.com/kokoichi206/cal-slotter/main/install.sh | SLOTTER_INSTALL_DIR=/usr/local/bin sh
```

既存 install を更新する場合:

```bash
slotter update
```

install せずに最新 release だけ確認する場合:

```bash
slotter update --dry-run
```

[最新の GitHub Release](https://github.com/kokoichi206/cal-slotter/releases/latest) から OS と CPU に合う archive をダウンロードし、手動で `slotter` binary を `PATH` の通った場所に置くこともできる。

macOS / Linux の手動 install 例:

```bash
tar -xzf slotter_*.tar.gz
chmod +x slotter
mkdir -p ~/.local/bin
mv slotter ~/.local/bin/
slotter version
```

Windows の場合は、release page から `.zip` archive をダウンロードし、`slotter.exe` を `PATH` の通った場所に置く。

Go で source から install する場合:

```bash
go install github.com/kokoichi206/cal-slotter/cmd/slotter@latest
slotter version
```

## セットアップ

### 1. Google Cloud で OAuth クライアントを作る

共有アカウントで Google Cloud Console に入り、以下を設定する。

1. Google Calendar API を有効化する
2. OAuth クライアント ID を作成する
3. アプリケーションの種類は「デスクトップアプリ」を選ぶ
4. OAuth クライアントの JSON をダウンロードする

### 2. credentials を配置する

ダウンロードした JSON を `~/.config/cal-slotter/credentials.json` に置く。

```bash
mkdir -p ~/.config/cal-slotter
mv ~/Downloads/client_secret_*.json ~/.config/cal-slotter/credentials.json
```

### 3. 設定ファイルを作る

`~/.config/cal-slotter/config.json` を作る。

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

`members` には空き確認と仮押さえ招待の対象にするカレンダー ID を入れる。通常は参加者のメールアドレスを指定する。

### 4. 初回認証する

```bash
slotter auth
```

表示された URL をブラウザで開き、共有アカウントで許可する。成功すると `~/.config/cal-slotter/token.json` が作られる。

## リリース

`v*` tag を push すると、GitHub Actions 上の GoReleaser が GitHub Release を作成する。

release tag は以下を満たす必要がある。

1. `v0.1.0` のような `vMAJOR.MINOR.PATCH` 形式である
2. 現在の `origin/main` commit を指している
3. 既存の release tag より新しい
4. 同じ tag の GitHub Release がまだ存在しない

tag を打つ前に、ローカルで release artifact を確認する。

```bash
go install github.com/goreleaser/goreleaser/v2@latest
goreleaser release --snapshot --clean
```

tag を作って push する。

```bash
git tag v0.1.0
git push origin v0.1.0
```

release workflow は macOS、Linux、Windows の amd64 / arm64 向け archive と checksums file をアップロードする。

## 使い方

```bash
slotter find --duration 60 \
  --range "2026-07-07 10:00-18:00" \
  --range "2026-07-08 10:00-18:00" \
  --count 5

slotter hold \
  --title "AI 導入プロ-○○様7月初回" \
  --range "2026-07-07 10:00-18:00" \
  --range "2026-07-08 10:00-18:00" \
  --count 5

slotter confirm \
  --title "AI 導入プロ-○○様7月初回" \
  --keep "2026-07-08 10:30"
```

候補がない場合、stdout は空のまま stderr に `no available slots found` を出す。JSON で確認したい場合は `--json` を付ける。

`find` のデフォルト選定は `--strategy balanced`。選ばれる候補同士は重ならず、日付と午前/午後でできるだけ分散する。単純に早い順で見たい場合は `--strategy early` を指定する。

`hold` は `--slot` を直接渡せる。`--slot` がなく `--range` がある場合は、内部で `find` と同じ空き検索をして、その候補を仮押さえする。

仮押さえ作成と削除時のメール通知はデフォルトで送らない。送る場合だけ `--send-updates` を付ける。

## 設定

デフォルトの設定ファイルは `~/.config/cal-slotter/config.json`。別のパスを使う場合は `--config` で指定する。

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

`calendar_id` は仮押さえイベントを作成・削除する共有アカウント側のカレンダー ID。通常は `primary` を使う。

仮押さえ作成と削除時のメール通知はデフォルトで送らない。通知を送る場合だけ `hold` または `confirm` に `--send-updates` を付ける。

## ライセンス

[MIT](LICENSE)
