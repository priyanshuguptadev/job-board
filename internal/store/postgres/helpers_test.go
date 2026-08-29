package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelpers(t *testing.T) {
	t.Run("isUUID", func(t *testing.T) {
		assert.True(t, isUUID("c56a4180-65aa-42ec-a945-5fd21dec0538"))
		assert.True(t, isUUID("00000000-0000-0000-0000-000000000000"))
		assert.False(t, isUUID("senior-go-developer"))
		assert.False(t, isUUID("12345"))
		assert.False(t, isUUID(""))
	})

	t.Run("mapDBError", func(t *testing.T) {
		assert.Nil(t, mapDBError(nil))

		pgErrUnique := &pgconn.PgError{Code: "23505"}
		assert.ErrorIs(t, mapDBError(pgErrUnique), domain.ErrConflict)

		pgErrFK := &pgconn.PgError{Code: "23503"}
		assert.ErrorIs(t, mapDBError(pgErrFK), domain.ErrNotFound)

		otherErr := errors.New("some error")
		assert.Equal(t, otherErr, mapDBError(otherErr))
	})

	t.Run("TextArray Value and Scan", func(t *testing.T) {
		// Value nil
		var nilArr TextArray
		val, err := nilArr.Value()
		require.NoError(t, err)
		assert.Equal(t, "{}", val)

		// Value with items
		arr := TextArray{"item1", "item,with,comma", `item"with"quotes`, `item\with\backslash`}
		val, err = arr.Value()
		require.NoError(t, err)

		// Scan back
		var scanned TextArray
		err = scanned.Scan(val)
		require.NoError(t, err)
		assert.Equal(t, arr, scanned)

		// Scan nil
		var scannedNil TextArray
		require.NoError(t, scannedNil.Scan(nil))
		assert.Empty(t, scannedNil)

		// Scan empty string / empty array literal
		var scannedEmpty TextArray
		require.NoError(t, scannedEmpty.Scan("{}"))
		assert.Empty(t, scannedEmpty)

		// Scan []string directly
		var scannedSlice TextArray
		require.NoError(t, scannedSlice.Scan([]string{"a", "b"}))
		assert.Equal(t, TextArray{"a", "b"}, scannedSlice)

		// Scan invalid format
		var scannedInvalid TextArray
		assert.Error(t, scannedInvalid.Scan("invalid"))
	})
}
