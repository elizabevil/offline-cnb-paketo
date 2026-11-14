#!/bin/sh
# CNB lifecycle 执行器：编排逻辑全部在 Go（cnb lifecycle run），
# 本脚本只负责把控制权交出去，避免 shell 兼容问题。
exec /cnb/tools/cnb lifecycle run "$@"
