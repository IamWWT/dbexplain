// Package main provides platform-specific output encoding.
//
// On Windows, the file output encoding is chosen based on the system's
// active ANSI code page (ACP):
//   - ACP 936 (Simplified Chinese GBK): convert to GBK for CMD `type` compatibility
//   - Other: UTF-8 with BOM (works in Notepad, VS Code, all modern editors)

//go:build !windows

package main

// encodeOutput converts the UTF-8 string to the platform-appropriate encoding.
// On Unix/macOS: UTF-8 with BOM for maximum editor compatibility.
// On Windows: see encode_windows.go for code-page-aware behavior.
func encodeOutput(s string) ([]byte, error) {
	return append([]byte("\xEF\xBB\xBF"), []byte(s)...), nil
}
