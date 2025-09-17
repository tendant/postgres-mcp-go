package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tendant/postgres-mcp-go/internal/postgresmcp"
)

const (
	modeStdio = "stdio"
	modeHTTP  = "http"
)

type config struct {
	Mode           string
	DatabaseURL    string
	ListenAddr     string
	ReadOnly       bool
	MaxRows        int
	RequestTimeout time.Duration
	HTTPStateless  bool
	HTTPJSON       bool
}

func main() {
	cfg, err := parseConfig()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := configurePool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database error: %v", err)
	}
	defer pool.Close()

	serverOpts := postgresmcp.ServerOptions{
		Pool:           pool,
		ReadOnly:       cfg.ReadOnly,
		MaxRows:        cfg.MaxRows,
		RequestTimeout: cfg.RequestTimeout,
	}

	switch cfg.Mode {
	case modeStdio:
		srv, err := postgresmcp.NewServer(serverOpts)
		if err != nil {
			log.Fatalf("server setup failed: %v", err)
		}
		if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
			log.Fatalf("stdio session ended with error: %v", err)
		}
	case modeHTTP:
		if err := runHTTP(ctx, cfg, serverOpts); err != nil {
			log.Fatalf("http server error: %v", err)
		}
	default:
		log.Fatalf("unknown mode %q", cfg.Mode)
	}
}

func parseConfig() (config, error) {
	var cfg config

	mode := flag.String("mode", modeStdio, "Server mode: stdio or http")
	databaseURL := flag.String("database-url", defaultDatabaseURL(), "PostgreSQL connection string. Defaults to $DATABASE_URL")
	listen := flag.String("listen", ":8080", "HTTP listen address (http mode)")
	readOnly := flag.Bool("readonly", false, "Reject mutating SQL statements")
	maxRows := flag.Int("max-rows", 0, "Maximum rows returned per query (0 uses server default)")
	timeout := flag.Duration("timeout", 0, "Per-request timeout (e.g. 30s). 0 disables")
	httpStateless := flag.Bool("http-stateless", false, "Serve streamable HTTP sessions without retaining state")
	httpJSON := flag.Bool("http-json", false, "Prefer JSON responses for single-message HTTP POSTs")

	flag.Parse()

	cfg.Mode = strings.ToLower(strings.TrimSpace(*mode))
	switch cfg.Mode {
	case modeStdio, modeHTTP:
	default:
		return cfg, fmt.Errorf("invalid mode %q", *mode)
	}

	cfg.DatabaseURL = strings.TrimSpace(*databaseURL)
	if cfg.DatabaseURL == "" {
		return cfg, errors.New("database-url is required")
	}

	cfg.ListenAddr = strings.TrimSpace(*listen)
	cfg.ReadOnly = *readOnly
	cfg.MaxRows = *maxRows
	cfg.RequestTimeout = *timeout
	cfg.HTTPStateless = *httpStateless
	cfg.HTTPJSON = *httpJSON

	return cfg, nil
}

func defaultDatabaseURL() string {
	if v, ok := os.LookupEnv("DATABASE_URL"); ok {
		return v
	}
	return ""
}

func configurePool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = make(map[string]string)
	}
	if _, exists := cfg.ConnConfig.RuntimeParams["application_name"]; !exists {
		cfg.ConnConfig.RuntimeParams["application_name"] = "postgres-mcp-go"
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connectivity check failed: %w", err)
	}

	return pool, nil
}

func runHTTP(ctx context.Context, cfg config, serverOpts postgresmcp.ServerOptions) error {
	getServer := func(*http.Request) *mcp.Server {
		srv, err := postgresmcp.NewServer(serverOpts)
		if err != nil {
			log.Printf("failed to prepare session: %v", err)
			return nil
		}
		return srv
	}

	handler := mcp.NewStreamableHTTPHandler(getServer, &mcp.StreamableHTTPOptions{
		Stateless:    cfg.HTTPStateless,
		JSONResponse: cfg.HTTPJSON,
	})

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("streamable HTTP listening on %s", cfg.ListenAddr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
