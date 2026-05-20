//go:build windows

package main

import (
	"io"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// encodeOutput converts a UTF-8 string to the system-appropriate encoding.
// On Chinese Windows (ACP 936): converts to GBK so CMD `type` displays correctly.
// On other locales: UTF-8 with BOM (compatible with Notepad and modern editors).
func encodeOutput(s string) ([]byte, error) {
	acp := windows.GetACP()
	if acp == 936 {
		// Chinese (GBK) — CMD code page matches, `type` works
		r := transform.NewReader(strings.NewReader(s), simplifiedchinese.GBK.NewEncoder())
		return io.ReadAll(r)
	}
	// Other locales — UTF-8 is the universal standard
	return append([]byte("\xEF\xBB\xBF"), []byte(s)...), nil
}
