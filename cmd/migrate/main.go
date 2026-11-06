// Command migrate imports the legacy SQLite database into the PostgreSQL
// database configured by DATABASE_URL. It is intentionally a separate command
// so the application never has to access SQLite in production.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"noder/internal/config"
	"noder/internal/db"
)

var tables = []string{
	"user", "node", "usernodelink", "endpoint", "distfile", "user_download_log",
	"template", "templatehistory", "template_branch", "appsetting",
	"credentials", "access_grants", "agent_nodes", "endpoint_agent_bindings",
	"credential_deployments", "legacy_migration_map",
}

var tablesWithSequence = []string{
	"user", "node", "endpoint", "distfile", "user_download_log", "template",
	"templatehistory", "template_branch", "legacy_migration_map",
}

type columnInfo struct {
	name       string
	typeName   string
	nullable   bool
	hasDefault bool
}

func main() {
	sourcePath := flag.String("sqlite", "data.db", "legacy SQLite database path")
	filesSource := flag.String("files-source", "", "legacy uploaded-files directory")
	filesDestination := flag.String("files-destination", "", "destination uploaded-files directory")
	replace := flag.Bool("replace", false, "replace existing PostgreSQL application data")
	flag.Parse()

	if config.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx := context.Background()
	if err := run(ctx, *sourcePath, *replace, *filesSource, *filesDestination); err != nil {
		log.Fatal(err)
	}
	log.Println("SQLite data imported into PostgreSQL successfully")
}

