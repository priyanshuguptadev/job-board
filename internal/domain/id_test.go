package domain_test

import (
	"regexp"
	"testing"

	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/stretchr/testify/assert"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func TestNewID(t *testing.T) {
	id1 := domain.NewID()
	id2 := domain.NewID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.True(t, uuidRegex.MatchString(id1), "id1 %s must match UUID v4 format", id1)
	assert.True(t, uuidRegex.MatchString(id2), "id2 %s must match UUID v4 format", id2)
}
