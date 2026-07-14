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

## Windows ランチャー

リポジトリには PowerShell ランチャーとコマンド shim を含めています。サーバーを別名でビルドし、`claudex-server.exe`、`claudex.ps1`、`claudex.cmd` を同じ `PATH` 上のディレクトリに配置します。

```powershell
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64" # Windows on ARM では arm64
go build -o claudex-server.exe ./cmd/claudex
```

設定ファイルを `CLAUDEX_CONFIG` に指定し、PowerShell またはコマンドプロンプトから `claudex` を実行します。

```powershell
$env:CLAUDEX_CONFIG = "$HOME\.config\claudex\claudex.yaml"
claudex
```

ランチャーはサーバーが起動していなければ起動し、設定ファイルからローカルクライアントキーを読み取り、Claudex 用の環境変数を子プロセスの Claude Code にだけ渡します。`claude` は通常の Anthropic コマンドのままです。必要に応じて `CLAUDEX_SERVER_PATH`、`CLAUDEX_BASE_URL`、`CLAUDEX_LOG_DIR` で既定値を変更できます。

## 設定上の境界

起動時に、Codex 以外のプロバイダー、プラグイン、リモート管理、ループバック以外への bind、または `gpt-5.6` / `gpt-5.6-*` 以外を対象にする alias を有効にした設定は拒否します。

リクエスト時に公開するのは `/v1/messages` と `/v1/messages/count_tokens` のみです。その他の汎用プロキシ用ルートは Anthropic 互換の 404 を返します。

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
