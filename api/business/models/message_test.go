package models_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
)

func TestNormalizeBody(t *testing.T) {
	body, err := models.NormalizeBody("  hello \n")
	require.NoError(t, err)
	assert.Equal(t, "hello", body)

	for name, raw := range map[string]string{
		"empty":           "",
		"only whitespace": " \n\t ",
		"2001 characters": strings.Repeat("a", 2001),
	} {
		_, err := models.NormalizeBody(raw)
		assert.ErrorIs(t, err, models.ErrInvalidInput, name)
	}
}

func TestNormalizeBodyCountsCharactersNotBytes(t *testing.T) {
	thai := strings.Repeat("ส", 2000) // 6000 bytes, 2000 characters

	body, err := models.NormalizeBody(thai)

	require.NoError(t, err)
	assert.Equal(t, thai, body)
}
