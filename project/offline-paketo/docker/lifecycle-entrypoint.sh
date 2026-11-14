#!/bin/sh
# CNB lifecycle 执行器：编排逻辑在 cnb（lifecycle run），本脚本只转交控制权。
exec /cnb/tools/cnb lifecycle run "$@"
