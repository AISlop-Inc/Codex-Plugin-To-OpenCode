> AISlop-Inc npm package: `@aislop-inc/codex-mcp-portal`.
> Manual release instructions: [npm publishing](docs/NPM-PUBLISHING.md).

# Codex MCP Portal

Codex / ChatGPTで接続済みのGoogle Driveなどを、**標準のstdio MCPサーバーとして公開するGo製のポータル**です。  
Codexの保存済みChatGPT認証情報を読み、OpenAI側の共通MCPへ要求を中継します。

OpenCode専用プラグインではなく、stdio MCPを起動できるクライアントから利用できます。  
**実行時のNode.js、npm、Goランタイムは不要です。**  
非公式の連携であり、ChatGPT専用エンドポイントの変更によって動作しなくなる可能性があります。

GitHubリポジトリのURLは`AISlop-Inc/Codex-Plugin-To-OpenCode`を使用しています。

## インストール

### go install

ビルドにはGo 1.24以降が必要です。

```sh
go install github.com/AISlop-Inc/Codex-Plugin-To-OpenCode/cmd/codex-mcp-portal@latest
```

実行ファイルは`go env GOBIN`の出力先、未設定なら通常`$(go env GOPATH)/bin`へ入ります。  
そのディレクトリをPATHへ追加するか、MCP設定へバイナリの絶対パスを指定します。

```sh
codex-mcp-portal --help
```

引数なしで起動するとstdio MCPとして待機します。  
バイナリのダウンロード配布はまだ用意していないため、現在はGoでビルドしてください。

### ソースからビルド

以下はmacOS/Linux向けの例です。  
既存のディレクトリがある場合は内容を確認し、上書きしないでください。

```sh
INSTALL_DIR="$HOME/.local/share/codex-mcp-portal"
git clone https://github.com/AISlop-Inc/Codex-Plugin-To-OpenCode.git "$INSTALL_DIR"
cd "$INSTALL_DIR"
go build -o bin/codex-mcp-portal ./cmd/codex-mcp-portal
"$INSTALL_DIR/bin/codex-mcp-portal" --help
```

バイナリはソースディレクトリから独立して実行できます。  
外部Goモジュールへの依存はありません。

## 必要な認証と接続

- CodexでChatGPTアカウントにログイン済み。
- `${CODEX_HOME:-$HOME/.codex}/auth.json`に`tokens.access_token`と`tokens.account_id`があること。
- ChatGPT側で目的のPluginが利用可能で、必要なサービス接続が済んでいること。

APIキーのみのログイン、OSのキーチェーンだけに保存された認証情報には対応していません。  
Googleへの再認可や、トークンをMCPクライアントの設定へコピーする操作は不要です。

## MCPクライアントへの登録

### 汎用の設定例

`mcpServers`形式のクライアントでは、次のエントリーを既存設定へ追加します。  
設定ファイルの場所と再読み込み方法はクライアントのドキュメントに従ってください。

```json
{
  "mcpServers": {
    "codex-mcp-portal": {
      "command": "/absolute/path/to/codex-mcp-portal",
      "args": []
    }
  }
}
```

`command`にはビルドしたバイナリの絶対パスを指定します。  
`CODEX_HOME`が標準以外なら、サーバーの`env`へ`{"CODEX_HOME": "/absolute/path/to/codex-home"}`を追加します。  
HTTP URLではなく、ローカルの起動コマンドを登録してください。

### OpenCode V2

```sh
opencode mcp add codex_apps --global -- /absolute/path/to/codex-mcp-portal
opencode mcp list
```

すでに`codex_apps`がある場合は、重複登録せず既存の起動先を確認してください。  
`CODEX_HOME`が標準以外なら、登録時に`--env "CODEX_HOME=$CODEX_HOME"`を付けます。

手動で設定する場合は、次のエントリーを既存設定へマージします。

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "servers": {
      "codex_apps": {
        "type": "local",
        "command": ["/absolute/path/to/codex-mcp-portal"]
      }
    }
  }
}
```

OpenCode V2の設定は保存後に反映されます。  
`codex_apps connected`を確認し、読み取りで動作確認してください。  
接続の確認と再接続には`/mcps`を使えます。

## AIエージェントにインストールを頼む

```text
https://github.com/AISlop-Inc/Codex-Plugin-To-OpenCode をインストールしてください。
README.mdとdocs/INSTALL_FOR_AGENTS.mdを読み、利用中のMCPクライアントに登録してください。
クライアントが不明なら確認し、既存の設定を保持してください。
トークンやauth.jsonの内容を表示・コピーしないでください。
接続状態とツール一覧を確認し、Google Driveが利用可能なら最近のファイル1件の
メタデータ取得で動作確認してください。書き込み操作は不要です。
```

詳細は[AIエージェント向けインストールガイド](docs/INSTALL_FOR_AGENTS.md)にあります。

## 仕組み

```text
MCPクライアント ──stdio──> Codex MCP Portal ──HTTPS──> ChatGPT共通MCP
                               │                        │
                               └─ Codexのauth.json       └─ Google Driveなど
```

送信先は`https://chatgpt.com/backend-api/ps/mcp`に固定しています。  
`Authorization`、`ChatGPT-Account-ID`、`X-OpenAI-Product-Sku: codex`を付け、JSONまたはSSEの応答を返します。  
認証情報は設定へ書き出さず、HTTPポートも開きません。

