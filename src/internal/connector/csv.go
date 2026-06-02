package connector

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/connector/filequery"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/query"
	"github.com/IamWWT/dbexplain/internal/schema"
)

func init() {
	Register("csv", func() Connector { return csvConnector{} })
}

type csvConnector struct{}

func (csvConnector) Capabilities() []capabilities.Capability {
	return []capabilities.Capability{capabilities.CapRowCount, capabilities.CapFile}
}

// Collect reads CSV/TSV files and returns schema metadata.
func (csvConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	path := d.FilePath()
	delimiter := getDelimiter(d)
	encoding := d.DSNParam("encoding")
	if encoding == "" {
		encoding = "utf-8"
	}

	var files []string
	switch {
	case hasGlobMeta(path):
		// Explicit glob expression
		matches, err := filepath.Glob(path)
		if err != nil {
			return nil, fmt.Errorf("csv glob %q: %w", path, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("csv glob %q: no matching files", path)
		}
		files = matches
	default:
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("csv path %q: %w", path, err)
		}
		if info.IsDir() {
			// Directory — scan for csv/tsv files
			extPattern := "*.csv"
			if delimiter == '\t' {
				extPattern = "*.tsv"
				// Also check *.csv in TSV mode
				matches, _ := filepath.Glob(filepath.Join(path, "*.csv"))
				files = append(files, matches...)
			}
			matches, err := filepath.Glob(filepath.Join(path, extPattern))
			if err != nil {
				return nil, fmt.Errorf("csv dir %q: %w", path, err)
			}
			files = append(files, matches...)
			if len(files) == 0 {
				return nil, fmt.Errorf("csv dir %q: no CSV files found", path)
			}
		} else {
			files = append(files, path)
		}
	}

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "csv", Label: d.Label}
	db := &schema.Database{Name: d.Label}

	for _, f := range files {
		tbl, err := readCSVFile(f, delimiter, encoding)
		if err != nil {
			logf(ctx, "[csv] skip %s: %v", f, err)
			continue
		}
		db.Tables = append(db.Tables, tbl)
	}

	if len(db.Tables) == 0 {
		return nil, fmt.Errorf("csv: no valid tables found in %q", path)
	}

	inst.Databases = append(inst.Databases, db)
	return inst, nil
}

