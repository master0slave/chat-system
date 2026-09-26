package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The web app runs on :3100 (web/package.json), so that is the origin the API allows by default.
func TestDefaultWebOriginIsTheWebApp(t *testing.T) {
	t.Setenv("WEB_ORIGINS", "")

	assert.Equal(t, []string{"http://localhost:3100"}, configFromEnv().AllowedOrigins)
}

func TestWebOriginsFromEnvironment(t *testing.T) {
	t.Setenv("WEB_ORIGINS", "http://a.example,http://b.example")

	assert.Equal(t, []string{"http://a.example", "http://b.example"}, configFromEnv().AllowedOrigins)
}
