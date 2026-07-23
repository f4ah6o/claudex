# Claudex

[English](README.md)

Claudex は、Claude Code が利用する Anthropic Messages API を通じて、OpenAI Codex のモデルをローカルで利用するための小さなゲートウェイです。

対応する製品範囲を意図的に限定しています。

- クライアント: Claude Code
- 受け付けるプロトコル: Anthropic Messages API
- 上流プロバイダー: OpenAI Codex OAuth または Codex 互換 API キー
- 利用可能なモデル: `gpt-5.6` および `gpt-5.6-*`
- ネットワーク公開: ループバックのみ
- 管理 UI、プラグイン、その他のプロバイダー: 無効

## 構成

| パス | 役割 |
| --- | --- |
| `cmd/claudex` | `login`、`serve`、`version` を提供する専用 CLI |
| `internal/claudex` | 設定検証、ルート制限、GPT-5.6 モデルポリシー |
| `claudex.example.yaml` | 最小構成の設定例 |
| その他の upstream パッケージ | Codex OAuth、Anthropic↔Responses 変換、ストリーミング、ツール、認証ローテーション |

`cmd/server` は upstream 実装を保持するためのもので、Claudex のサポート対象実行ファイルではありません。

## クイックスタート

ビルドして設定ファイルを作成します。

```bash
go build -o claudex ./cmd/claudex
cp claudex.example.yaml claudex.yaml
```

`claudex.yaml` の `replace-with-a-local-random-key` を、ローカルで使用するランダムなキーに置き換えてください。このキーは Claude Code から Claudex への認証用であり、上流プロバイダーの認証情報ではありません。プレースホルダーのままでは起動できません。

Codex にログインしてプロキシを起動します。

```bash
./claudex login
./claudex serve
```

デバイスコードでログインする場合は `./claudex login --device` を実行します。認証情報はデフォルトで `~/.claudex` に保存され、通常の CLIProxyAPI の認証情報とは分離されます。コマンドを省略した `./claudex` は `./claudex serve` と同じです。

別の設定ファイルを使う場合は `--config <path>` または `CLAUDEX_CONFIG` を指定します。

## Claude Code からの利用

ローカルゲートウェイを指定し、対応モデルと推論強度を選択します。

```bash
export ANTHROPIC_BASE_URL="http://127.0.0.1:8317"
export ANTHROPIC_AUTH_TOKEN="claudex.yaml に設定したローカルキー"

claude --model gpt-5.6-luna --effort xhigh
```

`xhigh` は Claude Code の effort 設定として渡されるため、モデル名にサフィックスを付ける必要はありません。設定例では Claude Code の組み込み Opus、Sonnet、Haiku の ID を `gpt-5.6-luna` に割り当てています。`gpt-5.6` および `gpt-5.6-*` への直接リクエストも利用できます。このファミリー以外のモデルはプロバイダーへ転送する前に拒否されます。

通常の Anthropic Claude を使う場合は、ローカルゲートウェイ用の環境変数を解除します。

```bash
unset ANTHROPIC_BASE_URL ANTHROPIC_AUTH_TOKEN
claude --model opus
```

この使い分けを自動化する任意のシェルランチャーも用意できます。`claude` は通常の Anthropic Claude を起動し、別名の `claudex` はローカルゲートウェイを起動して `gpt-5.6-luna` と `xhigh` を指定した Claude Code を起動します。このリポジトリからビルドされる `./claudex` はゲートウェイサーバー本体です。

## macOSのClaude Code Desktop

Finderから起動できる `ClaudexDesktop.app` をビルドします。

```sh
./script/build_and_run.sh --verify
```

Finderから起動したい場合は `dist/ClaudexDesktop.app` を `~/Applications` にコピーしてください。初回起動時に `~/.config/claudex/claudex.yaml` を作成し、同梱サーバーを使ったCodexログインコマンドを表示します。コマンドを一度実行してから、もう一度 `ClaudexDesktop` を起動します。

`ClaudexDesktop` はループバックゲートウェイを起動し、Claude Desktopの公式Third-Party Inference Gateway設定を有効にしてからClaude Desktopを開きます。表示するモデルは `Codex GPT-5.6 Luna (xhigh)` の1件に固定し、セッション終了時に通常のClaude Desktop設定へ戻します。ランチャーが中断された場合は、もう一度 `ClaudexDesktop` を開くと、保留中のバックアップを復元してから起動します。

標準の `Claude Desktop` アプリ本体は変更しません。Desktopのプロバイダー設定は `ClaudexDesktop` がセッションを管理している間だけ変更し、ゲートウェイはループバック限定で、Claude Desktop終了後も常駐できます。

## クロスプラットフォームセットアップ

リポジトリには `justfile` と、Windows・macOS・Linux で動作するネイティブランチャーを含めています。`just` を Cargo で一度だけ導入し、リポジトリのルートでセットアップタスクを実行します。

```sh
cargo install just --locked
just setup
```

`just setup` は設定ファイルを作成し、ローカルクライアントキーを生成し、ネイティブサーバーをビルドしてランチャーを配置します。検出できるシェルではランチャーのディレクトリをユーザー `PATH` に追加します。Windows ARM64 では `$HOME\\.config\\claudex\\claudex.yaml` と `$HOME\\bin`、macOS/Linux では `${XDG_CONFIG_HOME:-$HOME/.config}/claudex/claudex.yaml` と `${XDG_BIN_HOME:-$HOME/.local/bin}` を使用します。セットアップ後は新しいターミナルを開いてください。

ブラウザ OAuth で Codex にログインし、ローカルゲートウェイ経由で Claude Code を起動します。

```sh
just login
just run
```

`just serve` はゲートウェイだけを起動し、`just build` はネイティブランチャーを再ビルドし、`just verify` は対象テストとビルドを実行します。新しいターミナルでは `claudex` を直接起動することもできます。ランチャーはサーバーが起動していなければ起動し、設定ファイルからローカルクライアントキーを読み取り、Claudex 用の環境変数を子プロセスの Claude Code にだけ渡します。`claude` は通常の Anthropic コマンドのままです。

## 設定上の境界

起動時に、Codex 以外のプロバイダー、プラグイン、リモート管理、ループバック以外への bind、または `gpt-5.6` / `gpt-5.6-*` 以外を対象にする alias を有効にした設定は拒否します。

リクエスト時にAnthropicクライアントへ公開するのは `/v1/models`、`/v1/messages`、`/v1/messages/count_tokens` です。Desktopのモデル一覧にはCodex固定モデル1件だけを返します。その他の汎用プロキシ用ルートは Anthropic 互換の 404 を返します。

## Docker

ループバック限定のリスナーを使用するため、Linux では host network を使います。

```bash
docker build -t claudex .
docker run --rm --network host \
  -v "$PWD/claudex.yaml:/app/claudex.yaml:ro" \
  -v "$HOME/.claudex:/root/.claudex" \
  claudex
```

## 開発

```bash
go test ./internal/claudex ./cmd/claudex
go build -o claudex ./cmd/claudex
```

upstream の変更を専用プロダクト層と分離して取り込んでください。通常の同期では `cmd/claudex`、`internal/claudex`、`claudex.example.yaml`、Claudex 用の Docker ターゲットを維持します。

## Acknowledgements / 謝辞

Claudex は [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) を基にしています。upstream のメンテナーとコントリビューターに感謝します。プロトコル変換と Codex 認証が upstream の改善を引き続き利用できるよう、Claudex は upstream のコアを保持しつつ、製品として公開する範囲を意図的に小さくしています。

## ライセンス

MIT ライセンスです。詳細は [LICENSE](LICENSE) を参照してください。
