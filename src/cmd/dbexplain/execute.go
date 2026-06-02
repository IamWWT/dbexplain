// Package main handles the "execute" subcommand for read-only query execution.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/connector"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/config"
	"github.com/IamWWT/dbexplain/internal/dsl"
	"github.com/IamWWT/dbexplain/internal/dsnfilter"
	"github.com/IamWWT/dbexplain/internal/executor"
	"github.com/IamWWT/dbexplain/internal/query"
	"github.com/IamWWT/dbexplain/internal/queryutil"
	"github.com/IamWWT/dbexplain/internal/render"
	"github.com/IamWWT/dbexplain/internal/policy"
)

var queryLock = query.NewQueryLock()

func handleExecute(args []string) {
	fs := flag.NewFlagSet("execute", flag.ExitOnError)
	envMode := fs.Bool("env", false, "Load config from .env file and match by --label/--db")
	label := fs.String("label", "", "Match DSN by label")
	dbIndex := fs.Int("db", 0, "Match DSN by DB<N> index (1-based)")
	dsnFlag := fs.String("dsn", "", "Direct DSN connection string")
	configFile := fs.String("config", "", "JSON config file with array of DSNs")
	limit := fs.Int("limit", 1000, "Max rows to return")
	timeout := fs.Int("timeout", 30, "Query timeout in seconds")
	explain := fs.Bool("explain", false, "Wrap query with EXPLAIN")
	dslMode := fs.Bool("dsl", false, "Enable DSL mode: parse @label.table references")
	human := fs.Bool("human", false, "Human-readable table output (default: JSON)")
	fs.Parse(args)

	// Allow --human after the query for convenience.
	for _, a := range fs.Args() {
		if a == "--human" {
			*human = true
		}
	}

	sqlArg := fs.Arg(0)
	if sqlArg == "" {
		fmt.Fprintln(os.Stderr, "READ_ONLY_VIOLATION: empty query")
		os.Exit(1)
	}

	// Resolve DSNs — gather ALL entries before label filter (needed for JOIN source resolution)
	var allEntries []config.DSNEntry
	if *envMode {
		configPath := config.FindConfigFile()
		if configPath == "" {
			fmt.Fprintln(os.Stderr, "ERROR: no config file found")
			os.Exit(1)
		}
		envEntries, err := config.LoadEnvFile(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: load config %s: %v\n", configPath, config.SanitizeErr(err))
			os.Exit(1)
		}
		allEntries = append(allEntries, envEntries...)
	}
	if *configFile != "" {
		for _, raw := range config.LoadFromConfig(*configFile) {
			allEntries = append(allEntries, config.DSNEntry{Raw: raw})
		}
	}
	if *dsnFlag != "" {
		allEntries = append(allEntries, config.DSNEntry{Raw: *dsnFlag})
	}

	// DSL mode: parse @label.table references and route to the correct backend.
	if *dslMode {
		dslQuery, dslErr := dsl.Parse(sqlArg)
		if dslErr != nil {
			fmt.Fprintln(os.Stderr, dslErr)
			os.Exit(1)
		}
		if dslQuery.HasSourceRefs() {
			handleDSLExecute(dslQuery, allEntries, *human, *limit, *explain, *timeout)
			return
		}
	}

	// Filter by label/dbIndex to select the primary DSN
	entries := dsnfilter.FilterEntries(allEntries, label, dbIndex)
	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "ERROR: no matching DSN found")
		os.Exit(1)
	}
	if len(entries) > 1 {
		msg := fmt.Sprintf("ERROR: %d DSNs matched — use --label to select one:\n", len(entries))
		for _, e := range entries {
			d, err := dsn.ParseDSN(e.Raw)
			if err == nil {
				msg += fmt.Sprintf("  --label %s (%s)\n", d.Label, d.FilePath())
			}
		}
		fmt.Fprint(os.Stderr, msg)
		os.Exit(1)
	}

	parsed, err := dsn.ParseDSN(entries[0].Raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: invalid DSN: %v\n", config.SanitizeErr(err))
		os.Exit(1)
	}

	c, err := connector.GetConnector(parsed.Kind)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	caps := capabilities.FromProvider(c)
	isSQL := caps.Has(capabilities.CapSQL)
	isFile := caps.Has(capabilities.CapFile)

	policies := policy.Load(entries[0].EnvKey)

	// File datasources: bypass sqlguard, enforce policy engine
	if isFile {
		queryutil.HandleFileExecute(parsed, sqlArg, *human, *limit, policies, allEntries)
		return
	}

	// SQL and native execution via shared pipeline
	result, execErr := executor.ExecQuery(&executor.ExecOptions{
		Conn:       c,
		Parsed:     parsed,
		SQL:        sqlArg,
		Limit:      *limit,
		Explain:    *explain,
		TimeoutSec: *timeout,
		Policies:   policies,
		Lock:       queryLock,
		IsSQL:      isSQL,
	})
	if execErr != nil {
		fmt.Fprintln(os.Stderr, execErr)
		os.Exit(1)
	}

	// Output
	if *human {
		fmt.Print(render.FormatHuman(result))
	} else {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: json encode: %v\n", err)
			os.Exit(1)
		}
	}
}

