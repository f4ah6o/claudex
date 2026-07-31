# Claudex

[English](README.md)

Claudex は、Claude Code が利用する Anthropic Messages API を通じて、
OpenAI Codex のモデルをローカルで利用するための小さなゲートウェイです。

OpenAI または Anthropic の公式製品・提携製品ではありません。
Claude、Claude Code、Codex、OpenAI などの名称および商標は各権利者に帰属します。

対応範囲を意図的に限定しています。

- クライアント: Claude Code、Claude Desktop の Third-Party Inference Gateway
- 受け付けるプロトコル: Anthropic Messages API
- 上流プロバイダー: OpenAI Codex OAuth または Codex 互換 API キー
- 利用可能なモデル: `gpt-5.6` および `gpt-5.6-*`
- ネットワーク公開: ループバックのみ
- 利用形態: ローカルの単一ユーザー運用
- 管理 UI、プラグイン、その他のプロバイダー: 無効

## モデル割り当て

設定例では、Claude 互換 ID を3つの Codex モデルへ割り当てます。

| Claudeプロファイル | Codexモデル | Desktopでの表示名 |
| --- | --- | --- |
| Fable 5 (`claude-fable-5`) | `gpt-5.6-sol` | Codex GPT-5.6 Sol |
| Opus 5 (`claude-opus-5`) | `gpt-5.6-terra` | Codex GPT-5.6 Terra |
| Sonnet 5 (`claude-sonnet-5`) | `gpt-5.6-luna` | Codex GPT-5.6 Luna |

Haiku はクライアント側の context window が小さいため、Claudex では公開しません。Claude Code はバージョン付きIDや組み込みのClaudeモデルIDを送信することがあります。`claudex.example.yaml` に、それらの対応済みalias（旧Opus/Sonnet IDを含む）を定義しています。`gpt-5.6` および任意の `gpt-5.6-*` への直接リクエストも利用できます。このファミリー以外のモデルは、プロバイダーへ転送する前に拒否されます。

クライアントがthinkingを無効化した場合や独自のeffortを指定した場合を除き、Claudexは既定のeffortとして`xhigh`を適用します。Claude Code の `/effort max` は GPT 向けに `xhigh` へ正規化します。

## 構成

| パス | 役割 |
| --- | --- |
| `cmd/claudex` | `login`、`serve`、`version` を提供する専用 CLI |
| `cmd/claudexdesktop` | Claude Desktop用macOS/Linuxランチャー |
| `internal/claudex` | 設定検証、ルート制限、GPT-5.6 モデルポリシー |
| `claudex.example.yaml` | 最小構成の設定例 |
| その他のupstreamパッケージ | Codex OAuth、Anthropic↔Responses変換、ストリーミング、ツール、認証ローテーション |

## クイックスタート

リポジトリをcloneし、ゲートウェイをビルドして設定ファイルを作成します。

```bash
git clone https://github.com/f4ah6o/claudex.git
cd claudex
go build -o claudex ./cmd/claudex
cp claudex.example.yaml claudex.yaml
chmod 600 claudex.yaml
```

`claudex.yaml` の `replace-with-a-local-random-key` を、ローカルで使用するランダムなキーに置き換えてください。このキーはClaude CodeからClaudexへの認証用であり、上流プロバイダーの認証情報ではありません。プレースホルダーのままでは起動できません。

Claudexの設定スキーマは意図的に厳格です。listener、ローカルAPIキー、Codex認証ディレクトリ、retry／logging設定、Codex model aliasだけを受け付けます。未知フィールドと重複YAMLキーはmigration案内付きで起動前に拒否されます。

Codexにログインしてプロキシを起動します。

```bash
./claudex login
./claudex serve
```

デバイスコードでログインする場合は `./claudex login --device` を実行します。認証情報はデフォルトで `~/.claudex` に保存され、通常のCLIProxyAPIの認証情報とは分離されます。コマンドを省略した `./claudex` は `./claudex serve` と同じです。

別の設定ファイルを使う場合は `--config <path>` または `CLAUDEX_CONFIG` を指定します。

## Claude Codeからの利用

ローカルゲートウェイを指定し、対応モデルと推論強度を選択します。

```bash
export ANTHROPIC_BASE_URL="http://127.0.0.1:8317"
export ANTHROPIC_AUTH_TOKEN="claudex.yaml に設定したローカルキー"

claude --model claude-sonnet-5 --effort xhigh
```

Claude互換モデルIDは、前述のFable 5／Sol、Opus 5／Terra、Sonnet 5／Lunaの割り当てに従ってルーティングされます。対話中は `/model` で3つのClaude IDを切り替え、`/effort` で推論強度を変更できます。`gpt-5.6-*` を直接指定した場合は、許可されたモデルファミリー内でClaudeプロファイルのaliasを経由せずに利用します。

通常のAnthropic Claudeを使う場合は、ローカルゲートウェイ用の環境変数を解除します。

```bash
unset ANTHROPIC_BASE_URL ANTHROPIC_AUTH_TOKEN
claude --model opus
```

