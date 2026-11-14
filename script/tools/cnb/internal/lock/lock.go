// Package lock 解析 assets.lock.toml：物料清单的唯一事实源。
//
// 每条物料按作用域分区，避免不同含义的字段混在一起：
//
//	[[asset]]
//	id / version / default        身份
//	[asset.info]                 元信息（给人看）
//	[asset.unpack]               解包行为（kind/strip/dir/expect）
//	[asset.platform.<platform>]  下载源（url/sha256）
package lock
