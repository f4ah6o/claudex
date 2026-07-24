# Prepare Claudex for public release

Status: closed
Created: 2026-07-24
Updated: 2026-07-24
Closed: 2026-07-24
Branch: fix/20260724-public-release-readiness

## 概要

ClaudexをPublicリポジトリとして公開するため、秘密情報の誤コミット防止、ドキュメントと実装の整合、独立したOSSとしての境界、配布方法、CIを整理した。

`Claudex`という名称は、OpenAIでCodexを率いるThibault SottiauxがClaude CodeをCodexモデルへ接続するワークフローの名称として使用したものに由来する。

- https://x.com/thsottiaux/status/2076119366647894371?s=46

名称は維持する。READMEにはOpenAIまたはAnthropicの公式・提携製品ではない独立したOSSであることを明記した。

## 完了内容

### 秘密情報の保護

- `/claudex.yaml`を`.gitignore`へ追加した
- `/desktop-preferences-backup.json`を`.gitignore`へ追加した
- `*.local.yaml`を`.gitignore`へ追加した
- 全Git履歴を対象とするGitleaks CIを追加した
- Gitleaksのredact済みレポートをworkflow artifactとして保存するようにした

初回の全履歴scanでは、fork元の`router-for-me/CLIProxyAPI`公開履歴に含まれるGemini、Antigravity、Qwen、iFlow用OAuthクライアント定数を26件検出した。これらはClaudex固有または利用者固有の資格情報ではなく、同じcommitがupstreamの公開履歴に存在することを確認した。

`.gitleaksignore`には、この26件のexact fingerprintだけをbaselineとして登録した。ファイル単位やrule単位の包括的な除外は行わないため、新規・変更されたsecret findingは引き続きCIを失敗させる。

baseline適用後、全Git履歴のsecret scanは成功し、未対応のfindingは0件となった。

### READMEと実装の整合

- README.mdとREADME.ja.mdをSol/Terra/Lunaの3モデル構成へ更新した
- Opusを`gpt-5.6-sol`、Sonnetを`gpt-5.6-terra`、Haikuを`gpt-5.6-luna`へ割り当てることを記載した
- Claude Desktopのモデル一覧が3件であることを記載した
- `issues/open/20260723-claude-desktop-launcher.md`を実装済み記録として`issues/closed/`へ移した
- `CHANGES.md`の1モデル固定という古い記述を更新した

### 名称とプロジェクト境界

- Thibault Sottiauxによる名称の由来をREADMEの日英双方へ記載した
- OpenAIまたはAnthropicの公式製品・提携製品ではないことを記載した
- 自分自身のOpenAIアカウント、OAuthセッション、APIキーだけを使用することを記載した
- OAuthトークン、APIキー、アカウント、認証ファイルの共有・再配布をサポートしないことを記載した
- ホステッドサービス、共有ゲートウェイ、複数ユーザー向け認証情報ブローカーをサポートしないことを記載した
- 利用上限、プラン制限、アクセス制御、ポリシー適用の回避をサポートしないことを記載した
- ループバック限定をセキュリティ境界として明記した

### module pathと配布方法

upstream同期を維持するため、module pathは`github.com/router-for-me/CLIProxyAPI/v7`のまま保持する。

- `go install github.com/f4ah6o/claudex/...`をサポートしないことを記載した
- clone後の`just setup`または`go build ./cmd/claudex`を公式なソース導入方法として記載した
- tag付きGitHub Releaseに添付されたバイナリだけを公式配布物として扱うことを記載した

### CI

- Linux上のformat check、focused test、CLI/Desktop command buildが成功した
- macOS runner上の`ClaudexDesktop.app` bundle生成と内容検証が成功した
- `govulncheck ./...`が成功した
- baseline適用後の全Git履歴secret scanが成功した
- `script/build_and_run.sh --build-only`を追加し、アプリを起動せずbundleを検証できるようにした

## 受け入れ結果

- [x] 実キーを含む`claudex.yaml`および設定バックアップが通常操作でGit管理対象にならない
- [x] 全Git履歴のsecret scan結果に未対応の秘密情報がない
- [x] README.mdとREADME.ja.mdがSol/Terra/Lunaの現在の実装と一致する
- [x] 名称の由来と、OpenAI・Anthropicの非公式プロジェクトであることが明記されている
- [x] ローカル・単一ユーザー用途というセキュリティおよび利用境界が明確である
- [x] module pathと公式な導入方法が矛盾していない
- [x] Public化対象revisionでtest、build、macOS bundle、脆弱性検査が通る
- [x] MITライセンスとupstreamの著作権表示が保持されている
