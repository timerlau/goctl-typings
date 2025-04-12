# go-typings

goctl plugin，根据 api 生成 typescript 的 interface 结构，生成的参数都是可选项

## 安装

```bash
go install github.com/timerlau/goctl-typings@latest
```

## 使用

```bash
goctl api plugin -plugin "goctl-typings=typings --filename=tmp/typings.d.ts" -api app/backend/cmd/api/idl/main.api
```
