package nbe

import "context"

// Client defines the contract for checking the NBE credit registry (CRB).
type Client interface {
	// IsBlacklisted checks whether the given Fayda ID is currently in default
	// with any Ethiopian bank. Returns true if blacklisted.
	IsBlacklisted(ctx context.Context, faydaID string) (bool, error)
}

// StubClient is a development placeholder. In production, this hits the
// NBE/EthSwitch credit registry over mTLS.
type StubClient struct{}

func NewStubClient() Client { return &StubClient{} }

func (s *StubClient) IsBlacklisted(_ context.Context, _ string) (bool, error) {
	return false, nil
}

var _ Client = (*StubClient)(nil)
