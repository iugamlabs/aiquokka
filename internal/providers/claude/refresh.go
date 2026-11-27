package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/iugamlabs/aiquokka/internal/credential"
	"github.com/iugamlabs/aiquokka/internal/httpx"
)

// claudeCodeClientID is Claude Code's public OAuth client id.
const claudeCodeClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"

// refreshEndpoints are tried in order; Anthropic migrated from console to platform.
var refreshEndpoints = []string{
	"https://platform.claude.com/v1/oauth/token",
	"https://console.anthropic.com/v1/oauth/token",
}

type refreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// ensureFresh refreshes the OAuth token according to the active credential
// policy when the access token is expired (or force is set).
//
// A Keychain credential is never refreshed here: Claude Code refreshes that
// same item while sessions run, so a write-back could rotate the token out
// from under it. An expired one answers as readonly does.
func ensureFresh(ctx context.Context, o *oauth, force bool) error {
	if !force && !o.expired(time.Now()) {
		return nil
	}
	if o.source.isKeychain() {
		return credential.Apply("Claude", credential.ReadOnly, capabilities(), credential.RefreshFuncs{})
	}
	policy := credential.PolicyFrom(ctx)
	return credential.Apply("Claude", policy, capabilities(), credential.RefreshFuncs{
		Persist: func() error {
			return refreshAndPersist(ctx, o)
		},
	})
}

func refreshAndPersist(ctx context.Context, o *oauth) error {
	if err := refreshInPlace(ctx, o); err != nil {
		return err
	}
	return persist(ctx, o)
}

// refreshInPlace exchanges the refresh token and updates o in memory only.
func refreshInPlace(ctx context.Context, o *oauth) error {
	if o.RefreshToken == "" {
		return fmt.Errorf("access token expired and no refresh token available — run `claude` to re-login")
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {o.RefreshToken},
		"client_id":     {claudeCodeClientID},
	}

	var errs []string
	for _, endpoint := range refreshEndpoints {
		rr, err := doRefresh(ctx, endpoint, form)
		if err != nil {
			errs = append(errs, endpoint+": "+err.Error())
			continue
		}
		o.AccessToken = rr.AccessToken
		if rr.RefreshToken != "" {
			o.RefreshToken = rr.RefreshToken
		}
		if rr.ExpiresIn > 0 {
			o.ExpiresAt = time.Now().Add(time.Duration(rr.ExpiresIn) * time.Second).UnixMilli()
		}
		return nil
	}
	return fmt.Errorf("refreshing Claude token: %s", strings.Join(errs, "; "))
}

func doRefresh(ctx context.Context, endpoint string, form url.Values) (*refreshResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := httpx.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var rr refreshResponse
	if err := json.Unmarshal(body, &rr); err != nil {
		return nil, err
	}
	if rr.AccessToken == "" {
		return nil, fmt.Errorf("refresh returned no access token")
	}
	return &rr, nil
}

// persist writes the updated oauth block back into the credentials file,
// preserving any other keys already present. Keychain credentials are
// read-only: Claude Code refreshes that item itself.
func persist(_ context.Context, o *oauth) error {
	if o.source.isKeychain() {
		return fmt.Errorf("Claude Keychain credentials are read-only — run `claude` to re-login")
	}

	path, err := credentialsPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, err := mergeCredentials(data, false, o)
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return credential.WriteFileAtomic(path, out, 0o600)
}