同梱のネイティブランチャーはこの使い分けを自動化します。`claude` は通常のAnthropicコマンドのまま維持し、`claudex` はローカルゲートウェイを起動または再利用して、既定ではSonnet 5／LunaプロファイルでClaude Codeを起動します。Claude Codeのtier切り替え用にFable 5／Sol、Opus 5／Terraも設定します。このリポジトリからビルドする `./claudex` はゲートウェイサーバー本体です。

## macOSのClaude Desktop

Finderから起動できる `ClaudexDesktop.app` を、起動せずにビルドします。

```sh
./script/build_and_run.sh --build-only
```

アプリをビルドし、起動できることまで確認します。

```sh
./script/build_and_run.sh --verify
```

Finderから起動する場合は `dist/ClaudexDesktop.app` を `~/Applications` にコピーします。初回起動時に `~/.config/claudex/claudex.yaml` を作成し、同梱サーバーを使ったCodexログインコマンドを表示します。コマンドを一度実行してから、もう一度 `ClaudexDesktop` を起動します。

`ClaudexDesktop` はループバックゲートウェイを起動し、Claude DesktopのThird-Party Inference Gateway設定を有効にしてからClaude Desktopを開きます。モデル一覧にはFable 5／Sol、Opus 5／Terra、Sonnet 5／Lunaの3件を表示します。セッション終了時に以前のClaude Desktop設定へ戻します。ランチャーが中断された場合は、もう一度 `ClaudexDesktop` を開くと保留中の設定バックアップを復元してから新しいセッションを開始します。

標準の `Claude Desktop` アプリ本体は変更しません。Desktopのプロバイダー設定は `ClaudexDesktop` がセッションを管理している間だけ変更します。ゲートウェイはループバック限定で、Claude Desktop終了後も常駐できます。

## セキュリティと利用境界

Claudexのサポート対象は、1人の利用者がローカルで使うゲートウェイです。

- 自分自身のOpenAIアカウント、OAuthセッション、APIキーだけを使用してください。
- OAuthトークン、APIキー、アカウントへのアクセス権、生成された認証ファイルを共有・再配布しないでください。
- ホステッドサービス、共有ゲートウェイ、複数ユーザー向け認証情報ブローカーとして運用しないでください。
- 利用上限、プラン制限、アクセス制御、プロバイダーのポリシー適用を回避する目的で使用しないでください。
- 適用されるOpenAIおよびAnthropicの利用規約・ポリシーに従ってください。
- ループバック限定を解除しないでください。これは配備時の初期値ではなく、セキュリティ境界です。

起動時に、Codex以外のプロバイダー、プラグイン、リモート管理、ループバック以外へのbind、または `gpt-5.6` / `gpt-5.6-*` 以外を対象にするaliasを有効にした設定は拒否します。

リクエスト時にAnthropicクライアントへ公開するのは `/v1/models`、`/v1/messages`、`/v1/messages/count_tokens` です。モデル一覧には前述のCodexモデル3件を返します。その他の汎用プロキシ用ルートはAnthropic互換の404を返します。

## インストール方法とmodule path

Claudexはupstreamとの同期を維持するため、Go module pathとして `github.com/router-for-me/CLIProxyAPI/v7` を保持しています。そのため、次のインストール方法はサポートしません。

```sh
go install github.com/f4ah6o/claudex/cmd/claudex@latest
```

ソースからの公式な導入方法は、このリポジトリをcloneして `just setup` を実行するか、clone内の `./cmd/claudex` をビルドする方法です。バイナリについては、このリポジトリのtag付きReleaseに添付されたものだけを公式配布物として扱います。

## Docker

ループバック限定のリスナーを使用するため、Linuxではhost networkを使います。

```bash
docker build -t claudex .
docker run --rm --network host \
  -v "$PWD/claudex.yaml:/app/claudex.yaml:ro" \
  -v "$HOME/.claudex:/home/claudex/.claudex" \
  claudex
```

## 開発

```bash
go test ./internal/claudex ./cmd/claudex
go build -o claudex ./cmd/claudex
govulncheck ./...
gitleaks detect --log-opts="--all"
```

macOSでは `./script/build_and_run.sh --build-only` または `--verify` でDesktop bundleを確認します。

upstreamの変更を専用プロダクト層と分離して取り込んでください。通常の同期では `cmd/claudex`、`cmd/claudexdesktop`、`internal/claudex`、`claudex.example.yaml`、Claudex用Docker targetを維持します。

機械可読のownership manifestと [UPSTREAM_SYNC.md](UPSTREAM_SYNC.md) を使って、レビュー可能な同期を実行してください。tag releaseではgatewayとDesktopを分離したarchive、checksum、SPDX SBOM、provenance attestationを生成します。

## Acknowledgements / 謝辞

Claudexは [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) を基にしています。upstreamのメンテナーとコントリビューターに感謝します。プロトコル変換とCodex認証がupstreamの改善を引き続き利用できるよう、Claudexはupstreamのコアを保持しつつ、製品として公開する範囲を意図的に小さくしています。

## ライセンス

MITライセンスです。詳細は [LICENSE](LICENSE) を参照してください。第三者依存関係には、それぞれのライセンスが適用されます。
