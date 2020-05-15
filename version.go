package main

import (
	"bytes"
	"fmt"
	"strings"
)

//语义字母表
const semanticAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"

//这些常量定义应用程序版本并遵循语义
//版本控制2.0.0规范(http://semver.org/).

const (
	appMajor uint = 0
	appMinor uint = 12
	appPatch uint = 0

	//AppPreRelease只能包含SemanticAlphabet中的字符
	//根据语义版本规范。
	appPreRelease = "beta"
)

//AppBuild被定义为变量，因此可以在生成期间重写它
//如果需要，使用“-ldflags”-x main.appbuild foo”处理。它必须只有
//根据语义版本规范包含语义字母表中的字符。

var appBuild string

//version根据
//语义版本控制2.0.0规范（http://semver.org/）。
func version() string {

	// 从主要版本、次要版本和补丁版本开始.
	version := fmt.Sprintf("%d.%d.%d", appMajor, appMinor, appPatch)

	//如果有预发布版本，则附加预发布版本。连字符要求由语义版本控制规范自动附加，
	//并且应该 不包含在预发布字符串中。预发布版本 如果包含无效字符，则不追加。
	PreRelease := normalizeVerString(appPreRelease)
	if PreRelease != "" {
		version = fmt.Sprintf("%s-%s", version, PreRelease)
	}

	//如果存在任何生成元数据，则追加该元数据。要求的加号 由语义版本控制规范自动附加，
	//并且应该 不包含在生成元数据字符串中。生成元数据
	//如果字符串包含无效字符，则不追加字符串。

	build := normalizeVerString(appBuild)
	if build != "" {
		version = fmt.Sprintf("%s+%s", version, build)
	}

	return version

}

//normalizeverstring返回从以下所有字符中删除的传递字符串：
//根据预发布的语义版本控制指南，无效 版本和生成元数据字符串。
//尤其是它们必须只包含 语义字母表中的字符。
func normalizeVerString(str string) string {
	var result bytes.Buffer
	for _, r := range str {
		if strings.ContainsRune(semanticAlphabet, r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// over
