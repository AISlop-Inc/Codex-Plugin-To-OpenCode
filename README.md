# Codex MCP Portal

Codex / ChatGPTで接続済みのGoogle Driveなどを、**標準のstdio MCPサーバーとして公開する**TypeScript製のポータルです。  
Codexの保存済みChatGPT認証情報を読み、OpenAI側の共通MCPへ要求を中継します。

OpenCode専用プラグインではなく、stdio MCPを起動できるクライアントから利用できます。  
**Googleへの再認可や、トークンをクライアント設定へコピーする操作は不要です。**  
非公式の連携であり、ChatGPT専用エンドポイントの変更によって動作しなくなる可能性があります。

GitHubリポジトリのURLは従来の`AISlop-Inc/Codex-Plugin-To-OpenCode`を使用しています。

## AIエージェントにインストールを頼む

以下をOpenCodeなどのエージェントへ渡してください。

```text
https://github.com/AISlop-Inc/Codex-Plugin-To-OpenCode をインストールしてください。
README.mdとdocs/INSTALL_FOR_AGENTS.mdを読み、利用中のMCPクライアントに登録してください。
クライアントが不明なら確認し、既存の設定を保持してください。
トークンやauth.jsonの内容を表示・コピーしないでください。
接続状態とツール一覧を確認し、Google Driveが利用可能なら最近のファイル1件の
メタデータ取得で動作確認してください。書き込み操作は不要です。
```

手順の詳細は[AIエージェント向けインストールガイド](docs/INSTALL_FOR_AGENTS.md)にあります。

## 必要なもの

- Node.js 24以降、Git、stdio MCPに対応したクライアント。
- CodexでChatGPTアカウントにログイン済み。
- `${CODEX_HOME:-$HOME/.codex}/auth.json`に`tokens.access_token`と`tokens.account_id`があること。
- ChatGPT側で目的のPluginが利用可能で、必要なサービス接続が済んでいること。

APIキーのみのログイン、OSのキーチェーンだけに保存された認証情報には対応していません。  
確認した環境はmacOS・Node.js 24で、以下はmacOS/Linux向けのシェル例です。

## インストール

### CLIとしてインストール

```sh
npm install -g git+https://github.com/AISlop-Inc/Codex-Plugin-To-OpenCode.git
codex-mcp-portal --help
```

Gitからのインストール時にTypeScriptをビルドし、`codex-mcp-portal`コマンドを登録します。  
引数なしで起動するとstdio MCPとして待機します。  
npmレジストリには公開していないため、インストール元には上記Git URLを指定してください。

### cloneして直接実行

インストール先が未使用の場合の手順です。

```sh
INSTALL_DIR="$HOME/.local/share/codex-mcp-portal"
git clone https://github.com/AISlop-Inc/Codex-Plugin-To-OpenCode.git "$INSTALL_DIR"

# mise/nvmなどのshimではなく、実行中のNodeの絶対パスを保存する
NODE_BIN="$(node -p 'process.execPath')"
"$NODE_BIN" --version

"$NODE_BIN" "$INSTALL_DIR/bridge.ts" --help
```

cloneしたソースはNode.jsで直接実行できるため、**この方式ではnpm installやビルドは不要**です。  
Nodeのバージョンマネージャーを利用している場合は、事前にNode.js 24以降を有効にしてください。  
登録したNodeの実体を削除・更新した場合は、クライアント設定の実行パスも更新します。

## MCPクライアントへの登録

### 汎用の設定例

`mcpServers`形式のクライアントでは、次のエントリーを既存設定へ追加します。  
設定ファイルの場所と再読み込み方法はクライアントのドキュメントに従ってください。

```json
{
  "mcpServers": {
    "codex-mcp-portal": {
      "command": "codex-mcp-portal",
      "args": []
    }
  }
}
```

GUIアプリからコマンドが見つからない場合は、`command -v codex-mcp-portal`で確認した絶対パスを使ってください。  
そのCLIを起動する`node`もクライアントのPATHから解決できる必要があります。  
PATHに依存させたくない場合は、clone方式で次のようにNodeとソースの両方を絶対パスにします。

```json
{
  "mcpServers": {
    "codex-mcp-portal": {
      "command": "/absolute/path/to/node",
      "args": ["/absolute/path/to/codex-mcp-portal/bridge.ts"]
    }
  }
}
```

`CODEX_HOME`が標準以外なら、サーバーの`env`へ`{"CODEX_HOME": "/absolute/path/to/codex-home"}`を追加します。  
MCPクライアントにはHTTP URLではなく、ローカルの起動コマンドを登録してください。

### OpenCode V2

従来と同じ`codex_apps`名で登録できます。  
すでに登録済みの場合は、重複登録せず既存の起動先を確認してください。

```sh
# CLIをインストールした場合
opencode mcp add codex_apps --global -- codex-mcp-portal

# cloneした場合（上のINSTALL_DIRとNODE_BINを使用）
opencode mcp add codex_apps --global -- "$NODE_BIN" "$INSTALL_DIR/bridge.ts"

opencode mcp list
```

登録コマンドはインストール方式に合う方を一つだけ実行します。  
`CODEX_HOME`を標準以外にしている場合は、登録時に明示します。

```sh
opencode mcp add codex_apps --global \
  --env "CODEX_HOME=$CODEX_HOME" \
  -- "$NODE_BIN" "$INSTALL_DIR/bridge.ts"
```

