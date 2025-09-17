package postgresmcp

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type queryHandler struct {
	pool           *pgxpool.Pool
	readOnly       bool
	maxRows        int
	requestTimeout time.Duration
}

type queryInput struct {
	SQL     string        `json:"sql" jsonschema:"title=SQL statement,description=Statement to execute against PostgreSQL"`
	Args    []any         `json:"args,omitempty" jsonschema:"title=Parameters,description=Positional parameters that map to $1, $2, ..."`
	MaxRows int           `json:"maxRows,omitempty" jsonschema:"title=Row limit,description=Override the default row limit for this call,minimum=1"`
	Meta    mcp.Meta      `json:"_meta,omitempty"`
}

type queryOutput struct {
	Command   string              `json:"command"`
	RowCount  int64               `json:"rowCount"`
	Columns   []string            `json:"columns,omitempty"`
	Rows      []map[string]any    `json:"rows,omitempty"`
	Truncated bool                `json:"truncated,omitempty"`
	Elapsed   string              `json:"elapsed"`
	Meta      mcp.Meta            `json:"_meta,omitempty"`
}

func registerQueryTool(server *mcp.Server, handler *queryHandler) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "postgres.query",
		Description: "Execute a SQL statement against PostgreSQL.",
	}, handler.call)
}

func (h *queryHandler) call(ctx context.Context, _ *mcp.CallToolRequest, input queryInput) (*mcp.CallToolResult, queryOutput, error) {
	sqlText := strings.TrimSpace(input.SQL)
	if sqlText == "" {
		return nil, queryOutput{}, errors.New("sql must not be empty")
	}
	if !isSingleStatement(sqlText) {
		return nil, queryOutput{}, errors.New("only a single SQL statement is supported per call")
	}
	if h.readOnly && !isReadOnlyStatement(sqlText) {
		return nil, queryOutput{}, errors.New("mutating statements are disabled in read-only mode")
	}

	limit := h.maxRows
	if input.MaxRows < 0 {
		return nil, queryOutput{}, errors.New("maxRows must be positive")
	}
	if input.MaxRows > 0 && (limit == 0 || input.MaxRows < limit) {
		limit = input.MaxRows
	}

	params := make([]any, len(input.Args))
	for i := range input.Args {
		params[i] = normalizeArgument(input.Args[i])
	}

	ctx, cancel := applyTimeout(ctx, h.requestTimeout)
	if cancel != nil {
		defer cancel()
	}

	start := time.Now()
	rows, err := h.pool.Query(ctx, sqlText, params...)
	if err != nil {
		return nil, queryOutput{}, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	columns := make([]string, len(fields))
	for i, fd := range fields {
		columns[i] = fd.Name
	}

	var (
		dataRows []map[string]any
		count    int64
		trunc    bool
	)

	for rows.Next() {
		if limit > 0 && count >= int64(limit) {
			trunc = true
			break
		}
		values, err := rows.Values()
		if err != nil {
			return nil, queryOutput{}, err
		}
		record := make(map[string]any, len(values))
		for i, col := range columns {
			var val any
			if i < len(values) {
				val = normalizeValue(values[i])
			}
			record[col] = val
		}
		dataRows = append(dataRows, record)
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, queryOutput{}, err
	}

	tag := rows.CommandTag()
	rowCount := count
	if rowCount == 0 && len(columns) == 0 {
		rowCount = int64(tag.RowsAffected())
	}

	out := queryOutput{
		Command:   commandString(tag),
		RowCount:  rowCount,
		Columns:   columns,
		Rows:      dataRows,
		Truncated: trunc,
		Elapsed:   time.Since(start).Round(time.Millisecond).String(),
	}

	return nil, out, nil
}

func applyTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return ctx, nil
	}
	return context.WithTimeout(ctx, d)
}

func commandString(tag pgconn.CommandTag) string {
	if tag == nil {
		return ""
	}
	return string(tag)
}

func normalizeArgument(arg any) any {
	switch v := arg.(type) {
	case float64:
		if math.Trunc(v) == v {
			return int64(v)
		}
		return v
	case map[string]any:
		converted := make(map[string]any, len(v))
		for k, val := range v {
			converted[k] = normalizeArgument(val)
		}
		return converted
	case []any:
		converted := make([]any, len(v))
		for i := range v {
			converted[i] = normalizeArgument(v[i])
		}
		return converted
	default:
		return arg
	}
}

func normalizeValue(v any) any {
	switch val := v.(type) {
	case []byte:
		return string(val)
	case time.Time:
		return val.UTC().Format(time.RFC3339Nano)
	case pgtype.Numeric:
		if !val.Valid {
			return nil
		}
		return val.String()
	case *pgtype.Numeric:
		if val == nil || !val.Valid {
			return nil
		}
		return val.String()
	case fmt.Stringer:
		return val.String()
	default:
		return v
	}
}

var allowedReadOnly = map[string]struct{}{
	"SELECT": {},
	"WITH":   {},
	"SHOW":   {},
	"EXPLAIN": {},
	"VALUES":  {},
	"TABLE":   {},
}

func isReadOnlyStatement(sql string) bool {
	kw := firstKeyword(sql)
	if kw == "" {
		return false
	}
	_, ok := allowedReadOnly[kw]
	return ok
}

func isSingleStatement(sql string) bool {
	t := strings.TrimSpace(sql)
	if t == "" {
		return false
	}
	semi := strings.Count(t, ";")
	if semi == 0 {
		return true
	}
	if semi > 1 {
		return false
	}
	idx := strings.LastIndex(t, ";")
	if idx == len(t)-1 {
		return strings.TrimSpace(t[:idx]) != ""
	}
	return false
}

func firstKeyword(sql string) string {
	s := strings.TrimSpace(sql)
	for {
		switch {
		case strings.HasPrefix(s, "--"):
			newline := strings.IndexByte(s, '\n')
			if newline < 0 {
				return ""
			}
			s = strings.TrimSpace(s[newline+1:])
			continue
		case strings.HasPrefix(s, "/*"):
			end := strings.Index(s, "*/")
			if end < 0 {
				return ""
			}
			s = strings.TrimSpace(s[end+2:])
			continue
		}
		break
	}
	if s == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || r == '_' {
			builder.WriteRune(unicode.ToUpper(r))
			continue
		}
		if builder.Len() == 0 && unicode.IsSpace(r) {
			continue
		}
		break
	}
	return builder.String()
}
