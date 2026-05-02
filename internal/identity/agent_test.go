//go:build unix

package identity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAgentPID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{
			name: "ssh-agent -s output (default for `ssh-agent -a`)",
			in: "SSH_AUTH_SOCK=/tmp/x.sock; export SSH_AUTH_SOCK;\n" +
				"SSH_AGENT_PID=12345; export SSH_AGENT_PID;\n" +
				"echo Agent pid 12345;\n",
			want: 12345,
		},
		{
			name: "PID line without trailing semicolon",
			in:   "SSH_AGENT_PID=42",
			want: 42,
		},
		{
			name: "missing PID line",
			in:   "no pid here\n",
			want: 0,
		},
		{
			name: "empty",
			in:   "",
			want: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseAgentPID(tc.in))
		})
	}
}

func TestCanonPubkey(t *testing.T) {
	assert.Equal(t,
		"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI...==",
		canonPubkey("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI...== user@host"))
	assert.Equal(t,
		"ssh-rsa AAAAB3...",
		canonPubkey("  ssh-rsa AAAAB3...  \n"))
	assert.Equal(t, "", canonPubkey(""))
	assert.Equal(t, "", canonPubkey("only-one-field"))
}

func TestAppendEnvDedupes(t *testing.T) {
	in := []string{"PATH=/usr/bin", "FOO=old", "BAR=keep"}
	got := appendEnv(in, "FOO=new")
	// FOO should appear exactly once, with the new value.
	count := 0
	var foo string
	for _, kv := range got {
		if len(kv) >= 4 && kv[:4] == "FOO=" {
			count++
			foo = kv[4:]
		}
	}
	assert.Equal(t, 1, count)
	assert.Equal(t, "new", foo)
	assert.Contains(t, got, "PATH=/usr/bin")
	assert.Contains(t, got, "BAR=keep")
}