OpenCode V2の設定は保存後に反映されます。  
`codex_apps connected`を確認し、「Google Driveの最近のファイルを1件見せて」などで読み取りを試してください。  
認識されない場合は`/mcps`で接続状態を確認します。

#### OpenCodeの手動設定

既存設定へ次の`mcp.servers.codex_apps`をマージします。  
設定ファイル全体を置き換えず、パスは自分の環境の絶対パスにしてください。

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "servers": {
      "codex_apps": {
        "type": "local",
        "command": [
          "/absolute/path/to/node",
          "/absolute/path/to/codex-mcp-portal/bridge.ts"
        ]
      }
    }
  }
}
```

## 仕組み

```text
MCPクライアント ──stdio──> Codex MCP Portal ──HTTPS──> ChatGPT共通MCP
                               │                        │
                               └─ Codexのauth.json       └─ Google Driveなど
```

送信先は`https://chatgpt.com/backend-api/ps/mcp`に固定しています。  
`Authorization`、`ChatGPT-Account-ID`、`X-OpenAI-Product-Sku: codex`を付け、JSONまたはSSEの応答を返します。  
認証情報は設定へ書き出さず、HTTPポートも開きません。

- Codexが保存した新しいアクセストークンは、次の要求で読み直します。
- refresh tokenは使用せず、認証の更新はCodex側に任せます。
- 別ホストへ認証情報が送られないよう、HTTPリダイレクトを拒否します。
- 実行中にアカウントIDが変わったら要求を止め、再接続を求めます。
- 二重書き込みを避けるため、失敗した要求を自動再送しません。
- ツール呼び出しは最大10分、それ以外の要求は最大60秒でタイムアウトします。

## 対応範囲と検証状況

| 項目 | 状況 |
| --- | --- |
| 共通MCPのツール一覧 | 開発時のアカウントで335ツールを取得 |
| CLI / stdio | ビルド済みCLI、npm形式のシンボリックリンク経由起動、JSON-RPC入出力をテスト |
| Google Drive | OpenCode経由で最近のファイル1件のメタデータ取得に成功 |
| その他のMCPクライアント | 標準stdio接続用の設定を用意、個別アプリでの実機操作は未検証 |
| Gmail・Calendar・GitHubなど | アカウントが公開するツールを中継、個別の実操作は未検証 |
| 書き込みツール | 中継対象だが実操作は未検証 |
| Skills | このリポジトリではインストールしない |
| カスタムUI・サーバーからの入力要求・sampling | 非対応 |
| ChatGPT同梱ブラウザー・Computer Use・Hooks | 非対応 |

ツール数や利用できるサービスはアカウントによって変わります。  
ブリッジ自身には読み取り専用フィルターがなく、共通MCPが返す書き込みツールも公開します。  
ツールの実行権限は接続元のMCPクライアント側で管理してください。

## トラブルシューティング

| 症状 | 確認すること |
| --- | --- |
| `Codex ChatGPT credentials unavailable` | CodexのChatGPTログイン、`CODEX_HOME`、ファイル形式の認証保存先を確認 |
| HTTP 401 / 403 | Codex側でログイン状態を更新し、クライアントで再接続。403は認証以外の接続条件やサービス変更でも起こり得る |
| `Codex account changed` | MCPサーバーを切断・再接続し、切り替え後のアカウントを使う |
| 起動時にNodeが見つからない | 設定したNodeの絶対パスとバージョンを確認 |
| ツールがない・サービス接続エラー | ChatGPT側のPluginと接続状態、対象アカウントを確認 |
| タイムアウト | 書き込み済みの可能性があるため、結果を確認してから再実行 |

認証が切れた場合、Portalから自動的にChatGPTへログインする機能はありません。  
Codexを使わず長期間運用する場合のトークン更新は未検証です。

## 開発

```sh
npm ci --ignore-scripts
npm run typecheck
npm test
npm run build
```

テストはモック通信のみで、実際の認証情報やGoogle Driveを使用しません。  
ランタイム依存パッケージはなく、TypeScriptとNode型定義だけを開発用に使用します。

`npm run build`は配布用の`dist/bridge.js`を生成します。  
npmインストール先ではNode.jsのTypeScript直接実行に制限があるため、CLIにはビルド済みJavaScriptを使用します。  
ソースの実装は引き続きTypeScriptです。

## 更新・削除

clone方式は、作業ツリーに変更がないことを確認して`git pull --ff-only`を実行します。  
CLI方式は、同じGit URLから`npm install -g`を再実行します。  
更新後はMCPクライアントで切断・再接続して新しいコードを起動してください。

削除時は、クライアント設定からこのMCPサーバーだけを削除します。  
CLI方式では`npm uninstall -g codex-mcp-portal`を実行します。  
clone方式では、必要ならclone先を削除してください。  
Codexの認証情報やChatGPT側の接続を削除する必要はありません。

## 参考

- [OpenCode V2 MCP](https://opencode.ai/v2/docs/mcp-servers)
- [OpenAI MCPドキュメント](https://developers.openai.com/codex/mcp)
- [Codexの共通MCP設定](https://github.com/openai/codex/blob/main/codex-rs/codex-mcp/src/mcp/mod.rs)
- [Codexの認証プロバイダー](https://github.com/openai/codex/blob/main/codex-rs/model-provider/src/auth.rs)

## ライセンス

[MIT](LICENSE)
