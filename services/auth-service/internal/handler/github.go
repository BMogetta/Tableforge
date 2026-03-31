package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	githubAuthURL  = "https://github.com/login/oauth/authorize"
	githubTokenURL = "https://github.com/login/oauth/access_token"
	githubUserURL  = "https://api.github.com/user"
	githubEmailURL = "https://api.github.com/user/emails"
)

type githubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func exchangeCode(ctx context.Context, clientID, clientSecret, code string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, githubTokenURL, nil)
	q := req.URL.Query()
	q.Set("client_id", clientID)
	q.Set("client_secret", clientSecret)
	q.Set("code", code)
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("github oauth error: %s", result.Error)
	}
	return result.AccessToken, nil
}

func fetchGitHubUser(ctx context.Context, token string) (githubUser, error) {
	var u githubUser
	if err := githubGET(ctx, token, githubUserURL, &u); err != nil {
		return u, err
	}
	return u, nil
}

func fetchPrimaryEmail(ctx context.Context, token string) (string, error) {
	var emails []githubEmail
	if err := githubGET(ctx, token, githubEmailURL, &emails); err != nil {
		return "", err
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	return "", fmt.Errorf("no verified primary email on GitHub account")
}

func githubGET(ctx context.Context, token, url string, dst any) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github API %s: status %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
