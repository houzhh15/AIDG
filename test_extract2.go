package main

import (
"fmt"
"regexp"
"strings"
)

func extractAttributes(text string) (string, map[string]string) {
	attributePattern := regexp.MustCompile(`(\w+):("[^"]+"|\S+)`)
	attributes := make(map[string]string)
	matches := attributePattern.FindAllStringSubmatch(text, -1)
	fmt.Printf("Text: %q\n", text)
	fmt.Printf("Matches: %v\n", matches)
	cleaned := text
	for _, match := range matches {
		key := strings.ToLower(match[1])
		value := match[2]
		value = strings.Trim(value, "\"")
		attributes[key] = value
		cleaned = strings.ReplaceAll(cleaned, match[0], "")
		fmt.Printf("After removing %q: cleaned=%q\n", match[0], cleaned)
	}
	return strings.TrimSpace(cleaned), attributes
}

func main() {
	text := "修改后的草稿步骤 priority:high"
	desc, attrs := extractAttributes(text)
	fmt.Printf("Final - Description: %q, Attributes: %v\n", desc, attrs)
}
