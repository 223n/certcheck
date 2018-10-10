# certcheck

[![GitHub](https://img.shields.io/badge/license-Apache%20Version%202.0-blue.svg)](LICENSE)

## なにするもの？

1. 証明書の有効期限を確認します。
2. 残日数が指定された日数以下の場合には、Slackに通知します。

## 使い方

### インストール

  [releases](https://github.com/223n/certcheck/releases)から、実行ファイルをダウンロードします。

### 設定ファイルの作成

  ダウンロードした実行ファイルと同じディレクトリに、
  設定ファイルを作成、配置します。

### 起動

  実行ファイルを起動します。

  実行ログには、実行結果が出力されます。

  また、残日数が指定日数より少ない場合には、Slackに通知されます。

## 設定ファイル

[certcheck.yml.format](certcheck.yml.format) を参考に **certcheck.yml** ファイルを作成してください。

* フォーマットと定義例

```yml
targets:
  -
    name: (分かりやすい名称)
    endpoint: (調べるURL)
    slackno: (slackの通知先 / slacksで定義しているnoを指定してください)
    threshold: (slackに通知する残日数)
  -
    name: xxxx
    endpoint: https://google.com
    slackno: 1
    threshold: 15
slacks:
  -
    name: (一意の番号)
    url: (slackの Incomming WebHooks で発行した Webhook URL)
    username: (slackの通知で表示するユーザ名)
  -
    name: 1
    url: https://hooks.slack.com/services/xxx/yyy/zzz
    username: certcheck
```

## ライセンス

このソースのライセンスは、 [LICENSE](LICENSE) を参照してください。

### その他

このソースの一部は、 [Apache License, Version 2.0](http://www.apache.org/licenses/LICENSE-2.0) のライセンスで配布されている成果物を含んでいます。

* [ynozue/apichecker](https://github.com/ynozue/apichecker) / Copyright (C) 2017 ynozue