// ExecQuery implements query.Queryable for CSV files.
// Supports SELECT * [LIMIT N [OFFSET M]] (fast path) and
// SELECT col, ... WHERE ... GROUP BY ... ORDER BY ... JOIN ... (filequery engine).
func (csvConnector) ExecQuery(ctx context.Context, opts query.ExecuteOpts) (*query.QueryResult, error) {
	path := opts.DSN.FilePath()
	delimiter := getDelimiter(opts.DSN)
	encoding := opts.DSN.DSNParam("encoding")
	if encoding == "" {
		encoding = "utf-8"
	}

	// Fast path: SELECT * [LIMIT N [OFFSET M]]
	limit, offset := parseSelectStar(opts.SQL)
	if limit >= 0 {
		return execCSVSelectStar(path, delimiter, encoding, limit, offset, opts.MaxRows)
	}

	// New path: filequery engine for all other SQL (WHERE, GROUP BY, ORDER BY, JOIN, etc.)
	files, err := resolveCSVFiles(path)
	if err != nil {
		return nil, fmt.Errorf("csv: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("csv: no matching files for %q", path)
	}

	// Read data from all files
	var allData [][]string
	var columns []string
	skipRows := 0
	for _, f := range files {
		data, cols, err := readCSVData(f, delimiter, encoding)
		if err != nil {
			return nil, fmt.Errorf("csv: %w", err)
		}
		if columns == nil {
			columns = cols
		}
		// Skip header from each file after the first one
		if skipRows == 0 {
			data = data[1:] // skip header
			skipRows = 1
		} else {
			data = data[1:]
		}
		allData = append(allData, data...)
	}

	if columns == nil {
		return nil, fmt.Errorf("csv: empty result")
	}

	// Load extra tables for JOIN (if any)
	extras := make([]filequery.NamedData, 0, len(opts.ExtraTables))
	for _, et := range opts.ExtraTables {
		extras = append(extras, filequery.NamedData{
			Alias:  et.Alias,
			Header: et.Header,
			Rows:   et.Rows,
		})
	}
	// Execute via filequery engine
	result, err := filequery.Execute(opts.SQL, columns, allData, extras, opts.MaxRows)
	if err != nil {
		return nil, fmt.Errorf("csv query error: %w", err)
	}
	return result, nil
}

// execCSVSelectStar handles the fast SELECT * path for CSV files.
func execCSVSelectStar(path string, delimiter rune, encoding string, limit, offset, maxRows int) (*query.QueryResult, error) {
	// Use MaxRows if no explicit LIMIT
	if limit <= 0 {
		limit = maxRows
	}

	files, err := resolveCSVFiles(path)
	if err != nil {
		return nil, fmt.Errorf("csv: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("csv: no matching files for %q", path)
	}

	// Read data from all files
	var allData [][]string
	var columns []string
	skipRows := 0
	for _, f := range files {
		data, cols, err := readCSVData(f, delimiter, encoding)
		if err != nil {
			return nil, fmt.Errorf("csv: %w", err)
		}
		if columns == nil {
			columns = cols
		}
		// Skip header from each file after the first one
		if skipRows == 0 {
			data = data[1:] // skip header
			skipRows = 1
		} else {
			data = data[1:]
		}
		allData = append(allData, data...)
	}

	if columns == nil {
		return nil, fmt.Errorf("csv: empty result")
	}

	// Apply LIMIT/OFFSET
	totalRows := len(allData)
	start := offset
	if start > totalRows {
		start = totalRows
	}
	end := start + limit
	if end > totalRows {
		end = totalRows
	}
	rows := allData[start:end]

	// Build QueryResult
	result := &query.QueryResult{
		Columns:  make([]query.ColumnInfo, len(columns)),
		Rows:     make([][]*string, len(rows)),
		RowCount: len(rows),
	}
	for i, col := range columns {
		result.Columns[i] = query.ColumnInfo{Name: col, Type: "TEXT"}
	}
	for i, row := range rows {
		result.Rows[i] = make([]*string, len(row))
		for j, val := range row {
			v := val
			result.Rows[i][j] = &v
		}
	}

	return result, nil
}

// --- internal helpers ---

func getDelimiter(d *dsn.DSN) rune {
	delim := d.DSNParam("delimiter")
	switch strings.ToLower(delim) {
	case "tab", "\\t", "%09", "\t":
		return '\t'
	case "pipe", "|":
		return '|'
	case "semicolon", ";":
		return ';'
	default:
		// Check if dsn scheme is tsv
		if strings.HasPrefix(d.Raw, "tsv://") {
			return '\t'
		}
		return ','
	}
}

// hasGlobMeta checks if a path contains glob metacharacters.
func hasGlobMeta(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

// resolveCSVFiles resolves a path (file, dir, or glob) to a list of files.
func resolveCSVFiles(path string) ([]string, error) {
	switch {
	case hasGlobMeta(path):
		matches, err := filepath.Glob(path)
		if err != nil {
			return nil, fmt.Errorf("glob %q: %w", path, err)
		}
		return matches, nil
	default:
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			matches, _ := filepath.Glob(filepath.Join(path, "*.csv"))
			tsvMatches, _ := filepath.Glob(filepath.Join(path, "*.tsv"))
			matches = append(matches, tsvMatches...)
			return matches, nil
		}
		return []string{path}, nil
	}
}

// readCSVFile reads a CSV file and returns a schema.Table.
func readCSVFile(path string, delimiter rune, encoding string) (*schema.Table, error) {
	data, cols, err := readCSVData(path, delimiter, encoding)
	if err != nil {
		return nil, err
	}
	if len(data) < 2 {
		// Only header, no data rows
		tbl := &schema.Table{
			Name:     tableName(path),
			RowCount: 0,
		}
		for _, c := range cols {
			tbl.Columns = append(tbl.Columns, &schema.Column{Name: c, Type: "TEXT"})
		}
		return tbl, nil
	}

	body := data[1:] // skip header
	tbl := &schema.Table{
		Name:     tableName(path),
		RowCount: int64(len(body)),
	}

	// Sample first 100 rows for type inference
	sampleLimit := 100
	if len(body) < sampleLimit {
		sampleLimit = len(body)
	}

	for colIdx, colName := range cols {
		samples := make([]string, 0, sampleLimit)
		for _, row := range body[:sampleLimit] {
			if colIdx < len(row) {
				samples = append(samples, row[colIdx])
			}
		}
		colType := inferColumnType(samples)
		tbl.Columns = append(tbl.Columns, &schema.Column{
			Name: colName,
			Type: colType,
		})
	}

	return tbl, nil
}

// readCSVData reads all records from a CSV file.
func readCSVData(path string, delimiter rune, encoding string) ([][]string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("file not found: %s (use absolute path)", path)
		}
		return nil, nil, err
	}
	defer f.Close()

	var reader io.Reader = f
	if encoding != "utf-8" && encoding != "utf8" {
		switch encoding {
		case "gbk", "gb2312", "gb18030":
			reader = transform.NewReader(f, simplifiedchinese.GBK.NewDecoder())
		default:
			return nil, nil, fmt.Errorf("unsupported encoding %q", encoding)
		}
	}

	csvReader := csv.NewReader(bufio.NewReader(reader))
	csvReader.Comma = delimiter
	csvReader.LazyQuotes = true
	csvReader.FieldsPerRecord = -1 // variable columns

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("csv read %q: %w", path, err)
	}
	if len(records) == 0 {
		return nil, nil, fmt.Errorf("empty CSV file %q", path)
	}

	// Strip UTF-8 BOM (EF BB BF) from first column header
	if len(records[0]) > 0 && len(records[0][0]) >= 3 &&
		records[0][0][0] == 0xEF && records[0][0][1] == 0xBB && records[0][0][2] == 0xBF {
		records[0][0] = records[0][0][3:]
		for i := 1; i < len(records); i++ {
			if len(records[i]) > 0 && len(records[i][0]) >= 3 &&
				records[i][0][0] == 0xEF && records[i][0][1] == 0xBB && records[i][0][2] == 0xBF {
				records[i][0] = records[i][0][3:]
			}
		}
	}

	return records, records[0], nil
}

