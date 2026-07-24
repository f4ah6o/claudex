# Add ClaudexDesktop launcher for Claude Desktop

Status: closed
Created: 2026-07-23
Updated: 2026-07-24
Closed: 2026-07-24

## 概要

macOS向けの`ClaudexDesktop.app`を追加した。アプリは同梱のClaudexサーバーを起動し、Claude DesktopのThird-Party Inference Gateway設定をローカルゲートウェイへ切り替え、セッション終了時に以前の設定を復元する。

## 実装内容

- `cmd/claudexdesktop`にmacOSランチャーを実装した
- `script/build_and_run.sh`でFinder起動可能なapp bundleを生成する
- `--build-only`でアプリを起動せずbundleを検証できる
- 初回起動時に`~/.config/claudex/claudex.yaml`を生成する
- Codexログインが未完了の場合は実行コマンドを案内する
- Claude Desktopが実行中の場合は確認してから再起動する
- provider設定とconfig libraryを退避し、正常終了時または次回起動時に復元する
- 標準のClaude Desktop app bundleは変更しない
- ゲートウェイはループバック限定とする
- `/v1/models`にCodex GPT-5.6 Sol、Terra、Lunaの3モデルを公開する
- `/v1/messages`と`/v1/messages/count_tokens`以外の汎用ルートを公開しない
- APIキー、OAuthトークン、設定バックアップをユーザー専用権限で保存する

## モデル割り当て

| Claudeプロファイル | Codexモデル |
| --- | --- |
| Opus | `gpt-5.6-sol` |
| Sonnet | `gpt-5.6-terra` |
| Haiku | `gpt-5.6-luna` |

既定のeffortは`xhigh`。クライアントがthinkingを無効化した場合、または独自のeffortを指定した場合は上書きしない。

## 検証

```sh
./script/build_and_run.sh --build-only
./script/build_and_run.sh --verify
```

CIではmacOS runner上でbundleを生成し、ランチャー、同梱サーバー、設定例、`Info.plist`を検証する。
