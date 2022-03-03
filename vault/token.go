package vault

// Initial version from: authn-authz/customer-credential-service/blob/develop/pkg/vault/vault.go

import (
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/vault/api"
)

// TokenHandler automatically deals with the renewal of tokens used for authentication with the
// Vault API. It uses the AuthMethod to generate new tokens if required (e.g. if the current token
// is not renewable anymore).
type TokenHandler struct {
	client *Client
	method AuthMethod
	log    logr.Logger
	tokens chan string

	mu      sync.Mutex
	closed  bool
	renewer *api.Renewer
}

// NewTokenHandler creates a new TokenHandler.
func NewTokenHandler(c *Client, m AuthMethod) *TokenHandler {
	h := &TokenHandler{
		client: c,
		method: m,
		log:    c.log.WithName("TokenHandler"),
		tokens: make(chan string),
	}
	if h.method.IsRenewable() {
		go h.run()
	}
	return h
}

// WaitForToken blocks until a renewed token or the initial token has been received. It returns an
// error if no token is received before the timeout is reached.
func (h *TokenHandler) WaitForToken(timeout time.Duration) error {
	if h.method.IsRenewable() {
		select {
		case <-h.tokens:
		case <-time.After(timeout):
			return ErrTimeout
		}
	} else {
		secret, err := h.method.Login(h.client)
		if err != nil {
			return err
		}
		h.client.SetToken(secret.Auth.ClientToken)
	}
	return nil
}

// Close the token handler and stop the background renewal process.
func (h *TokenHandler) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.closed = true
	if h.renewer != nil {
		h.renewer.Stop()
	}
}

func (h *TokenHandler) run() {
	h.log.Info("Starting token renewal loop.")
	for {
		h.mu.Lock()
		if h.closed {
			h.mu.Unlock()
			break
		}
		h.mu.Unlock()

		secret, err := h.method.Login(h.client)
		if err != nil {
			h.log.Error(err, "Failed to request client token")
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if secret == nil {
			h.log.Info("Received empty token response. Retrying...")
			time.Sleep(500 * time.Millisecond)
			continue
		}

		token := secret.Auth.ClientToken
		h.log.V(2).Info("Setting client-token", "token", token)
		h.client.SetToken(token)

		select {
		case h.tokens <- token:
		default:
		}

		renewer, err := h.client.NewRenewer(&api.RenewerInput{Secret: secret})
		if err != nil {
			h.log.Error(err, "Failed to start token renewer.")
		}
		h.mu.Lock()
		h.renewer = renewer
		h.mu.Unlock()

		go h.renewer.Renew()
		h.monitorRenewal()
	}
}

func (h *TokenHandler) monitorRenewal() {
	for {
		select {
		case err := <-h.renewer.DoneCh():
			if err != nil {
				h.log.Error(err, "Vault token renewer returned error.")
			}
			return
		case result := <-h.renewer.RenewCh():
			h.log.V(2).Info(
				"Renewed Vault client token.",
				"token", result.Secret.Auth.ClientToken,
				"lease", time.Duration(result.Secret.Auth.LeaseDuration)*time.Second,
				"renewable", result.Secret.Auth.Renewable,
			)
		}
	}
}
