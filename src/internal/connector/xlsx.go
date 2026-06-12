//go:build xlsx || full

package connector

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/IamWWT/dbexplain/internal/capabilities"
	"github.com/IamWWT/dbexplain/internal/connector/filequery"
	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/query"
	"github.com/IamWWT/dbexplain/internal/schema"
	"github.com/xuri/excelize/v2"
)

func init() {
	Register("xlsx", func() Connector { return xlsxConnector{} })
}

type xlsxConnector struct{}

func (xlsxConnector) Capabilities() []capabilities.Capability {
	return []capabilities.Capability{capabilities.CapRowCount, capabilities.CapFile}
}

// ---- excelize helpers ---- //

// xlsxSheet represents a single sheet's data.
type xlsxSheet struct {
	Name     string
	Columns  []string
	Rows     [][]string
	RowCount int
}

// xlsxCollectSheets opens an Excel file and returns all sheets.
func xlsxCollectSheets(path string) ([]xlsxSheet, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("file not found: %s (use absolute path)", path)
		}
		return nil, fmt.Errorf("xlsx open %q: %w", path, err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	result := make([]xlsxSheet, 0, len(sheets))

	for _, sheetName := range sheets {
		rows, err := f.GetRows(sheetName)
		if err != nil {
			return nil, fmt.Errorf("xlsx read sheet %q: %w", sheetName, err)
		}
		if len(rows) == 0 {
			continue
		}

		s := xlsxSheet{
			Name:    sheetName,
			Columns: rows[0],
			Rows:    rows[1:],
		}
		s.RowCount = len(s.Rows)
		if s.RowCount > 0 {
			result = append(result, s)
		}
	}

	return result, nil
}

// xlsxQueryResult is a pure data query result.
type xlsxQueryResult struct {
	Columns  []string
	Rows     [][]string
	RowCount int
}

// xlsxQuerySheet performs SELECT * equivalent on a single sheet with LIMIT/OFFSET.
func xlsxQuerySheet(path, sheetName string, limit, offset int) (*xlsxQueryResult, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("file not found: %s (use absolute path)", path)
		}
		return nil, fmt.Errorf("xlsx open %q: %w", path, err)
	}
	defer f.Close()

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("xlsx read sheet %q: %w", sheetName, err)
	}
	if len(rows) == 0 {
		return &xlsxQueryResult{}, nil
	}

	columns := rows[0]
	data := rows[1:]

	totalRows := len(data)
	start := offset
	if start > totalRows {
		start = totalRows
	}
	end := start + limit
	if end > totalRows {
		end = totalRows
	}

	return &xlsxQueryResult{
		Columns:  columns,
		Rows:     data[start:end],
		RowCount: end - start,
	}, nil
}

// xlsxGetSheetNames returns the list of sheet names in an Excel file.
func xlsxGetSheetNames(path string) ([]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("file not found: %s (use absolute path)", path)
		}
		return nil, fmt.Errorf("xlsx open %q: %w", path, err)
	}
	defer f.Close()
	return f.GetSheetList(), nil
}

// XLSXSheet holds a single sheet's data for multi-sheet loading.
type XLSXSheet struct {
	Alias  string
	Header []string
	Rows   [][]string
}

// ReadXLSXFile is an exported wrapper for reading XLSX data, used by execute.go for JOIN source loading.
// Returns (rows, header, error) — rows exclude the header row.
// Deprecated: Use ReadXLSXSheets for multi-sheet support.
func ReadXLSXFile(path string) ([]string, [][]string, error) {
	sheets, err := xlsxCollectSheets(path)
	if err != nil {
		return nil, nil, err
	}
	if len(sheets) == 0 {
		return nil, nil, fmt.Errorf("xlsx: no sheets found in %q", path)
	}
	return sheets[0].Columns, sheets[0].Rows, nil
}

// ReadXLSXSheets returns all sheets from an XLSX file, each as a separate data source.
func ReadXLSXSheets(path string) ([]XLSXSheet, error) {
	sheets, err := xlsxCollectSheets(path)
	if err != nil {
		return nil, err
	}
	result := make([]XLSXSheet, 0, len(sheets))
	for _, s := range sheets {
		result = append(result, XLSXSheet{
			Alias:  s.Name,
			Header: s.Columns,
			Rows:   s.Rows,
		})
	}
	return result, nil
}

// ---- Connector interface implementation ---- //

func (xlsxConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	path := d.FilePath()

	sheets, err := xlsxCollectSheets(path)
	if err != nil {
		return nil, err
	}
	if len(sheets) == 0 {
		return nil, fmt.Errorf("xlsx: no sheets found in %q", path)
	}

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "xlsx", Label: d.Label}
	db := &schema.Database{Name: d.Label}

	for _, s := range sheets {
		tbl := &schema.Table{
			Name:     s.Name,
			RowCount: int64(s.RowCount),
		}

		sampleLimit := 100
		if s.RowCount < sampleLimit {
			sampleLimit = s.RowCount
		}

		for colIdx, colName := range s.Columns {
			col := &schema.Column{Name: colName, Type: "TEXT"}
			if s.RowCount > 0 {
				samples := make([]string, 0, sampleLimit)
				for _, row := range s.Rows[:sampleLimit] {
					if colIdx < len(row) {
						samples = append(samples, row[colIdx])
					}
				}
				col.Type = inferColumnType(samples)
			}
			tbl.Columns = append(tbl.Columns, col)
		}

		db.Tables = append(db.Tables, tbl)
	}

	inst.Databases = append(inst.Databases, db)
	return inst, nil
}

