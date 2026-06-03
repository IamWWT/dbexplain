//go:build !duckdb

package manual

func init() {
	duckdbHelp = "  duckdb (not included; build from source with -tags duckdb)\n"
}
