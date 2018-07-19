package common

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// DataStrToBytes 把形如 123M 的字符串 转成字节数
func DataStrToBytes(s string) (uint64, error) {
	// 首先看看是不是数字，是就直接返回了
	m, err := strconv.ParseUint(s, 10, 64)
	if err == nil {
		return m, nil
	}

	// 有的可能是形如 123K 123M ，都转小写了处理
	s = strings.ToLower(s)

	re := regexp.MustCompile(`(\d+(?:\.\d+)?)([kmg])`)
	ss := re.FindStringSubmatch(s)
	if ss == nil {
		return 0, fmt.Errorf("wrong format: %s", s)
	}

	n, err := strconv.ParseFloat(ss[1], 64)
	if err != nil {
		return 0, err
	}

	var bytes uint64
	u := ss[2]
	if u == "k" {
		bytes = uint64(n * 1024)
	} else if u == "m" {
		bytes = uint64(n * 1024 * 1024)
	} else if u == "g" {
		bytes = uint64(n * 1024 * 1024 * 1024)
	} else {
		return 0, fmt.Errorf("invalid unit: %s", u)
	}

	return bytes, nil
}
