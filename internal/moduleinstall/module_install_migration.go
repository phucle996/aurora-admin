package moduleinstall

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func materializeMigrations(
	ctx context.Context,
	moduleName string,
	source moduleMigrationSource,
) (string, string, func(), error) {
	rootDir, err := os.MkdirTemp("", "aurora-module-migrations-*")
	if err != nil {
		return "", "", func() {}, err
	}
	cleanup := func() {
		_ = os.RemoveAll(rootDir)
	}

	destDir := filepath.Join(rootDir, moduleName)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		cleanup()
		return "", "", func() {}, err
	}

	for _, url := range source.DownloadURLs {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		if err := downloadMigrationZip(ctx, url, destDir); err == nil {
			files, listErr := listMigrationUpFiles(destDir)
			if listErr == nil && len(files) > 0 {
				return destDir, "download:" + url, cleanup, nil
			}
		}
	}

	cleanup()
	return "", "", func() {}, fmt.Errorf("no migration source available for module %s", moduleName)
}

func downloadMigrationZip(ctx context.Context, url string, destDir string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 20 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("download failed with status %d", response.StatusCode)
	}

	payload, err := io.ReadAll(io.LimitReader(response.Body, 64<<20))
	if err != nil {
		return err
	}

	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return err
	}

	count := 0
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		name := filepath.ToSlash(strings.TrimSpace(file.Name))
		if !strings.Contains(name, "/migrations/") || !strings.HasSuffix(name, ".up.sql") {
			continue
		}

		targetPath := filepath.Join(destDir, filepath.Base(name))
		if err := extractZipFile(file, targetPath); err != nil {
			return err
		}
		count++
	}
	if count == 0 {
		return fmt.Errorf("no *.up.sql found in archive")
	}
	return nil
}

func extractZipFile(file *zip.File, destination string) error {
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()

	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, reader); err != nil {
		return err
	}
	return nil
}

func listMigrationUpFiles(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no migration file (*.up.sql) found")
	}
	sort.Strings(files)
	return files, nil
}

func rewriteMigrationFilesForSchema(files []string, legacySchema string, targetSchema string) ([]string, func(), bool, error) {
	cleanLegacy := strings.TrimSpace(legacySchema)
	cleanTarget := strings.TrimSpace(targetSchema)
	if cleanLegacy == "" || cleanTarget == "" || strings.EqualFold(cleanLegacy, cleanTarget) {
		return files, func() {}, false, nil
	}

	rootDir, err := os.MkdirTemp("", "aurora-module-migrations-rewrite-*")
	if err != nil {
		return nil, func() {}, false, err
	}
	cleanup := func() {
		_ = os.RemoveAll(rootDir)
	}

	rewrittenFiles := make([]string, 0, len(files))
	for _, file := range files {
		raw, readErr := os.ReadFile(file)
		if readErr != nil {
			cleanup()
			return nil, func() {}, false, readErr
		}

		rewrittenSQL, rewriteErr := rewriteMigrationSQLForSchema(string(raw), cleanLegacy, cleanTarget)
		if rewriteErr != nil {
			cleanup()
			return nil, func() {}, false, rewriteErr
		}

		targetPath := filepath.Join(rootDir, filepath.Base(file))
		if writeErr := os.WriteFile(targetPath, []byte(rewrittenSQL), 0o644); writeErr != nil {
			cleanup()
			return nil, func() {}, false, writeErr
		}
		rewrittenFiles = append(rewrittenFiles, targetPath)
	}

	return rewrittenFiles, cleanup, true, nil
}

