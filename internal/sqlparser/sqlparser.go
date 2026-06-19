package sqlparser

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	selectRegex = regexp.MustCompile(`(?is)^\s*SELECT\s+`)
	withRegex   = regexp.MustCompile(`(?is)^\s*WITH\s+`)
)

var forbiddenKeywords = []string{
	"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "TRUNCATE",
	"REPLACE", "MERGE", "GRANT", "REVOKE", "EXECUTE", "EXEC", "CALL",
	"COPY", "LOAD", "UNLOAD", "LOCK", "UNLOCK", "RENAME", "COMMENT",
	"PRAGMA", "VACUUM", "ATTACH", "DETACH", "BEGIN", "COMMIT", "ROLLBACK",
	"SET", "SHOW", "EXPLAIN", "ANALYZE", "OPTIMIZE", "REPAIR",
}

var forbiddenFunctions = []string{
	"pg_sleep", "sleep", "benchmark", "load_file", "into outfile",
	"into dumpfile", "system", "eval", "exec", "xp_cmdshell",
}

func IsReadOnlyQuery(sql string) error {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return fmt.Errorf("empty SQL query")
	}

	if !selectRegex.MatchString(trimmed) && !withRegex.MatchString(trimmed) {
		return fmt.Errorf("only SELECT queries are allowed")
	}

	upperSQL := strings.ToUpper(trimmed)

	for _, kw := range forbiddenKeywords {
		if strings.Contains(upperSQL, kw) {
			if kw == "SET" && !strings.Contains(upperSQL, "SET ") && !strings.Contains(upperSQL, "SET(") {
				continue
			}
			return fmt.Errorf("forbidden keyword found: %s", kw)
		}
	}

	lowerSQL := strings.ToLower(trimmed)
	for _, fn := range forbiddenFunctions {
		if strings.Contains(lowerSQL, fn) {
			return fmt.Errorf("forbidden function found: %s", fn)
		}
	}

	if strings.Contains(upperSQL, "INTO") {
		intoRegex := regexp.MustCompile(`(?i)\bINTO\s+`)
		if intoRegex.MatchString(trimmed) {
			return fmt.Errorf("SELECT INTO is not allowed")
		}
	}

	if strings.Contains(upperSQL, "FOR UPDATE") || strings.Contains(upperSQL, "FOR SHARE") ||
		strings.Contains(upperSQL, "FOR NO KEY UPDATE") || strings.Contains(upperSQL, "FOR KEY SHARE") {
		return fmt.Errorf("locking clauses (FOR UPDATE, FOR SHARE, etc.) are not allowed")
	}

	return nil
}