// handleDSLExecute handles the DSL execution path.
func handleDSLExecute(dslQuery *dsl.DSLQuery, allEntries []config.DSNEntry, human bool, limit int, explain bool, timeoutSec int) {
	bound, err := dsl.Bind(dslQuery, allEntries)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	kinds := bound.SourceKinds()
	if len(kinds) == 0 {
		fmt.Fprintln(os.Stderr, "DSL error: no source references resolved")
		os.Exit(1)
	}
	if len(kinds) > 1 {
		fmt.Fprintf(os.Stderr, "DSL error: cross-source queries not supported in v0.1.1 (%d source kinds)\n", len(kinds))
		os.Exit(1)
	}

	switch kinds[0] {
	case dsl.SourceSQL:
		dslExecSQL(dslQuery, bound, human, limit, explain, timeoutSec)
	case dsl.SourceFile:
		dslExecFile(dslQuery, bound, human, limit)
	default:
		fmt.Fprintf(os.Stderr, "DSL error: %s data sources not supported in DSL mode\n", kindName(kinds[0]))
		os.Exit(1)
	}
}

func kindName(k dsl.SourceKind) string {
	switch k {
	case dsl.SourceSQL:
		return "SQL"
	case dsl.SourceFile:
		return "file"
	case dsl.SourceNative:
		return "native"
	default:
		return "unknown"
	}
}

// dslExecSQL compiles and executes a DSL query against an SQL database.
func dslExecSQL(dslQuery *dsl.DSLQuery, bound *dsl.BoundQuery, human bool, limit int, explain bool, timeoutSec int) {
	compiledSQL, err := dsl.CompileToSQL(dslQuery, bound)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	primary := bound.PrimarySource()
	if primary == nil {
		fmt.Fprintln(os.Stderr, "DSL error: no resolved source")
		os.Exit(1)
	}
	parsed := primary.DSN

	c, err := connector.GetConnector(parsed.Kind)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	policies := policy.Load("")

	result, execErr := executor.ExecQuery(&executor.ExecOptions{
		Conn:       c,
		Parsed:     parsed,
		SQL:        compiledSQL,
		Limit:      limit,
		Explain:    explain,
		TimeoutSec: timeoutSec,
		Policies:   policies,
		Lock:       queryLock,
		IsSQL:      true,
	})
	if execErr != nil {
		fmt.Fprintln(os.Stderr, execErr)
		os.Exit(1)
	}

	if human {
		fmt.Print(render.FormatHuman(result))
	} else {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: json encode: %v\n", err)
			os.Exit(1)
		}
	}
}

// dslExecFile executes a DSL query against a file data source (CSV/XLSX).
func dslExecFile(dslQuery *dsl.DSLQuery, bound *dsl.BoundQuery, human bool, limit int) {
	primary := bound.PrimarySource()
	if primary == nil {
		fmt.Fprintln(os.Stderr, "DSL error: no resolved source")
		os.Exit(1)
	}
	parsed := primary.DSN

	policies := policy.Load("")
	queryutil.HandleFileExecute(parsed, dslQuery.SQL, human, limit, policies, nil)
}
