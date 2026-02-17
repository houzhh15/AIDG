package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// printOutput 按指定格式输出响应数据
func printOutput(format string, data []byte) error {
	if format == "json" {
		var out bytes.Buffer
		if err := json.Indent(&out, data, "", "  "); err != nil {
			// 非 JSON 数据直接输出
			fmt.Println(string(data))
			return nil
		}
		fmt.Println(out.String())
		return nil
	}
	// text 模式：直接输出
	fmt.Println(string(data))
	return nil
}

// exitError 输出错误到 stderr 并退出
func exitError(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+msg+"\n", args...)
	os.Exit(1)
}
