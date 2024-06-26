package clients

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
	"github.com/sirupsen/logrus"
)

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	ExpiresIn   int    `json:"expires_in"`
	ExpiresAt   time.Time
}

const (
	scopes    = "openid api.iam.service_accounts"
	grantType = "client_credentials"
)

var token tokenResponse
var tokenMu sync.Mutex

// currentToken returns the current token or fetches a new one if the current one is expired.
func currentToken(ctx context.Context, issuerURL, clientID, clientSecret string) (string, error) {
	tokenMu.Lock()
	defer tokenMu.Unlock()

	if token.AccessToken == "" || time.Until(token.ExpiresAt) < 30*time.Second {
		t, err := getToken(ctx, issuerURL, clientID, clientSecret)
		if err != nil {
			return "", err
		}
		logrus.WithContext(ctx).Debugf("Acquired oauth2 token which expires in %s", time.Until(token.ExpiresAt).String())
		token = t
	} else {
		logrus.WithContext(ctx).Debugf("Reused oauth2 token which expires in %s", time.Until(token.ExpiresAt).String())
	}

	return token.AccessToken, nil
}

// getToken fetches a new token from the issuer. Use currentToken instead of this function directly.
func getToken(ctx context.Context, issuerURL, clientID, clientSecret string) (tokenResponse, error) {
	data := url.Values{}
	data.Add("grant_type", grantType)
	data.Add("scope", scopes)
	data.Add("client_id", clientID)
	data.Add("client_secret", clientSecret)
	encoded := data.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, issuerURL, strings.NewReader(encoded))
	if err != nil {
		return tokenResponse{}, fmt.Errorf("failed to form request: %w", err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(encoded)))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("failed to request a token: %w", err)
	}
	defer res.Body.Close()

	tr := tokenResponse{}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("failed to parse a token response: %w", err)
	}

	err = json.Unmarshal(body, &tr)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("failed to parse a token response: %w", err)
	}

	tr.ExpiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return tr, nil
}

func AddOAuth2IdentityHeader(ctx context.Context, req *http.Request, issuerURL, clientID, clientSecret string) error {
	if issuerURL == "" || clientID == "" || clientSecret == "" {
		logrus.WithContext(ctx).Warning("Service account authentication is not configured")
		return nil
	}

	logrus.WithContext(ctx).Debugf("Using service account authentication: %s", clientID)
	token, err := currentToken(ctx, issuerURL, clientID, clientSecret)
	if err != nil {
		logrus.WithContext(ctx).WithField("err", err).Error("Fetching service account access token failed")
		return err
	}
	req.Header.Add("Authorization", "Bearer "+token)

	return nil
}

func AddServiceMockIdentityHeader(_ context.Context, req *http.Request) error {
	id := identity.XRHID{Identity: identity.Identity{
		User:           &identity.User{Username: config.Get().PulpIdentityName},
		ServiceAccount: &identity.ServiceAccount{Username: config.Get().PulpIdentityName},
	}}
	identityHeaders, err := json.Marshal(id)
	if err != nil {
		return err
	}

	req.Header.Add("X-Rh-Identity", base64.StdEncoding.EncodeToString(identityHeaders))

	return nil
}

func AddBasicCredentialsHeader(_ context.Context, req *http.Request) {
	req.SetBasicAuth(config.Get().PulpUsername, config.Get().PulpPassword)
}
