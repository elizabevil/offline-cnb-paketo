package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"bksmartpacker/pkg/types"
)

// CopyEnvsFile 复制 EnvsOverride 目录下的所有文件到工作目录的 Envs 目录
func CopyEnvsFile(workspace string, envsOverride string) error {
	os.Mkdir(filepath.Join(workspace, "Envs"), os.ModePerm)
	files, err := os.ReadDir(envsOverride)
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := Copy(filepath.Join(envsOverride, file.Name()), filepath.Join(workspace, "Envs", file.Name())); err != nil {
			return err
		}
	}
	return nil
}

// GenerateEnvs 渲染 AppDesc 中声明的环境变量
func GenerateEnvs(workspace string, appDesc types.ApplicationDescription) error {
	os.Mkdir(filepath.Join(workspace, "Envs"), os.ModePerm)
	for _, moduleDesc := range appDesc.Modules {
		var builder strings.Builder
		for _, item := range moduleDesc.Spec.Configuration.Env {
			if intValue, err := strconv.Atoi(item.Value); err == nil {
				builder.WriteString(fmt.Sprintf("%s=%d\n", item.Name, intValue))
			} else {
				builder.WriteString(fmt.Sprintf("%s=\"%s\"\n", item.Name, doubleQuoteEscape(item.Value)))
			}
		}
		if err := os.WriteFile(filepath.Join(workspace, "Envs", moduleDesc.Name+".env"), []byte(builder.String()), 0777); err != nil {
			return err
		}
	}
	return nil
}

const doubleQuoteSpecialChars = "\\\n\r\"!$`"

func doubleQuoteEscape(line string) string {
	for _, c := range doubleQuoteSpecialChars {
		toReplace := "\\" + string(c)
		if c == '\n' {
			toReplace = `\n`
		}
		if c == '\r' {
			toReplace = `\r`
		}
		line = strings.Replace(line, string(c), toReplace, -1)
	}
	return line
}
