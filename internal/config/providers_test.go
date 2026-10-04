package config

import "testing"

// A secret resolved from a secret store turns captcha on as surely as one in the environment.
func TestCaptchaProviderFollowsTheResolvedSecret(t *testing.T) {
	t.Setenv("CAPTCHA_PROVIDER", "")
	t.Setenv("TURNSTILE_SECRET", "")
	t.Cleanup(func() { SetTurnstileSecretResolved(false) })

	SetTurnstileSecretResolved(false)
	if got := CaptchaProvider(); got != "none" {
		t.Fatalf("no secret anywhere: got %q, want none", got)
	}
	SetTurnstileSecretResolved(true)
	if got := CaptchaProvider(); got != "turnstile" {
		t.Fatalf("secret from a secret store: got %q, want turnstile", got)
	}
}

func TestMailvendorSandboxURLOnlyInDevelopment(t *testing.T) {
	t.Setenv("MAILVENDOR_SANDBOX_URL", "http://127.0.0.1:18099/")
	for env, want := range map[string]string{"": "http://127.0.0.1:18099", "dev": "http://127.0.0.1:18099", "prod": "", "production": "", "staging": ""} {
		t.Setenv("APP_ENV", env)
		if got := MailvendorSandboxURL(); got != want {
			t.Errorf("APP_ENV=%q: got %q, want %q", env, got, want)
		}
	}
}