- 要求ごとに認証ファイルを読み、Codexが保存した新しいアクセストークンを利用。
- refresh tokenは使用せず、認証更新はCodexに委任。
- HTTPリダイレクトを拒否し、別ホストへの認証情報送信を防止。
- 実行中のアカウント変更を検出したら、再接続するまで要求を停止。
- 二重書き込みを避けるため、失敗した要求のアプリケーション側自動再送なし。
- 複数要求を並行処理し、stdoutへのJSON-RPC応答は1メッセージ1行で出力。
- ツール呼び出しは最大10分、それ以外は最大60秒でタイムアウト。
- 入出力メッセージとSSEイベントは最大32 MiB。

## ソース構成

Goの一般的な`cmd/`・`internal/`構成で、入口・認証・通信の責務を分けています。

```text
cmd/codex-mcp-portal/
  main.go                 CLI、ヘルプ、終了コード
  main_test.go
internal/auth/
  credentials.go          Codex認証ファイルの読み取り
  credentials_test.go
internal/portal/
  client.go               HTTP転送、認証ヘッダー、SSE、セッション状態
  protocol.go             JSON-RPC検証と初期化要求の変換
  stdio.go                標準入出力と並行処理
  client_test.go
  protocol_test.go
docs/
  INSTALL_FOR_AGENTS.md
go.mod
```

## 対応範囲

| 項目 | 状況 |
| --- | --- |
| Goバイナリ | macOSで共通MCPの初期化と335ツール取得を実機確認 |
| Google Drive | Go版へ切り替えたOpenCodeから、最近のファイル1件のメタデータ取得に成功 |
| その他のツール | アカウントが公開するツールを中継、全サービスの実操作は未検証 |
| その他のMCPクライアント | 標準stdio設定を用意、個別アプリでの実機操作は未検証 |
| Skills・Hooks・ローカルPluginのMCP定義 | 読み込み対象外 |
| カスタムUI・サーバーからの入力要求・sampling | 非対応 |
| ChatGPT同梱ブラウザー・Computer Use | 非対応 |

ツール数と利用できるサービスはアカウントによって変わります。  
読み取り専用フィルターはなく、共通MCPが返す書き込みツールも公開します。  
実行権限は接続元のMCPクライアントで管理してください。

## トラブルシューティング

| 症状 | 確認すること |
| --- | --- |
| `Codex ChatGPT credentials unavailable` | Codexログイン、`CODEX_HOME`、ファイル形式の認証保存先 |
| HTTP 401 / 403 | Codex側でログインを更新して再接続。403はアクセス条件やサービス変更の可能性もある |
| `Codex account changed` | MCPサーバーを切断・再接続 |
| バイナリが見つからない | `go env GOBIN`、`go env GOPATH`、設定した絶対パス |
| ツールがない・接続エラー | ChatGPT側のPlugin、サービス接続、アカウント |
| タイムアウト | 書き込みが完了している可能性があるため、結果を確認してから再実行 |

Portalから自動でChatGPTへログインする機能はありません。  
Codexを使わず長期間運用する場合のトークン更新は未検証です。

## 開発と検証

```sh
go test -race ./...
go vet ./...
go build -o bin/codex-mcp-portal ./cmd/codex-mcp-portal
```

テストはモック通信と一時ファイルのみで、実際の認証情報やGoogle Driveを使用しません。  
認証の再読み取り、アカウント固定、リダイレクト拒否、非再送、SSE、大きなメッセージ、並行処理、JSON-RPC検証、CLIを検証します。

- `tools/call`の応答で`structuredContent`が`null`の場合だけ、その項目を省略して中継します。
- 本文、正常な構造化データ、エラー、メタデータは保持します。
- JSON応答とSSE応答の両方に適用し、`null`以外の不正な値は書き換えません。

## TypeScript版からの移行・更新・削除

旧版のnpmインストールや`node bridge.ts`起動は、Goバイナリに置き換えます。  
同じサーバー名を保ち、起動コマンド全体をバイナリの絶対パス1つへ変更してください。  
`CODEX_HOME`など必要な環境設定は引き継ぎます。  
Go版の接続確認後、不要なら旧npm版を`npm uninstall -g codex-mcp-portal`で削除できます。

更新は`go install ...@latest`を再実行するか、変更のないcloneで`git pull --ff-only`後に再ビルドします。  
実行中のプロセスは自動で差し替わらないため、更新後にMCPクライアントで再接続してください。

削除時はクライアント設定からこのサーバーだけを外し、必要ならバイナリを削除します。  
Codex認証情報とChatGPT側の接続は保持してください。

## 参考・ライセンス

- [OpenCode V2 MCP](https://opencode.ai/v2/docs/mcp-servers)
- [OpenAI MCPドキュメント](https://developers.openai.com/codex/mcp)
- [Codexの共通MCP設定](https://github.com/openai/codex/blob/main/codex-rs/codex-mcp/src/mcp/mod.rs)
- [MIT License](LICENSE)

## npm installation

After the first npm publication:

```sh
npm install -g @aislop-inc/codex-mcp-portal
codex-mcp-portal --help
# or
npx @aislop-inc/codex-mcp-portal --help
```

This npm distribution needs Node.js to launch the bundled Go binary.
It includes macOS, Linux and Windows binaries for x64 and arm64.
Standalone Go installations remain available without Node.js.
