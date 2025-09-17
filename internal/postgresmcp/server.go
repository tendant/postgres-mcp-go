package postgresmcp

import (
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// Version identifies this MCP server implementation. Update when behavior changes.
	Version = "0.1.0"

	defaultMaxRows = 200
)

// ServerOptions configures construction of an MCP server backed by PostgreSQL.
type ServerOptions struct {
	Pool *pgxpool.Pool

	// ReadOnly blocks mutating SQL statements when true.
	ReadOnly bool
	// MaxRows caps the number of rows returned per query. Zero falls back to the
	// internal default.
	MaxRows int
	// RequestTimeout defines the maximum duration allowed for handling a single
	// tool invocation. Zero disables the additional timeout.
	RequestTimeout time.Duration
}

// NewServer wires up an MCP server that exposes PostgreSQL via the go-sdk.
func NewServer(opts ServerOptions) (*mcp.Server, error) {
	if opts.Pool == nil {
		return nil, fmt.Errorf("postgresmcp: pool must not be nil")
	}

	maxRows := opts.MaxRows
	if maxRows <= 0 {
		maxRows = defaultMaxRows
	}

	instructions := []string{
		"Use the `postgres.query` tool to run SQL against PostgreSQL.",
		"Provide JSON arguments {\"sql\": string, \"args\": array, \"maxRows\": number}.",
	}
	if opts.ReadOnly {
		instructions = append(instructions, "This server enforces read-only queries.")
	}

	impl := &mcp.Implementation{
		Name:    "postgres-mcp-go",
		Title:   "PostgreSQL MCP Server",
		Version: Version,
	}

	server := mcp.NewServer(impl, &mcp.ServerOptions{
		Instructions: strings.Join(instructions, " "),
		HasTools:     true,
	})

	h := queryHandler{
		pool:           opts.Pool,
		readOnly:       opts.ReadOnly,
		maxRows:        maxRows,
		requestTimeout: opts.RequestTimeout,
	}

	registerQueryTool(server, &h)

	return server, nil
}
