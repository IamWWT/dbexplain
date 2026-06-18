// Package list handles the "list" subcommand for displaying configured DSNs.
package list

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/config"
)

// Handle processes the list subcommand.
func Handle(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	dsnFlag := fs.String("dsn", "", "Direct DSN string (repeatable)")
	configFile := fs.String("config", "", "JSON config file path")
	jsonOut := fs.Bool("json", false, "output JSON")
	logDirFlag := fs.String("log-dir", "/var/log/dbexplain", "directory for log files")
	fs.Parse(args)

	var entries []config.DSNEntry
	configPath := ""

	if *configFile != "" {
		for _, raw := range config.LoadFromConfig(*configFile) {
			entries = append(entries, config.DSNEntry{Raw: raw})
		}
	}

	if *dsnFlag != "" {
		entries = append(entries, config.DSNEntry{Raw: *dsnFlag})
	}

	if *configFile == "" && *dsnFlag == "" {
		configPath = config.FindConfigFile()
		if configPath == "" {
			fmt.Fprintln(os.Stderr, "  no config file found. Create .env.dbexplain (or .env.dbexplain.enc) in",
				config.ConfigDirDisplay(), "or current directory.")
			os.Exit(1)
		}
		envEntries, err := config.LoadEnvFile(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: load config %s: %v\n", configPath, config.SanitizeErr(err))
			os.Exit(1)
		}
		entries = append(entries, envEntries...)
	}

	logDir := config.ResolveLogDir(*logDirFlag)

	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "  no DSNs found. Use -dsn or -config.")
		if configPath != "" {
			fmt.Fprintf(os.Stderr, "  Config file %s has no active DSN connections.\n", config.DescribeConfigSource(configPath))
			fmt.Fprintf(os.Stderr, "  Edit this file to add your connections, or copy the template from\n")
			fmt.Fprintf(os.Stderr, "  dbexplain-skill/.env.dbexplain.example\n")
		}
		os.Exit(1)
	}

	// ── JSON output ──
	if *jsonOut {
		type listEntry struct {
			Index    int    `json:"index"`
			Label    string `json:"label"`
			Kind     string `json:"kind"`
			HostPort string `json:"hostPort"`
			Database string `json:"database"`
		}
		var entriesList []listEntry
		for _, e := range entries {
			parsed, err := dsn.ParseDSN(e.Raw)
			if err != nil {
				continue
			}
			label := parsed.Label
			if label == "" {
				label = "(no label)"
			}
			hostPort := parsed.Host
			if parsed.Port != "" {
				hostPort += ":" + parsed.Port
			}
			dbName := parsed.DBName
			if dbName == "" {
				dbName = "(n/a)"
			}
			entriesList = append(entriesList, listEntry{
				Index: len(entriesList) + 1,
				Label: label, Kind: parsed.Kind,
				HostPort: hostPort, Database: dbName,
			})
		}
		out := struct {
			LogDirectory string      `json:"logDirectory"`
			Entries      []listEntry `json:"entries"`
		}{
			LogDirectory: logDir,
			Entries:      entriesList,
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: json encode: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
		return
	}

	// ── Table output ──
	fmt.Println()
	if configPath != "" {
		fmt.Printf("  Config source: %s\n", config.DescribeConfigSource(configPath))
	}
	fmt.Printf("  Log directory: %s\n\n", logDir)
	fmt.Println("  Available databases:")
	fmt.Println()

	fmt.Printf("  %-6s %-22s %-17s %-24s %s\n", "INDEX", "LABEL", "KIND", "HOST:PORT", "DATABASE")
	fmt.Println("  " + strings.Repeat("─", 90))

	for i, e := range entries {
		parsed, err := dsn.ParseDSN(e.Raw)
		if err != nil {
			continue
		}
		label := parsed.Label
		if label == "" {
			label = "(no label)"
		}
		hostPort := parsed.Host
		if parsed.Port != "" {
			hostPort += ":" + parsed.Port
		}
		dbName := parsed.DBName
		if dbName == "" {
			dbName = "(n/a)"
		}
		fmt.Printf("  %-6d %-22s %-17s %-24s %s\n", i+1, label, parsed.Kind, hostPort, dbName)
	}
	fmt.Println()
	fmt.Println("  Use --db <INDEX> or --label <LABEL> with execute subcommand.")
	fmt.Println()
}
