# offline-paketo 测试

## 入口

```bash
just test-quick
```

`quick.sh` 执行 builder 构建和 pipeline plan 冒烟，不依赖 run image。
完整 lifecycle e2e 仍使用：

```bash
just test-case /path/to/app
```

## 历史

项目级测试从通用 `test/` 逐步迁移到这里。