func rewriteMigrationSQLForSchema(sql string, legacySchema string, targetSchema string) (string, error) {
	cleanLegacy := strings.TrimSpace(legacySchema)
	cleanTarget := strings.TrimSpace(targetSchema)
	if cleanLegacy == "" || cleanTarget == "" {
		return sql, nil
	}
	if strings.EqualFold(cleanLegacy, cleanTarget) {
		return sql, nil
	}

	legacyQuoted := quoteSQLIdentifier(cleanLegacy)
	targetQuoted := quoteSQLIdentifier(cleanTarget)

	rewritten := sql

	// Normalize hard-coded schema declaration into target runtime schema.
	schemaDeclPattern := `(?im)CREATE\s+SCHEMA\s+IF\s+NOT\s+EXISTS\s+("?` + regexp.QuoteMeta(cleanLegacy) + `"?)(\s*);`
	reDecl, err := regexp.Compile(schemaDeclPattern)
	if err != nil {
		return "", err
	}
	rewritten = reDecl.ReplaceAllString(rewritten, `CREATE SCHEMA IF NOT EXISTS `+targetQuoted+`;`)

	// Replace qualified object references: ums.table or "ums".table -> "<target>".table
	quotedRefPattern := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(legacyQuoted) + `\s*\.`)
	rewritten = quotedRefPattern.ReplaceAllString(rewritten, targetQuoted+".")

	unquotedRefPattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(cleanLegacy) + `\s*\.`)
	rewritten = unquotedRefPattern.ReplaceAllString(rewritten, targetQuoted+".")

	return rewritten, nil
}

func generateSchemaName(moduleName string) (string, error) {
	digits, err := randomDigits(schemaDigitsCount)
	if err != nil {
		return "", err
	}
	return normalizeSchemaPrefix(moduleName) + "_" + digits, nil
}

func randomDigits(length int) (string, error) {
	if length <= 0 {
		return "", nil
	}
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	out := strings.Builder{}
	out.Grow(length)
	for _, b := range buffer {
		out.WriteByte('0' + (b % 10))
	}
	return out.String(), nil
}

func normalizeSchemaPrefix(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "/", "_")
	if value == "" {
		return "svc"
	}
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' {
			continue
		}
		value = strings.ReplaceAll(value, string(ch), "_")
	}
	if value[0] < 'a' || value[0] > 'z' {
		value = "svc_" + value
	}
	return value
}

func createSchemaWithSQL(ctx context.Context, databaseURL string, schema string) error {
	sql := "CREATE SCHEMA IF NOT EXISTS " + quoteSQLIdentifier(schema) + ";"
	return execSQL(ctx, databaseURL, sql)
}

func dropSchemaWithSQL(ctx context.Context, databaseURL string, schema string) error {
	cleanSchema := strings.TrimSpace(schema)
	if cleanSchema == "" {
		return nil
	}

	sql := "DROP SCHEMA IF EXISTS " + quoteSQLIdentifier(cleanSchema) + " CASCADE;"
	return execSQL(ctx, databaseURL, sql)
}

func runMigrationFilesWithSQL(
	ctx context.Context,
	databaseURL string,
	schema string,
	migrationFiles []string,
	logFn InstallLogFn,
) error {
	connection, err := pgx.Connect(ctx, strings.TrimSpace(databaseURL))
	if err != nil {
		return fmt.Errorf("postgres connect failed: %w", err)
	}
	defer connection.Close(ctx)

	searchPathSQL := "SET search_path TO " + quoteSQLIdentifier(schema) + ", public;"

	for _, file := range migrationFiles {
		filename := filepath.Base(file)
		logInstall(logFn, "migration", "running %s", filename)
		payload, readErr := os.ReadFile(file)
		if readErr != nil {
			return fmt.Errorf("read migration %s failed: %w", filename, readErr)
		}
		sql := searchPathSQL + "\n" + string(payload)

		_, err = connection.Exec(ctx, sql)
		if err != nil {
			return fmt.Errorf("apply migration %s failed: %w", filename, err)
		}
		logInstall(logFn, "migration", "applied %s", filename)
	}
	return nil
}

func execSQL(ctx context.Context, databaseURL string, sql string) error {
	connection, err := pgx.Connect(ctx, strings.TrimSpace(databaseURL))
	if err != nil {
		return fmt.Errorf("postgres connect failed: %w", err)
	}
	defer connection.Close(ctx)

	if _, err := connection.Exec(ctx, sql); err != nil {
		return fmt.Errorf("postgres exec failed: %w", err)
	}
	return nil
}

func quoteSQLIdentifier(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.ReplaceAll(value, `"`, `""`)
	return `"` + value + `"`
}
