package serverless

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aynaash/nextdeploy/shared/config"

	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/d1"
)

// migrationsTable is the bookkeeping table NextDeploy maintains inside each D1
// database to make migrations idempotent. Forward-only: a file is applied once,
// its basename recorded here, and skipped on every subsequent deploy.
const migrationsTable = "_nextdeploy_migrations"

// ensureD1Database creates a D1 database with decl.Name if one doesn't already
// exist (name is unique per account), then applies any pending migrations from
// decl.MigrationsDir. Returns the database UUID for binding resolution.
func (p *CloudflareProvider) ensureD1Database(ctx context.Context, decl config.CFD1Resource) (string, error) {
	id, err := p.findD1ID(ctx, decl.Name)
	if err != nil {
		return "", fmt.Errorf("list d1 databases: %w", err)
	}
	if id == "" {
		params := d1.DatabaseNewParams{
			AccountID: cloudflare.F(p.accountID),
			Name:      cloudflare.F(decl.Name),
		}
		if decl.LocationHint != "" {
			params.PrimaryLocationHint = cloudflare.F(d1.DatabaseNewParamsPrimaryLocationHint(decl.LocationHint))
		}
		created, err := p.cf.D1.Database.New(ctx, params)
		if err != nil {
			return "", fmt.Errorf("create d1 database %q: %w", decl.Name, err)
		}
		id = created.UUID
		p.log.Info("D1 database created: %s (uuid=%s)", decl.Name, id)
	} else {
		p.log.Info("D1 database already exists: %s (uuid=%s)", decl.Name, id)
	}

	if decl.MigrationsDir != "" {
		if err := p.applyD1Migrations(ctx, id, decl); err != nil {
			return "", fmt.Errorf("d1 %q migrations: %w", decl.Name, err)
		}
	}
	return id, nil
}

// findD1ID returns the UUID of the D1 database named name, or "" if none.
// The List endpoint accepts a name filter but still returns prefix matches,
// so we compare exactly.
func (p *CloudflareProvider) findD1ID(ctx context.Context, name string) (string, error) {
	iter := p.cf.D1.Database.ListAutoPaging(ctx, d1.DatabaseListParams{
		AccountID: cloudflare.F(p.accountID),
		Name:      cloudflare.F(name),
	})
	for iter.Next() {
		db := iter.Current()
		if db.Name == name {
			return db.UUID, nil
		}
	}
	if err := iter.Err(); err != nil {
		var apiErr *cloudflare.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return "", nil
		}
		return "", err
	}
	return "", nil
}

// applyD1Migrations applies the *.sql files in decl.MigrationsDir that haven't
// been recorded yet, in lexical order, then records each. Each file is sent as
// a single semicolon-batched query (D1's /query runs multi-statement SQL).
func (p *CloudflareProvider) applyD1Migrations(ctx context.Context, dbID string, decl config.CFD1Resource) error {
	files, err := sortedMigrationFiles(decl.MigrationsDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		p.log.Info("D1 %s: no .sql migrations in %s", decl.Name, decl.MigrationsDir)
		return nil
	}

	if err := p.d1Exec(ctx, dbID, fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s (name TEXT PRIMARY KEY, applied_at TEXT NOT NULL);", migrationsTable)); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	applied, err := p.queryAppliedMigrations(ctx, dbID)
	if err != nil {
		return fmt.Errorf("read applied migrations: %w", err)
	}

	pending := pendingMigrations(files, applied)
	if len(pending) == 0 {
		p.log.Info("D1 %s: migrations up to date (%d applied)", decl.Name, len(applied))
		return nil
	}

	for _, path := range pending {
		name := filepath.Base(path)
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if err := p.d1Exec(ctx, dbID, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if err := p.d1Record(ctx, dbID, name); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		p.log.Info("D1 %s: applied migration %s", decl.Name, name)
	}
	return nil
}

// d1Exec runs a (possibly multi-statement) SQL string against the database.
func (p *CloudflareProvider) d1Exec(ctx context.Context, dbID, sql string) error {
	_, err := p.cf.D1.Database.Query(ctx, dbID, d1.DatabaseQueryParams{
		AccountID: cloudflare.F(p.accountID),
		Body:      d1.DatabaseQueryParamsBodyD1SingleQuery{Sql: cloudflare.F(sql)},
	})
	return err
}

// d1Record inserts a migration name into the tracking table with a UTC stamp.
func (p *CloudflareProvider) d1Record(ctx context.Context, dbID, name string) error {
	_, err := p.cf.D1.Database.Query(ctx, dbID, d1.DatabaseQueryParams{
		AccountID: cloudflare.F(p.accountID),
		Body: d1.DatabaseQueryParamsBodyD1SingleQuery{
			Sql:    cloudflare.F(fmt.Sprintf("INSERT INTO %s (name, applied_at) VALUES (?, ?);", migrationsTable)),
			Params: cloudflare.F([]string{name, time.Now().UTC().Format(time.RFC3339)}),
		},
	})
	return err
}

// queryAppliedMigrations returns the set of migration names already recorded.
func (p *CloudflareProvider) queryAppliedMigrations(ctx context.Context, dbID string) (map[string]bool, error) {
	page, err := p.cf.D1.Database.Query(ctx, dbID, d1.DatabaseQueryParams{
		AccountID: cloudflare.F(p.accountID),
		Body:      d1.DatabaseQueryParamsBodyD1SingleQuery{Sql: cloudflare.F("SELECT name FROM " + migrationsTable + ";")},
	})
	if err != nil {
		return nil, err
	}
	applied := map[string]bool{}
	for _, qr := range page.Result {
		for n := range parseAppliedMigrations(qr.Results) {
			applied[n] = true
		}
	}
	return applied, nil
}

// --- pure helpers (no network) -------------------------------------------------

// sortedMigrationFiles returns absolute paths of *.sql files in dir, sorted by
// basename so numbered files (drizzle-kit: 0000_x.sql, 0001_y.sql) apply in order.
func sortedMigrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %q: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".sql") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Slice(files, func(i, j int) bool {
		return filepath.Base(files[i]) < filepath.Base(files[j])
	})
	return files, nil
}

// pendingMigrations returns the files (in input order) whose basename is not in
// the applied set.
func pendingMigrations(files []string, applied map[string]bool) []string {
	var pending []string
	for _, f := range files {
		if !applied[filepath.Base(f)] {
			pending = append(pending, f)
		}
	}
	return pending
}

// parseAppliedMigrations extracts the "name" column from D1 query result rows.
// Rows arrive as []interface{} of map[string]interface{} (JSON object per row).
func parseAppliedMigrations(rows []interface{}) map[string]bool {
	out := map[string]bool{}
	for _, row := range rows {
		m, ok := row.(map[string]interface{})
		if !ok {
			continue
		}
		if name, ok := m["name"].(string); ok && name != "" {
			out[name] = true
		}
	}
	return out
}
