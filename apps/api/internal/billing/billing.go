// Package billing defines the entitlement contract for the trivia platform.
//
// The design uses an interface so that handlers never depend directly on Stripe.
// Today every call goes through NoopChecker (which always grants access).
// When billing is implemented, the only change required is swapping
// NoopChecker for a Stripe-backed implementation in main.go — no handler code
// needs to change.
//
// This is the "dependency inversion" principle: high-level business logic
// (handlers) depends on an abstraction (EntitlementChecker), not a concrete
// implementation (Stripe). In Go, interfaces are satisfied implicitly — any
// type that has the right methods is automatically considered to implement the
// interface, without needing to declare "implements" explicitly.
package billing

import "context"

// EntitlementChecker answers gate questions about what a host is allowed to do.
// Handlers call this before performing gated actions (e.g. creating a game).
type EntitlementChecker interface {
	// CanCreateGame returns true if the host is allowed to create a new game.
	// userID is our internal UUID (not the Auth0 sub).
	CanCreateGame(ctx context.Context, userID string) (bool, error)

	// CanUseFeature returns true if the host is allowed to use a named feature.
	// This is a catch-all gate for future premium features (e.g. "custom_branding").
	CanUseFeature(ctx context.Context, userID, feature string) (bool, error)
}

// NoopChecker is a pass-through implementation that always grants access.
// It is used until Stripe billing is implemented. Replacing it with a real
// implementation requires only a one-line change in main.go.
type NoopChecker struct{}

// var _ EntitlementChecker = (*NoopChecker)(nil) is a compile-time assertion.
// It tells the compiler "verify that *NoopChecker satisfies the EntitlementChecker
// interface". If NoopChecker is missing a method, this line produces a build
// error rather than a confusing runtime panic. The variable is blank (_) so
// it's never actually allocated.
var _ EntitlementChecker = (*NoopChecker)(nil)

// The underscore (_) parameter names below mean the parameters are intentionally
// unused — Go requires all declared variables to be used, so _ is the conventional
// way to acknowledge a parameter exists but isn't needed in this implementation.

func (NoopChecker) CanCreateGame(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (NoopChecker) CanUseFeature(_ context.Context, _, _ string) (bool, error) {
	return true, nil
}
