# certcheck

[![GitHub](https://img.shields.io/badge/license-Apache%20Version%202.0-blue.svg)](LICENSE)
[![CI](https://github.com/223n/certcheck/actions/workflows/ci.yml/badge.svg)](https://github.com/223n/certcheck/actions/workflows/ci.yml)

## なにするもの？

1. 証明書の有効期限を確認します。
1. 残日数が指定された日数以下の場合には、Slackに通知します。

## 使い方

### インストール

[releases](https://github.com/223n/certcheck/releases)から、実行ファイルを
ダウンロードします。

配布している実行ファイルにコード署名は行っていません。

### 設定ファイルの作成

ダウンロードした実行ファイルと同じディレクトリに、設定ファイルを作成、
配置します。

### 起動

実行ファイルを起動します。

実行ログには、実行結果が出力されます。

また、残日数が指定日数より少ない場合には、Slackに通知されます。

```bash
certcheck
certcheck -c="config.yml"
```

#### 引数

| 引数名 | 説明 | 例 |
| :-: | --- | --- |
| c | 設定ファイルを指定できます。 | `certcheck -c="config.yml"` |
| v | バージョン情報を出力します。 | `certcheck -v` |

## 設定ファイル

[certcheck.yml.format](certcheck.yml.format)を参考に、**certcheck.yml**
ファイルを作成してください。

なお、`targets`でSlackの設定（`hook_url`、`channel`、`username`、`icon`）を
指定した場合は、`slack`の設定を上書きします。

### フォーマット

```yaml
targets:
  - name:       (分かりやすい名称)
    endpoint:   (調べるURL)
    threshold:  (Slackに通知する残日数)
    hook_url:   (SlackのIncoming Webhooksで発行したWebhook URL / 上書き項目)
    channel:    (Slackの通知で投稿するチャンネル / 上書き項目)
    username:   (Slackの通知で表示するユーザー名 / 上書き項目)
    icon:       (Slackの通知で表示するアイコン / 上書き項目)
slack:
  hook_url:     (SlackのIncoming Webhooksで発行したWebhook URL)
  channel:      (Slackの通知で投稿するチャンネル)
  username:     (Slackの通知で表示するユーザー名)
  icon:         (Slackの通知で表示するアイコン)
```

### 設定項目

| 項目 | 必須 | 説明 |
| --- | :-: | --- |
| `targets[].name` | ○ | 分かりやすい名称。ログの識別に使用します。 |
| `targets[].endpoint` | ○ | 調べるURL。`http://`を指定するとエラーになります。 |
| `targets[].threshold` | | Slackに通知する残日数。既定値は0です。 |
| `targets[].hook_url` | | `slack.hook_url`を上書きします。 |
| `targets[].channel` | | `slack.channel`を上書きします。 |
| `targets[].username` | | `slack.username`を上書きします。 |
| `targets[].icon` | | `slack.icon`を上書きします。 |
| `slack.hook_url` | ○ | Incoming Webhooksで発行したWebhook URL。 |
| `slack.channel` | | 通知を投稿するチャンネル。 |
| `slack.username` | | 通知で表示するユーザー名。 |
| `slack.icon` | | 通知で表示するアイコン。例: `:lock:` |

## 動作仕様

### 通知の条件

残日数が`threshold`以下になったときに、Slackへ通知します。

エンドポイントへ接続できなかった場合も、`NG:`で始まるメッセージを通知します。

残日数に余裕がある場合は、ログに出力するだけで通知はしません。

出力するメッセージは、次の3種類です。

```text
Cert OK: https://example.com expire: 2026/11/02 17:39 at 61 days
Cert Warning: https://example.com expire: 2026/11/02 17:39 at 5 days
NG: request "https://example.com": dial tcp: connection refused
```

有効期限は、日本標準時（UTC+9）で表示します。

### エラー時のふるまい

1つのターゲットで問題が起きても、残りのターゲットの確認は続きます。

| 状況 | ふるまい |
| --- | --- |
| 設定ファイルを読み込めない | エラーを出力して終了します。 |
| `name`や`endpoint`が空 | そのターゲットを読み飛ばします。 |
| エンドポイントへ接続できない | `NG:`を通知して次へ進みます。 |
| Slackへの通知に失敗した | エラーを出力して次へ進みます。 |

### タイムアウト

1つのエンドポイントへの接続とTLSハンドシェイクは、10秒で打ち切ります。

Slackへの通知も同じく10秒で打ち切ります。

### 終了コード

| 終了コード | 意味 |
| :-: | --- |
| 0 | 正常終了。ターゲット個別の失敗はここに含みます。 |
| 1 | 設定ファイルの読み込みまたは引数の解析に失敗しました。 |

## 開発

### 必要なもの

Go 1.24以降が必要です。

### ディレクトリ構成

```text
main.go                  コマンドの入り口。引数の解析と組み立てを行う
internal/config/         設定ファイルの読み込み、検証、上書きの解決
internal/checker/        証明書の取得と残日数の判定
internal/notify/         通知先への配送（Slack Incoming Webhooks）
internal/runner/         ターゲットごとの処理順序の制御
docs/ARCHITECTURE.md     設計の意図と依存関係の説明
```

設計の詳細は[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)を参照してください。

### ビルド

```bash
go build .
```

### テスト

```bash
go test -race -cover ./...
```

外部のネットワークへは接続しません。証明書の検証は、テスト内で発行した
使い捨ての証明書を`httptest`のTLSサーバーに載せて確認します。

### 静的解析

```bash
gofmt -l .
go vet ./...
```

### CI

`master`へのpushとpull requestで、GitHub Actionsが次を実行します。

1. `gofmt`、`go vet`、`go test -race -cover ./...`
1. Windows、Linuxそれぞれのamd64、386向けクロスビルド

### リリース

GitHubのActionsタブから**Release**ワークフローを手動実行し、`version`に
タグ名（例: `v1.3.0`）を指定します。

タグの作成、リリースノートの生成、実行ファイルの添付まで自動で行います。

| 入力 | 説明 |
| --- | --- |
| `version` | タグ名。`v1.2.3`または`v1.2.3-rc1`の形式のみ受け付けます。 |
| `prerelease` | pre-releaseとして公開するかどうか。既定値はfalseです。 |

## ライセンス

このソースのライセンスは、[LICENSE](LICENSE)を参照してください。

### その他

このソースの一部は、[Apache License, Version 2.0](http://www.apache.org/licenses/LICENSE-2.0)
のライセンスで配布されている成果物を含んでいます。

- [ynozue/apichecker](https://github.com/ynozue/apichecker) / Copyright (C) 2017 ynozue
