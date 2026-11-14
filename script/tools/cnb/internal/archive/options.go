package archive

// Options 控制解包行为。
type Options struct {
	// Strip 去掉前 N 层目录。
	Strip int
	// Include 是解包白名单（通配符）；为空表示全部。
	Include []string
}
