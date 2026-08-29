package postgres

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/priyanshuguptadev/job-board/internal/domain"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func isUUID(s string) bool {
	return uuidRegex.MatchString(s)
}

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return domain.ErrConflict
		case "23503": // foreign_key_violation
			return domain.ErrNotFound
		}
	}
	return err
}

// TextArray is a slice of strings that implements driver.Valuer and sql.Scanner for PostgreSQL text[] columns.
type TextArray []string

// Value implements driver.Valuer.
func (a TextArray) Value() (driver.Value, error) {
	if a == nil {
		return "{}", nil
	}
	var b strings.Builder
	b.WriteByte('{')
	for i, s := range a {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		for _, r := range s {
			if r == '\\' || r == '"' {
				b.WriteByte('\\')
			}
			b.WriteRune(r)
		}
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String(), nil
}

// Scan implements sql.Scanner.
func (a *TextArray) Scan(src interface{}) error {
	if src == nil {
		*a = []string{}
		return nil
	}

	var str string
	switch v := src.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	case []string:
		*a = v
		return nil
	case []interface{}:
		res := make([]string, len(v))
		for i, item := range v {
			res[i] = fmt.Sprint(item)
		}
		*a = res
		return nil
	default:
		return fmt.Errorf("unsupported type for TextArray scan: %T", src)
	}

	str = strings.TrimSpace(str)
	if str == "" || str == "{}" {
		*a = []string{}
		return nil
	}
	if !strings.HasPrefix(str, "{") || !strings.HasSuffix(str, "}") {
		return fmt.Errorf("invalid array format: %s", str)
	}

	content := str[1 : len(str)-1]
	if content == "" {
		*a = []string{}
		return nil
	}

	var elements []string
	var cur strings.Builder
	inQuotes := false
	escaped := false

	for i := 0; i < len(content); i++ {
		c := content[i]
		if escaped {
			cur.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			inQuotes = !inQuotes
			continue
		}
		if c == ',' && !inQuotes {
			elements = append(elements, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	elements = append(elements, cur.String())
	*a = elements
	return nil
}
