package connector

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/IamWWT/dbexplain/capabilities"
	"github.com/IamWWT/dbexplain/dsn"
	"github.com/IamWWT/dbexplain/query"
	"github.com/IamWWT/dbexplain/schema"
	"github.com/xuri/excelize/v2"
)

func init() {
	Register("xlsx", func() Connector { return xlsxConnector{} })
}

type xlsxConnector struct{}

func (xlsxConnector) Capabilities() []capabilities.Capability {
	return []capabilities.Capability{capabilities.CapRowCount}
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
		return nil, fmt.Errorf("xlsx open %q: %w", path, err)
	}
	defer f.Close()
	return f.GetSheetList(), nil
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
	limit, offset := xlsxParseSelectStar(opts.SQL)
	if limit < 0 {
		return nil, &query.ErrNotSupported{Kind: "xlsx"}
	}
	if limit <= 0 {
		limit = opts.MaxRows
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
