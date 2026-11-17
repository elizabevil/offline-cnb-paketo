package config

import (
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// Config 配置
type Config struct {
	Bp BuildpackCfg
}

// BuildpackCfg buildpack 配置
type BuildpackCfg struct {
	Python PythonBpCfg
	Go     GoBpCfg
	NodeJS NodeJSBpCfg
	Apt    AptBpCfg
}

// PythonBpCfg python buildpack 配置
type PythonBpCfg struct {
	Version string
	Type    string

	S3BaseURL        string
	PipVersion       string
	PipIndexURL      string
	PipExtraIndexURL string
}

// GoBpCfg go buildpack 配置
type GoBpCfg struct {
	Version string
	Type    string

	GoBucketURL string
}

// NodeJSBpCfg nodejs buildpack 配置
type NodeJSBpCfg struct {
	Version string
	Type    string

	StdLibFileURL string
	S3Domain      string
	NpmRegistry   string
}

// AptBpCfg apt buildpack 配置
type AptBpCfg struct {
	Version string
	Type    string
}

// G 全局配置
var G *Config

// Init 初始化配置
func Init(cfgFile string) error {
	cfg := new(Config)

	// 使用 viper 从 cfgFile 加载配置
	vp := viper.New()

	// 设置默认值
	vp.SetDefault("bp.python.version", "v213")
	vp.SetDefault("bp.python.type", "oci-embedded")
	vp.SetDefault("bp.python.s3BaseURL", "https://bkpaas-runtimes-1252002024.file.myqcloud.com/python")
	vp.SetDefault("bp.python.pipVersion", "20.2.3")
	vp.SetDefault("bp.python.pipIndexURL", "https://mirrors.cloud.tencent.com/pypi/simple/")
	vp.SetDefault("bp.python.pipExtraIndexURL", "https://mirrors.tencent.com/tencent_pypi/simple/")

	vp.SetDefault("bp.go.version", "v191")
	vp.SetDefault("bp.go.type", "oci-embedded")
	vp.SetDefault("bp.go.goBucketURL", "https://bkpaas-runtimes-1252002024.file.myqcloud.com/golang")

	vp.SetDefault("bp.nodejs.version", "v163")
	vp.SetDefault("bp.nodejs.type", "oci-embedded")
	vp.SetDefault(
		"bp.nodejs.stdLibFileURL",
		"https://bkpaas-runtimes-1252002024.file.myqcloud.com/common/buildpack-stdlib/bk-node/stdlib.sh",
	)
	vp.SetDefault(
		"bp.nodejs.s3Domain",
		"https://bkpaas-runtimes-1252002024.file.myqcloud.com/nodejs/node/release/linux-x64/",
	)
	vp.SetDefault("bp.nodejs.npmRegistry", "https://mirrors.tencent.com/npm/")

	vp.SetDefault("bp.apt.version", "v2")
	vp.SetDefault("bp.apt.type", "oci-embedded")

	if cfgFile != "" {
		vp.SetConfigFile(cfgFile)
		if err := vp.ReadInConfig(); err != nil {
			return errors.Wrapf(err, "read config file %s", cfgFile)
		}
	}

	if err := vp.Unmarshal(cfg); err != nil {
		return errors.Wrap(err, "unmarshal config")
	}

	G = cfg

	return nil
}
