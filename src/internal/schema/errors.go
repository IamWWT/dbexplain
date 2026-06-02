package schema

import "fmt"

// DBError 携带数据库操作的详细上下文
type DBError struct {
	DSN   string // 脱敏后的 DSN
	DB    string // 数据库名
	Table string // 表名
	Op    string // 操作描述
	Err   error  // 底层错误
}

func (e *DBError) Error() string {
	loc := e.DSN
	if e.DB != "" {
		loc += "/" + e.DB
	}
	if e.Table != "" {
		loc += "." + e.Table
	}
	return fmt.Sprintf("[%s] %s: %v", loc, e.Op, e.Err)
}

func (e *DBError) Unwrap() error { return e.Err }

// NewDBError 快捷构造
func NewDBError(dsn, db, table, op string, err error) error {
	if err == nil {
		return nil
	}
	return &DBError{DSN: dsn, DB: db, Table: table, Op: op, Err: err}
}