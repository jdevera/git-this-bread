//go:build !unix

package identity

import "errors"

// SubAgent is a stub on non-unix platforms; the sub-agent feature relies on
// ssh-agent's UNIX socket model.
type SubAgent struct {
	Profile  *Profile
	Socket   string
	PIDFile  string
	LockFile string
}

var errNotSupported = errors.New("per-profile ssh sub-agent is not supported on this platform; set usecustomagent = false on the profile")

func Ensure(p *Profile) (*SubAgent, error)   { return nil, errNotSupported }
func ListAgents() ([]*SubAgent, error)       { return nil, nil }
func (s *SubAgent) IsAlive() bool            { return false }
func (s *SubAgent) HasKey() (bool, error)    { return false, errNotSupported }
func (s *SubAgent) LoadKey() error           { return errNotSupported }
func (s *SubAgent) Kill() error              { return errNotSupported }
func (s *SubAgent) PID() int                 { return 0 }
func (s *SubAgent) LoadedKeys() ([]string, error) {
	return nil, errNotSupported
}
