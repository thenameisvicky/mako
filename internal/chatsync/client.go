package chatsync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/thenameisvicky/tokenly-mako/internal/keychain"
	"github.com/thenameisvicky/tokenly-mako/pkg/models"
)

// Client talks to the proxy's chat sync endpoints as the logged-in user.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewClient builds a client using the Mako token from the system keychain.
func NewClient(baseURL string) (*Client, error) {
	_, token, err := keychain.Load()
	if err != nil || token == "" {
		return nil, fmt.Errorf("you are not logged in. Run /login first")
	}

	return &Client{
		baseURL: baseURL,
		token:   token,
		// Transcripts can be large, so allow a generous but bounded upload window.
		http: &http.Client{Timeout: 2 * time.Minute},
	}, nil
}

// Sync uploads transcripts to the proxy.
func (c *Client) Sync(sessions []models.ChatSessionPayload) (*models.ChatSyncResponse, error) {
	payload, err := json.Marshal(models.ChatSyncRequest{Sessions: sessions})
	if err != nil {
		return nil, fmt.Errorf("failed to encode sessions: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/v1/chats/sync", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	var out models.ChatSyncResponse
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	if out.Status != "success" {
		return nil, fmt.Errorf("proxy rejected the sync: %s", out.Message)
	}
	return &out, nil
}

// List fetches transcript metadata visible to the caller's team.
func (c *Client) List() ([]models.ChatSessionSummary, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/v1/chats", nil)
	if err != nil {
		return nil, err
	}

	var out models.ChatListResponse
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	if out.Status != "success" {
		return nil, fmt.Errorf("proxy error: %s", out.Message)
	}
	return out.Sessions, nil
}

// Get downloads one full transcript by session id.
func (c *Client) Get(sessionID string) (*models.ChatSessionPayload, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/v1/chats/"+url.PathEscape(sessionID), nil)
	if err != nil {
		return nil, err
	}

	var out models.ChatGetResponse
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	if out.Status != "success" {
		return nil, fmt.Errorf("proxy error: %s", out.Message)
	}
	return &out.Session, nil
}

// do sends a request carrying the Mako token and decodes the JSON response.
func (c *Client) do(req *http.Request, out interface{}) error {
	req.Header.Set("X-Mako-Token", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach the proxy at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("proxy returned an unreadable response (HTTP %d)", resp.StatusCode)
	}
	return nil
}
