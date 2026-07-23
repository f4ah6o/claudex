# Prepare Claudex for public release

Status: open
Created: 2026-07-24
Updated: 2026-07-24
Branch: docs/20260724-public-release-readiness

## 概要

ClaudexをPublicリポジトリとして公開する前に、秘密情報の誤コミット防止、ドキュメントと実装の整合、独立したOSSとしての境界、配布方法を整理する。

`Claudex`という名称は、OpenAIでCodexを率いるThibault SottiauxがClaude CodeをCodexモデルへ接続するワークフローの名称として使用したものに由来する。

- https://x.com/thsottiaux/status/2076119366647894371?s=46

このため名称変更は本issueの対象にしない。ただし、この投稿はOpenAIまたはAnthropicによる公式製品認定・商標利用許諾を意味するものではないため、独立したOSSであることは明記する。

## 問題

### ローカル設定がignoreされていない

READMEでは`claudex.example.yaml`を`claudex.yaml`へコピーし、ローカル認証キーを設定する。一方、現在の`.gitignore`は`claudex.yaml`を除外していないため、実キーを誤ってコミットする可能性がある。

### READMEが現在のモデル構成と一致していない

現在の実装と設定例では、Claude互換IDを次のCodexモデルへ割り当てる。

- Opus: `gpt-5.6-sol`
- Sonnet: `gpt-5.6-luna`
- Haiku: `gpt-5.6-terra`

README.md、README.ja.md、Desktop launcher関連文書には、すべてLunaへ割り当てる説明やDesktopモデルが1件固定という古い記述が残っている。

### Publicプロジェクトとしての利用境界が不足している

コードはループバックbind、リモート管理無効、プラグイン無効、GPT-5.6系モデル限定を強制しているが、READMEでは個人利用、認証情報の非共有、非公式プロジェクトという境界が十分に説明されていない。

### Go module pathと配布方法が一致していない

`go.mod`はupstreamのmodule pathである`github.com/router-for-me/CLIProxyAPI/v7`を維持している。この状態では、次のようなClaudex自身のmodule pathによる導入と整合しない。

```sh
go install github.com/f4ah6o/claudex/cmd/claudex@latest
```

## 対応方針

### 1. 秘密情報の保護

- [ ] `/claudex.yaml`を`.gitignore`へ追加する
- [ ] `/desktop-preferences-backup.json`を`.gitignore`へ追加する
- [ ] 必要に応じて`*.local.yaml`などローカル設定用パターンを追加する
- [ ] Public化前に全履歴を対象としたsecret scanを実施する
- [ ] 検出された実トークンやキーがある場合は、履歴除去だけでなく失効・再発行する

実行例:

```sh
gitleaks detect --log-opts="--all"
```

### 2. READMEを現在の実装へ合わせる

- [ ] README.mdをSol/Luna/Terraの3モデル構成へ更新する
- [ ] README.ja.mdを同じ内容へ更新する
- [ ] Desktopのモデル一覧と実際のルーティングを正確に記載する
- [ ] `issues/open/20260723-claude-desktop-launcher.md`を実装済みの仕様へ合わせるか、完了済み記録へ移動する

### 3. 名称の由来と非公式プロジェクトであることを明記する

READMEの日英双方に、名称の由来とOpenAIおよびAnthropicの公式製品・提携製品ではないことを追加する。

英語文案:

> Claudex implements the workflow named “claudex” by Thibault Sottiaux. It is an independent open-source project and is not an official OpenAI or Anthropic product. Claude, Claude Code, Codex, and OpenAI are trademarks of their respective owners.

日本語文案:

> Claudexは、Thibault Sottiauxが「claudex」と名付けたワークフローを実装する独立したオープンソースプロジェクトです。OpenAIまたはAnthropicの公式製品・提携製品ではありません。Claude、Claude Code、Codex、OpenAIなどの名称は各権利者に帰属します。

- [ ] README.mdへ追加する
- [ ] README.ja.mdへ追加する
- [ ] 名称の由来として上記X投稿を参照する

### 4. サポート対象となる利用境界を明記する

- [ ] 自分自身のOpenAIアカウントまたはAPIキーのみを使用することを記載する
- [ ] OAuthトークン、APIキー、アカウントを他者と共有しないことを記載する
- [ ] ホステッドサービス、複数ユーザー向け共有ゲートウェイ、認証情報の再配布をサポートしないことを記載する
- [ ] 利用上限、プラン制限、アクセス制御の回避を目的とした利用をサポートしないことを記載する
- [ ] OpenAIおよびAnthropicの利用規約・ポリシーに従う責任が利用者にあることを記載する
- [ ] ループバック限定を解除しないことをセキュリティ境界として明記する

### 5. Go module pathと配布方法を決める

次のどちらかを選択する。

- [ ] `github.com/f4ah6o/claudex`系へmodule pathと内部importを変更する
- [ ] upstream module pathを維持し、clone後のビルドまたはReleaseバイナリのみをサポートするとREADMEに明記する

upstream同期を重視する場合は、module path維持とReleaseバイナリ配布の組み合わせが現実的。

### 6. Public向けCIとライセンス確認

- [ ] mainへのpushでfocused testとbuildが実行されることを確認する
- [ ] `govulncheck ./...`または対象packageへの検査をCIへ追加する
- [ ] macOS Desktop launcherのビルド確認方法を文書化する
- [ ] Releaseバイナリを配布する場合は依存ライセンスを検査する
- [ ] 必要に応じて`THIRD_PARTY_NOTICES`を生成する

## 受け入れ条件

- [ ] 実キーを含む`claudex.yaml`および設定バックアップが通常操作でGit管理対象にならない
- [ ] 全Git履歴のsecret scan結果に未対応の秘密情報がない
- [ ] README.mdとREADME.ja.mdがSol/Luna/Terraの現在の実装と一致する
- [ ] 名称の由来と、OpenAI・Anthropicの非公式プロジェクトであることが明記されている
- [ ] ローカル・単一ユーザー用途というセキュリティおよび利用境界が明確である
- [ ] module pathと公式な導入方法が矛盾していない
- [ ] Public化するmain revisionでテスト、ビルド、脆弱性検査が通る
- [ ] MITライセンスとupstreamの著作権表示が保持されている
