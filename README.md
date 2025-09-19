# postgres-mcp-go

A PostgreSQL MCP (Model Context Protocol) server implementation in Go that provides secure database access for AI agents.

## Features

- **Multiple modes**: stdio and HTTP server
- **Security**: Read-only mode support, query validation
- **Connection pooling**: Efficient PostgreSQL connection management
- **Configurable limits**: Row limits, timeouts, request size controls

## Quick Start

```bash
# Build
make build

# Run in stdio mode (for AI agents)
DATABASE_URL="postgres://user:pass@host/db" make run-stdio

# Run HTTP server
DATABASE_URL="postgres://user:pass@host/db" make run-http
```

## Usage

```bash
# Basic usage
./bin/postgres-mcp --database-url "postgres://user:pass@host/db"

# Read-only mode (recommended)
./bin/postgres-mcp --database-url "postgres://user:pass@host/db" --readonly

# HTTP mode
./bin/postgres-mcp --mode http --listen :8080 --database-url "postgres://user:pass@host/db"
```

## Configuration

- `--mode`: `stdio` (default) or `http`
- `--database-url`: PostgreSQL connection string
- `--readonly`: Reject mutating SQL statements
- `--max-rows`: Limit rows returned per query
- `--timeout`: Per-request timeout
- `--listen`: HTTP listen address (HTTP mode)

## Requirements

- Go 1.23.3+
- PostgreSQL database