func run(ctx context.Context, sourcePath string, replace bool, filesSource, filesDestination string) error {
	source, err := openSQLite(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := db.OpenDB()
	if err != nil {
		return fmt.Errorf("open PostgreSQL: %w", err)
	}
	defer destination.Close()

	if err := db.CreateDBAndTables(ctx); err != nil {
		return fmt.Errorf("create PostgreSQL schema: %w", err)
	}
	if err := ensureStateTable(ctx); err != nil {
		return err
	}

	state, err := migrationState(ctx)
	if err != nil {
		return err
	}
	if state == "completed" && !replace {
		return nil
	}
	if state == "in_progress" {
		// A previous process may have been killed after clearing the target. The
		// import transaction itself rolled back, so retry from a clean target.
		replace = true
	}
	if !replace {
		nonEmpty, err := applicationDataExists(ctx)
		if err != nil {
			return err
		}
		if nonEmpty {
			return errors.New("PostgreSQL already contains application data; use --replace only after taking a backup")
		}
	}
	if replace {
		if err := clearApplicationData(ctx); err != nil {
			return err
		}
	}
	if err := setMigrationState(ctx, "in_progress"); err != nil {
		return err
	}

	if err := copyTables(ctx, source); err != nil {
		return err
	}
	if err := resetSequences(ctx); err != nil {
		return err
	}
	// Keep behavior identical to a normal first application start for older
	// SQLite files that predate settings/templates/canonical projections.
	if err := db.SeedDefaultData(ctx); err != nil {
		return fmt.Errorf("seed PostgreSQL defaults: %w", err)
	}
	if err := db.ProjectLegacyData(ctx); err != nil {
		return fmt.Errorf("project canonical data: %w", err)
	}
	if filesSource != "" && filesDestination != "" {
		if err := copyFiles(filesSource, filesDestination); err != nil {
			return fmt.Errorf("copy uploaded files: %w", err)
		}
	}
	return setMigrationState(ctx, "completed")
}

func copyFiles(sourceDir, destinationDir string) error {
	info, err := os.Stat(sourceDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", sourceDir)
	}
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(destinationDir, relative)
		if info.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func openSQLite(path string) (*sql.DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	database, err := sql.Open("sqlite", "file:"+abs+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open SQLite: %w", err)
	}
	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("read SQLite: %w", err)
	}
	return database, nil
}

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func ensureStateTable(ctx context.Context) error {
	_, err := db.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS noder_migration_state (
		name VARCHAR PRIMARY KEY,
		status VARCHAR NOT NULL,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	return err
}

func migrationState(ctx context.Context) (string, error) {
	var state string
	err := db.DB.NewRaw("SELECT status FROM noder_migration_state WHERE name = 'sqlite-import'").Scan(ctx, &state)
	if err != nil {
		// bun returns sql.ErrNoRows for an uninitialized state row.
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return state, nil
}

func setMigrationState(ctx context.Context, state string) error {
	_, err := db.DB.ExecContext(ctx, `INSERT INTO noder_migration_state (name, status)
		VALUES ('sqlite-import', ?)
		ON CONFLICT (name) DO UPDATE SET status = EXCLUDED.status, updated_at = CURRENT_TIMESTAMP`, state)
	return err
}

func applicationDataExists(ctx context.Context) (bool, error) {
	for _, table := range tables {
		var count int
		if err := db.DB.NewRaw("SELECT COUNT(*) FROM "+quoteIdentifier(table)).Scan(ctx, &count); err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}

func clearApplicationData(ctx context.Context) error {
	identifiers := make([]string, 0, len(tables))
	for _, table := range tables {
		identifiers = append(identifiers, quoteIdentifier(table))
	}
	if _, err := db.DB.ExecContext(ctx, "TRUNCATE TABLE "+strings.Join(identifiers, ", ")+" RESTART IDENTITY CASCADE"); err != nil {
		return fmt.Errorf("clear PostgreSQL application data: %w", err)
	}
	return nil
}

func copyTables(ctx context.Context, source *sql.DB) error {
	for _, table := range tables {
		sourceColumns, err := sqliteColumns(ctx, source, table)
		if err != nil {
			return err
		}
		destinationColumns, err := postgresColumns(ctx, table)
		if err != nil {
			return err
		}
		columns := make([]columnInfo, 0, len(sourceColumns))
		for _, column := range sourceColumns {
			if destination, ok := destinationColumns[column.name]; ok {
				column.typeName = destination.typeName
				column.nullable = destination.nullable
				column.hasDefault = destination.hasDefault
				columns = append(columns, column)
			}
		}
		if len(columns) == 0 {
			continue
		}
		if err := copyTable(ctx, source, table, columns); err != nil {
			return err
		}
	}
	return nil
}

func sqliteColumns(ctx context.Context, source *sql.DB, table string) ([]columnInfo, error) {
	rows, err := source.QueryContext(ctx, "PRAGMA table_info("+quoteIdentifier(table)+")")
	if err != nil {
		return nil, fmt.Errorf("inspect SQLite table %s: %w", table, err)
	}
	defer rows.Close()
	var columns []columnInfo
	for rows.Next() {
		var cid int
		var name, typeName string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		columns = append(columns, columnInfo{name: name, typeName: typeName})
	}
	return columns, rows.Err()
}

func postgresColumns(ctx context.Context, table string) (map[string]columnInfo, error) {
	rows, err := db.DB.QueryContext(ctx, `SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = ?`, table)
	if err != nil {
		return nil, fmt.Errorf("inspect PostgreSQL table %s: %w", table, err)
	}
	defer rows.Close()
	columns := make(map[string]columnInfo)
	for rows.Next() {
		var name, typeName, nullable string
		var defaultValue sql.NullString
		if err := rows.Scan(&name, &typeName, &nullable, &defaultValue); err != nil {
			return nil, err
		}
		columns[name] = columnInfo{
			name:       name,
			typeName:   typeName,
			nullable:   nullable == "YES",
			hasDefault: defaultValue.Valid,
		}
	}
	return columns, rows.Err()
}

func copyTable(ctx context.Context, source *sql.DB, table string, columns []columnInfo) error {
	columnNames := make([]string, len(columns))
	for i, column := range columns {
		columnNames[i] = quoteIdentifier(column.name)
	}
	query := "SELECT " + strings.Join(columnNames, ", ") + " FROM " + quoteIdentifier(table)
	rows, err := source.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("read SQLite table %s: %w", table, err)
	}
	defer rows.Close()

	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	count := 0
	for rows.Next() {
		values := make([]any, len(columns))
		scanTargets := make([]any, len(columns))
		for i := range values {
			scanTargets[i] = &values[i]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return fmt.Errorf("read row from SQLite table %s: %w", table, err)
		}
		for i := range values {
			values[i] = normalizeValue(values[i], columns[i].typeName)
		}

		rowColumns := make([]string, 0, len(columns))
		placeholders := make([]string, 0, len(columns))
		rowValues := make([]any, 0, len(columns))
		for i, value := range values {
			if value == nil && !columns[i].nullable && columns[i].hasDefault {
				// An explicit NULL bypasses a PostgreSQL default. Omitting this
				// legacy NULL lets the destination schema apply its default.
				continue
			}
			if value == nil && !columns[i].nullable {
				return fmt.Errorf("SQLite table %s has NULL in required column %s", table, columns[i].name)
			}
			rowColumns = append(rowColumns, columnNames[i])
			placeholders = append(placeholders, "?")
			rowValues = append(rowValues, value)
		}
		insert := "INSERT INTO " + quoteIdentifier(table) + " (" + strings.Join(rowColumns, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ") ON CONFLICT DO NOTHING"
		if _, err := tx.ExecContext(ctx, insert, rowValues...); err != nil {
			return fmt.Errorf("insert row into PostgreSQL table %s: %w", table, err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("imported %d rows from %s", count, table)
	return nil
}

func normalizeValue(value any, typeName string) any {
	if raw, ok := value.([]byte); ok && typeName != "bytea" {
		value = string(raw)
	}
	if typeName != "boolean" {
		return value
	}
	switch v := value.(type) {
	case int64:
		return v != 0
	case int:
		return v != 0
	case string:
		return v == "1" || strings.EqualFold(v, "true")
	case []byte:
		return string(v) == "1" || strings.EqualFold(string(v), "true")
	default:
		return value
	}
}

func resetSequences(ctx context.Context) error {
	for _, table := range tablesWithSequence {
		var maxID sql.NullInt64
		if err := db.DB.NewRaw("SELECT MAX(id) FROM "+quoteIdentifier(table)).Scan(ctx, &maxID); err != nil {
			return err
		}
		sequenceName := "public." + table
		if _, err := db.DB.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence(?, 'id'), ?, ?)`, sequenceName, maxOrOne(maxID), maxID.Valid); err != nil {
			return fmt.Errorf("reset sequence for %s: %w", table, err)
		}
	}
	return nil
}

func maxOrOne(value sql.NullInt64) int64 {
	if !value.Valid || value.Int64 < 1 {
		return 1
	}
	return value.Int64
}
