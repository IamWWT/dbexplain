// Package config provides configuration file discovery, DSN loading,
// and environment management for dbexplain.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/joho/godotenv"

	"github.com/IamWWT/dbexplain/internal/crypto"
	"github.com/IamWWT/dbexplain/internal/dsn"
)

// DSNEntry represents a parsed DSN entry from command-line flags or config files.
type DSNEntry struct {
	Raw    string // DSN string
	EnvKey string // e.g. "DB1" if from .env, "" otherwise
}

type envEntry struct {
	idx int
	key string
	val string
}

// ── Config file search ──

// findConfigFile searches for config files by priority.
// Priority: DBPROBE_ENV_FILE > .env.dbexplain (CWD) > .env.dbexplain.enc (CWD) >
// XDG/user config > user config .enc > .env (CWD, legacy) > .env.enc (CWD)
func FindConfigFile() string {
	// 1. DBPROBE_ENV_FILE env var (explicit override)
	if envFile := os.Getenv("DBPROBE_ENV_FILE"); envFile != "" {
		if _, err := os.Stat(envFile); err == nil {
			return envFile
		}
	}
	// 2. .env.dbexplain (CWD)
	if _, err := os.Stat(".env.dbexplain"); err == nil {
		return ".env.dbexplain"
	}
	// 2b. .env.dbexplain.enc (CWD) — encrypted variant
	if _, err := os.Stat(".env.dbexplain.enc"); err == nil {
		return ".env.dbexplain.enc"
	}
	// 3. User config dir — plaintext
	path := userConfigPath()
	if _, err := os.Stat(path); err == nil {
		return path
	}
	// 3b. User config dir — encrypted variant
	if encPath := userConfigEncPath(); encPath != "" {
		if _, err := os.Stat(encPath); err == nil {
			return encPath
		}
	}
	// 4. .env (CWD, legacy)
	if _, err := os.Stat(".env"); err == nil {
		return ".env"
	}
	// 4b. .env.enc (CWD) — encrypted legacy variant
	if _, err := os.Stat(".env.enc"); err == nil {
		return ".env.enc"
	}
	return ""
}