func (xlsxConnector) ExecQuery(ctx context.Context, opts query.ExecuteOpts) (*query.QueryResult, error) {
	path := opts.DSN.FilePath()

	// Fast path: SELECT * [LIMIT N [OFFSET M]]
	limit, offset := xlsxParseSelectStar(opts.SQL)
	if limit >= 0 {
		return execXLSXSelectStar(path, limit, offset, opts.MaxRows)
	}

	// Filequery engine path
	sheets, err := xlsxCollectSheets(path)
	if err != nil {
		return nil, fmt.Errorf("xlsx: %w", err)
	}
	if len(sheets) == 0 {
		return nil, fmt.Errorf("xlsx: no sheets in %q", path)
	}

	// Find the sheet matching the FROM table name in SQL.
	// If no match, use the first sheet as fallback.
	var columns []string
	var allData [][]string
	var primarySheetName string
	if tableName := extractTableName(opts.SQL); tableName != "" {
		for _, s := range sheets {
			if strings.EqualFold(s.Name, tableName) {
				columns = s.Columns
				allData = s.Rows
				primarySheetName = s.Name
				break
			}
		}
	}
	if columns == nil {
		columns = sheets[0].Columns
		allData = sheets[0].Rows
		primarySheetName = sheets[0].Name
	}

	// Load extra tables for JOIN:
	// 1) All non-primary sheets from the same xlsx (intra-xlsx JOIN, e.g. Sheet1 JOIN Sheet2)
	// 2) Any cross-DSN extras from opts.ExtraTables (cross-format JOIN, e.g. xlsx JOIN csv)
	extras := make([]filequery.NamedData, 0)
	for _, s := range sheets {
		if s.Name != primarySheetName {
			extras = append(extras, filequery.NamedData{
				Alias:  s.Name,
				Header: s.Columns,
				Rows:   s.Rows,
			})
		}
	}
	// Append cross-DSN extras (loaded by resolveJoinSources in execute.go)
	for _, et := range opts.ExtraTables {
		// Avoid alias collision with auto-loaded sheets
		alreadyLoaded := false
		for _, e := range extras {
			if e.Alias == et.Alias {
				alreadyLoaded = true
				break
			}
		}
		if !alreadyLoaded {
			extras = append(extras, filequery.NamedData{
				Alias:  et.Alias,
				Header: et.Header,
				Rows:   et.Rows,
			})
		}
	}

	result, err := filequery.Execute(opts.SQL, columns, allData, extras, opts.MaxRows)
	if err != nil {
		log.Printf("[xlsx] filequery error: %v", err)
		return nil, fmt.Errorf("xlsx filequery: %w", err)
	}
	return result, nil
}

// execXLSXSelectStar handles the fast SELECT * path for XLSX files.
func execXLSXSelectStar(path string, limit, offset, maxRows int) (*query.QueryResult, error) {
	if limit <= 0 {
		limit = maxRows
	}

	sheetNames, err := xlsxGetSheetNames(path)
	if err != nil {
		return nil, fmt.Errorf("xlsx: %w", err)
	}
	if len(sheetNames) == 0 {
		return nil, fmt.Errorf("xlsx: no sheets in %q", path)
	}

	qr, err := xlsxQuerySheet(path, sheetNames[0], limit, offset)
	if err != nil {
		return nil, fmt.Errorf("xlsx: %w", err)
	}

	result := &query.QueryResult{
		Columns:  make([]query.ColumnInfo, len(qr.Columns)),
		Rows:     make([][]*string, qr.RowCount),
		RowCount: qr.RowCount,
	}
	for i, col := range qr.Columns {
		result.Columns[i] = query.ColumnInfo{Name: col, Type: "TEXT"}
	}
	for i, row := range qr.Rows {
		result.Rows[i] = make([]*string, len(row))
		for j, val := range row {
			v := val
			result.Rows[i][j] = &v
		}
	}

	return result, nil
}

// xlsxParseSelectStar parses SELECT * [LIMIT N [OFFSET M]] for xlsx queries.
var xlsxSelectStarRE = regexp.MustCompile(`(?i)^\s*SELECT\s+\*\s*(?:\s+FROM\s+(\S+))?\s*(?:\s+LIMIT\s+(\d+))?\s*(?:\s+OFFSET\s+(\d+))?\s*$`)

func xlsxParseSelectStar(sql string) (limit int, offset int) {
	matches := xlsxSelectStarRE.FindStringSubmatch(sql)
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

// extractTableName extracts the FROM table name from a SQL query.
// Returns empty string if not found.
func extractTableName(sql string) string {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	// Find FROM keyword
	fromIdx := strings.Index(upper, "FROM")
	if fromIdx < 0 {
		return ""
	}
	// Skip past "FROM "
	rest := strings.TrimSpace(sql[fromIdx+4:])
	if rest == "" {
		return ""
	}
	// Table name is the first word
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return ""
	}
	name := parts[0]
	// Remove any leading/trailing quotes
	name = strings.Trim(name, "\"'`")
	return name
}