// ReadCSVFile is an exported wrapper for readCSVData, used by execute.go for JOIN source loading.
// Returns (allRecords, header, error) where allRecords[0] is the first data row (after header).
func ReadCSVFile(path string, delimiter rune, encoding string) ([][]string, []string, error) {
	records, header, err := readCSVData(path, delimiter, encoding)
	if err != nil {
		return nil, nil, err
	}
	// Skip header row — records[0] is the header
	if len(records) > 1 {
		return records[1:], header, nil
	}
	return nil, header, nil
}

// GetDelimiter is an exported wrapper for getDelimiter, used by execute.go for JOIN source loading.
func GetDelimiter(d *dsn.DSN) rune {
	return getDelimiter(d)
}

// tableName extracts a table name from a file path (filename without extension).
func tableName(path string) string {
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	return name[:len(name)-len(ext)]
}

// selectStarRE matches: SELECT * [FROM table] [LIMIT N] [OFFSET M]
var selectStarRE = regexp.MustCompile(`(?i)^\s*SELECT\s+\*\s*(?:\s+FROM\s+(\S+))?\s*(?:\s+LIMIT\s+(\d+))?\s*(?:\s+OFFSET\s+(\d+))?\s*$`)

// parseSelectStar parses LIMIT/OFFSET from a SELECT * query.
// Returns (-1, 0) if the query is not a supported SELECT * pattern.
func parseSelectStar(sql string) (limit int, offset int) {
	matches := selectStarRE.FindStringSubmatch(sql)
	if matches == nil {
		return -1, 0
	}
	limit = 0
	offset = 0
	if matches[2] != "" {
		limit, _ = strconv.Atoi(matches[2])
	}
	if matches[3] != "" {
		offset, _ = strconv.Atoi(matches[3])
	}
	return limit, offset
}