// ConfigDirDisplay returns a human-readable config directory path for messages.
func ConfigDirDisplay() string {
	if runtime.GOOS == "windows" {
		return `%USERPROFILE%\.config\dbexplain\`
	}
	return "~/.config/dbexplain/"
}

// configDirPath returns the actual filesystem path to the config directory.
func configDirPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".config", "dbexplain")
}

// userConfigPath returns the config file path in the user config directory.
func userConfigPath() string {
	dir := configDirPath()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, ".env.dbexplain")
}

// userConfigEncPath returns the encrypted config file path in the user config directory.
func userConfigEncPath() string {
	dir := configDirPath()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, ".env.dbexplain.enc")
}

// EncryptionKeyPath returns the encryption key file path.
func EncryptionKeyPath() string {
	dir := configDirPath()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, ".encryption_key")
}

// ResolveLogDir creates the log directory, falling back if the primary path is not writable.
func ResolveLogDir(requested string) string {
	if err := os.MkdirAll(requested, 0755); err == nil {
		return requested
	}

	candidates := []string{}
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		candidates = append(candidates, filepath.Join(d, "dbexplain", "logs"))
	}
	if homeDir, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(homeDir, ".local", "state", "dbexplain", "logs"))
	}
	candidates = append(candidates, filepath.Join(os.TempDir(), "dbexplain", "logs"))

	for _, dir := range candidates {
		if err := os.MkdirAll(dir, 0755); err == nil {
			log.Printf("log directory: %s (requested %s not writable)", dir, requested)
			return dir
		}
	}

	log.Printf("no writable log directory found; logging to stderr only")
	return os.TempDir()
}

// ReadEncryptionKey returns the password for decrypting password-mode .enc files.
func ReadEncryptionKey() string {
	if key := os.Getenv("APP_ENCRYPTION_KEY"); key != "" {
		return key
	}
	keyPath := EncryptionKeyPath()
	if keyPath != "" {
		data, err := os.ReadFile(keyPath)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

// ── DSN Loading ──

// LoadEnvFile reads and parses an env file, stripping UTF-8 BOM if present.
// Non-DSN entries (policy vars like DENY_TABLES, MASK_COLUMNS) are set as OS env vars
// so policy.Load() can read them via os.Getenv.
// DSN entries (DB1=..., DB2=...) are returned directly to avoid leaking passwords
// into the process environment.
func LoadEnvFile(path string) ([]DSNEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Strip UTF-8 BOM
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	// Auto-detect encrypted config file
	if len(data) > 0 && (data[0] == crypto.ModeMachine || data[0] == crypto.ModePassword) {
		machineID, err := crypto.MachineID()
		if err != nil {
			return nil, fmt.Errorf("compute machine fingerprint for config %s: %w", path, err)
		}
		password := ReadEncryptionKey()
		os.Unsetenv("APP_ENCRYPTION_KEY")
		plaintext, err := crypto.DecryptBytes(data, machineID, password)
		if err != nil {
			return nil, fmt.Errorf("decrypt config %s: %w", path, SanitizeErr(err))
		}
		data = plaintext
	}

	envMap, err := godotenv.UnmarshalBytes(data)
	if err != nil {
		return nil, err
	}
	var entries []envEntry
	for k, v := range envMap {
		if strings.HasPrefix(k, "DB") {
			numStr := k[2:]
			if idx, err := strconv.Atoi(numStr); err == nil && idx > 0 {
				entries = append(entries, envEntry{idx, k, v})
				continue
			}
		}
		os.Setenv(k, v)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].idx < entries[j].idx
	})
	var result []DSNEntry
	for _, e := range entries {
		result = append(result, DSNEntry{Raw: e.val, EnvKey: e.key})
	}
	return result, nil
}

// SanitizeErr redacts passwords from error messages to prevent credential leaks.
func SanitizeErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	for {
		protoIdx := strings.Index(msg, "://")
		if protoIdx < 0 {
			break
		}
		userStart := protoIdx + 3
		colonIdx := strings.Index(msg[userStart:], ":")
		if colonIdx < 0 {
			break
		}
		passStart := userStart + colonIdx + 1
		atIdx := strings.Index(msg[passStart:], "@")
		if atIdx < 0 {
			break
		}
		msg = msg[:passStart] + "***" + msg[passStart+atIdx:]
	}
	return fmt.Errorf("%s", msg)
}

// LoadFromConfig reads a JSON config file containing an array of DSN strings.
func LoadFromConfig(filename string) []string {
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("read config: %v", err)
	}
	var dsnList []string
	if err := json.Unmarshal(data, &dsnList); err != nil {
		log.Fatalf("parse config: %v", err)
	}
	return dsnList
}

// ── DSN filtering ──

// FilterDSNs filters DSN entries by include/exclude patterns.
func FilterDSNs(entries []DSNEntry, include, exclude string, logDir string) []DSNEntry {
	if include == "" && exclude == "" {
		return entries
	}

	includeSet := parseFilterSet(include)
	excludeSet := parseFilterSet(exclude)

	filterLogFile, err := os.OpenFile(filepath.Join(logDir, "filter.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	var filterLogger *log.Logger
	if err != nil {
		filterLogger = log.Default()
	} else {
		defer filterLogFile.Close()
		filterLogger = log.New(filterLogFile, "", log.LstdFlags)
	}

	var filtered []DSNEntry
	for _, e := range entries {
		parsed, err := dsn.ParseDSN(e.Raw)
		if err != nil {
			filtered = append(filtered, e)
			continue
		}

		if len(includeSet) > 0 && matchesDSNFilter(parsed, e.EnvKey, includeSet) {
			filtered = append(filtered, e)
			continue
		}

		if len(includeSet) > 0 {
			filterLogger.Printf("skipping %s (did not match include filter)", parsed.Redacted())
			continue
		}

		if len(excludeSet) > 0 && matchesDSNFilter(parsed, e.EnvKey, excludeSet) {
			filterLogger.Printf("excluding %s (matched exclude filter)", parsed.Redacted())
			continue
		}

		filtered = append(filtered, e)
	}
	return filtered
}

func parseFilterSet(csv string) map[string]bool {
	set := make(map[string]bool)
	if csv == "" {
		return set
	}
	for _, item := range strings.Split(csv, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			set[strings.ToLower(item)] = true
		}
	}
	return set
}

func matchesDSNFilter(d *dsn.DSN, envKey string, filterSet map[string]bool) bool {
	if filterSet[strings.ToLower(d.Kind)] {
		return true
	}
	if filterSet[strings.ToLower(d.Label)] {
		return true
	}
	if envKey != "" && filterSet[strings.ToLower(envKey)] {
		return true
	}
	return false
}

// PrintDSNMapping prints a summary of DSN entries to stderr.
func PrintDSNMapping(entries []DSNEntry) {
	hasEnvKeys := false
	for _, e := range entries {
		if e.EnvKey != "" {
			hasEnvKeys = true
			break
		}
	}
	if !hasEnvKeys {
		return
	}

	fmt.Fprintf(os.Stderr, "\n> DSN mapping:\n")
	for _, e := range entries {
		parsed, err := dsn.ParseDSN(e.Raw)
		if err != nil {
			continue
		}
		key := e.EnvKey
		if key == "" {
			key = "—"
		}
		label := parsed.Label
		if label == "" {
			label = "(no label)"
		}
		fmt.Fprintf(os.Stderr, "  %-4s → %-20s  %s\n", key, label, parsed.Redacted())
	}
	fmt.Fprintln(os.Stderr)
}

