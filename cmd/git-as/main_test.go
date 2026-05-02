package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jdevera/git-this-bread/internal/identity"
)

func envCount(env []string, key string) int {
	n := 0
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if name == key {
			n++
		}
	}
	return n
}

func envLookup(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		name, val, _ := strings.Cut(env[i], "=")
		if name == key {
			return val
		}
	}
	return ""
}

func TestBuildEnvOverridesParent(t *testing.T) {
	parent := []string{
		"PATH=/usr/bin",
		"GIT_SSH_COMMAND=junk-from-parent",
		"GIT_AUTHOR_EMAIL=junk@parent",
		"GIT_COMMITTER_EMAIL=junk@parent",
		"GIT_AUTHOR_NAME=junk-name",
		"GIT_COMMITTER_NAME=junk-name",
	}
	profile := &identity.Profile{
		Name:        "test",
		SSHKey:      "/tmp/k",
		Email:       "real@profile",
		DisplayName: "Real Name",
	}

	env := buildEnv(parent, profile, "/tmp/k", "")

	for _, key := range []string{
		"GIT_SSH_COMMAND",
		"GIT_AUTHOR_EMAIL",
		"GIT_COMMITTER_EMAIL",
		"GIT_AUTHOR_NAME",
		"GIT_COMMITTER_NAME",
	} {
		assert.Equal(t, 1, envCount(env, key), "%s must appear exactly once", key)
	}
	assert.Equal(t, "real@profile", envLookup(env, "GIT_AUTHOR_EMAIL"))
	assert.Equal(t, "real@profile", envLookup(env, "GIT_COMMITTER_EMAIL"))
	assert.Equal(t, "Real Name", envLookup(env, "GIT_AUTHOR_NAME"))
	assert.Equal(t, "Real Name", envLookup(env, "GIT_COMMITTER_NAME"))
	assert.Contains(t, envLookup(env, "GIT_SSH_COMMAND"), "/tmp/k")
	assert.Equal(t, "/usr/bin", envLookup(env, "PATH"))
}

func TestBuildEnvCommitNameOmittedWhenUnset(t *testing.T) {
	profile := &identity.Profile{
		Name:   "test",
		SSHKey: "/tmp/k",
		Email:  "real@profile",
	}
	env := buildEnv(nil, profile, "/tmp/k", "")
	assert.Equal(t, 0, envCount(env, "GIT_AUTHOR_NAME"))
	assert.Equal(t, 0, envCount(env, "GIT_COMMITTER_NAME"))
}

func TestBuildEnvIncludesIdentityAgentWhenSocketGiven(t *testing.T) {
	profile := &identity.Profile{Name: "test", SSHKey: "/tmp/k", Email: "x@y"}
	env := buildEnv(nil, profile, "/tmp/k", "/tmp/agents/test.sock")
	cmd := envLookup(env, "GIT_SSH_COMMAND")
	assert.Contains(t, cmd, "IdentitiesOnly=yes")
	assert.Contains(t, cmd, "IdentityAgent=/tmp/agents/test.sock")
}

func TestBuildEnvOmitsIdentityAgentWhenSocketEmpty(t *testing.T) {
	profile := &identity.Profile{Name: "test", SSHKey: "/tmp/k", Email: "x@y"}
	env := buildEnv(nil, profile, "/tmp/k", "")
	cmd := envLookup(env, "GIT_SSH_COMMAND")
	assert.Contains(t, cmd, "IdentitiesOnly=yes")
	assert.NotContains(t, cmd, "IdentityAgent=")
}
