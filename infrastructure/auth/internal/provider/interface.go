package provider

import "context"

type ExternalIdentity struct {
	Provider string
	Sub      string
	Email    string
}

type AuthProvider interface {
	Validate(ctx context.Context, token string) (ExternalIdentity, error)
}
