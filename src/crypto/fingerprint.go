// Package crypto provides hardware-bound configuration file encryption.
//
// Two modes are supported:
//   - Machine-only: the decryption key is derived solely from the machine fingerprint.
//     The encrypted file can only be decrypted on the same hardware.
//   - Password: the user supplies a password during encryption. The key is derived
//     via PBKDF2-HMAC-SHA256(password, SHA-256(fingerprint)[:16], 100000).
//     This provides an additional factor for headless/CI environments.
package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrNoHWInfo is returned when no hardware information sources are available.
var ErrNoHWInfo = errors.New("crypto: no hardware information available; fingerprint cannot be computed")

// MachineID computes a deterministic SHA-256 hash of available hardware characteristics.
// The same physical machine always produces the same ID.
// Returns hex-encoded SHA-256 digest (64 hex characters).
func MachineID() (string, error) {
	info := collectHWInfo()
	if len(info) == 0 {
		return "", ErrNoHWInfo
	}
	// Deterministic ordering: sort keys alphabetically, join as k=v\n.
	keys := make([]string, 0, len(info))
	for k := range info {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(info[k])
	}
	h := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(h[:]), nil
}

// VerifyMatch checks whether the current machine fingerprint matches a previously
// stored fingerprint. Returns nil on match, or an error describing the mismatch.
func VerifyMatch(previous string) error {
	current, err := MachineID()
	if err != nil {
		return fmt.Errorf("crypto: cannot compute current fingerprint: %w", err)
	}
	if current != previous {
		return fmt.Errorf("crypto: machine fingerprint mismatch; this file was encrypted on different hardware")
	}
	return nil
}
