# procfile fixture

被测应用是一个**已编译的静态二进制** `app`（不在此目录，由 `just fixtures` 生成），
`Procfile` 只声明进程：`web: ./app`。

源码在 `test/fixtures-src/procfile-app/`，构建产物放到本目录；
本目录在测试时只包含：`Procfile` + `app`。
