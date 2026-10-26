package k9o

import "context"

// SignInInput is the input shape for Gate.SignIn.
type SignInInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// SignUpInput is the input shape for Gate.SignUp.
type SignUpInput struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName,omitempty"`
}

// SignOutInput is the input shape for Gate.SignOut.
type SignOutInput struct {
	RefreshToken string `json:"refreshToken"`
}

// RefreshTokenInput is the input shape for Gate.RefreshToken.
type RefreshTokenInput struct {
	RefreshToken string `json:"refreshToken"`
}

// GateNamespace exposes client.Gate.* — Gate IAK auth surface.
// Requires ProjectSlug bound on the client (project-scoped).
type GateNamespace struct {
	config *httpClientConfig
}

func (g *GateNamespace) requireProject() (string, *KError) {
	if g.config.ProjectSlug == "" {
		return "", &KError{
			Code:    "K9O_MISSING_PROJECT_CONTEXT",
			Message: "client.Gate operations require ProjectSlug bound on the client",
			Context: map[string]any{},
		}
	}
	return g.config.ProjectSlug, nil
}

// SignUp creates a new end-user identity.
func (g *GateNamespace) SignUp(ctx context.Context, input SignUpInput) (*Envelope[any], *KError) {
	slug, kerr := g.requireProject()
	if kerr != nil {
		return nil, kerr
	}
	return doRequest[any](ctx, g.config, &requestInit{
		Method: "POST",
		Path:   ProjectPath(slug, "gate/signup"),
		Body:   input,
	})
}

// SignIn signs in an existing end-user with email + password.
func (g *GateNamespace) SignIn(ctx context.Context, input SignInInput) (*Envelope[any], *KError) {
	slug, kerr := g.requireProject()
	if kerr != nil {
		return nil, kerr
	}
	return doRequest[any](ctx, g.config, &requestInit{
		Method: "POST",
		Path:   ProjectPath(slug, "gate/signin"),
		Body:   input,
	})
}

// SignOut revokes the current session.
func (g *GateNamespace) SignOut(ctx context.Context, input SignOutInput) (*Envelope[any], *KError) {
	slug, kerr := g.requireProject()
	if kerr != nil {
		return nil, kerr
	}
	return doRequest[any](ctx, g.config, &requestInit{
		Method: "POST",
		Path:   ProjectPath(slug, "gate/signout"),
		Body:   input,
	})
}

// Whoami returns the active end-user identity.
func (g *GateNamespace) Whoami(ctx context.Context) (*Envelope[any], *KError) {
	slug, kerr := g.requireProject()
	if kerr != nil {
		return nil, kerr
	}
	return doRequest[any](ctx, g.config, &requestInit{
		Method: "GET",
		Path:   ProjectPath(slug, "gate/whoami"),
	})
}

// RefreshToken rotates the access token using the refresh token.
func (g *GateNamespace) RefreshToken(ctx context.Context, input RefreshTokenInput) (*Envelope[any], *KError) {
	slug, kerr := g.requireProject()
	if kerr != nil {
		return nil, kerr
	}
	return doRequest[any](ctx, g.config, &requestInit{
		Method: "POST",
		Path:   ProjectPath(slug, "gate/refresh"),
		Body:   input,
	})
}
