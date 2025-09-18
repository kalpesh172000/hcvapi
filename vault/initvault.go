package vault

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/sirupsen/logrus"
	"github.com/kalpesh172000/hcvapi/config"
)

type Client struct {
	client *api.Client
	config *config.Config
	logger *logrus.Logger
}

type TokenResponse struct {
	Token             string `json:"token"`
	TokenTTL          string `json:"token_ttl"`
	ExpiresAtSeconds  int64  `json:"expires_at_seconds"`
}

type ServiceAccountKeyResponse struct {
	PrivateKeyData string `json:"private_key_data"`
	KeyAlgorithm   string `json:"key_algorithm"`
	KeyType        string `json:"key_type"`
	KeyID          string `json:"key_id"`
}

type RolesetRequest struct {
	Project       string            `json:"project" binding:"required"`
	SecretType    string            `json:"secret_type" binding:"required,oneof=access_token service_account_key"`
	TokenScopes   string            `json:"token_scopes,omitempty"`
	Bindings      map[string]interface{} `json:"bindings"`
	TTL           string            `json:"ttl,omitempty"`
	MaxTTL        string            `json:"max_ttl,omitempty"`
}

func NewClient(cfg *config.Config, logger *logrus.Logger) (*Client, error) {
	vaultCfg := api.DefaultConfig()
	vaultCfg.Address = cfg.Vault.Address

	if cfg.Vault.SkipVerify {
		err := vaultCfg.ConfigureTLS(&api.TLSConfig{
			Insecure: true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to configure TLS: %w", err)
		}
	}

	client, err := api.NewClient(vaultCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %w", err)
	}

	// Set token
	client.SetToken(cfg.Vault.Token)

	// Set namespace if provided
	if cfg.Vault.Namespace != "" {
		client.SetNamespace(cfg.Vault.Namespace)
	}

	return &Client{
		client: client,
		config: cfg,
		logger: logger,
	}, nil
}

func (c *Client) Initialize(ctx context.Context) error {
	c.logger.Info("Initializing Vault GCP secrets engine...")

	// Check if GCP secrets engine is enabled
	mounts, err := c.client.Sys().ListMounts()
	if err != nil {
		return fmt.Errorf("failed to list mounts: %w", err)
	}

	gcpMountExists := false
	for path := range mounts {
		if strings.TrimSuffix(path, "/") == "gcp" {
			gcpMountExists = true
			break
		}
	}

	// Enable GCP secrets engine if not exists
	if !gcpMountExists {
		c.logger.Info("Enabling GCP secrets engine...")
		err := c.client.Sys().Mount("gcp", &api.MountInput{
			Type:        "gcp",
			Description: "GCP secrets engine for managing access tokens and service account keys",
		})
		if err != nil {
			return fmt.Errorf("failed to enable GCP secrets engine: %w", err)
		}
		c.logger.Info("GCP secrets engine enabled successfully")
	}

	// Configure GCP secrets engine
	if err := c.configureGCPEngine(ctx); err != nil {
		return fmt.Errorf("failed to configure GCP engine: %w", err)
	}

	c.logger.Info("Vault GCP secrets engine initialized successfully")
	return nil
}

func (c *Client) configureGCPEngine(ctx context.Context) error {
	c.logger.Info("Configuring GCP secrets engine...")

	configData := map[string]interface{}{
		"ttl":                         c.config.GCP.DefaultTTL,
		"max_ttl":                     c.config.GCP.MaxTTL,
		"disable_automated_rotation":  c.config.GCP.DisableAutomatedRotation,
	}

	// If service account path is provided, read and set credentials
	if c.config.GCP.ServiceAccountPath != "" {
		credentials, err := ioutil.ReadFile(c.config.GCP.ServiceAccountPath)
		if err != nil {
			return fmt.Errorf("failed to read service account file: %w", err)
		}
		configData["credentials"] = string(credentials)
	}

	_, err := c.client.Logical().WriteWithContext(ctx, "gcp/config", configData)
	if err != nil {
		return fmt.Errorf("failed to configure GCP engine: %w", err)
	}

	c.logger.Info("GCP secrets engine configured successfully")
	return nil
}

func (c *Client) CreateRoleset(ctx context.Context, name string, req *RolesetRequest) error {
	c.logger.WithField("roleset", name).Info("Creating GCP roleset...")

	data := map[string]interface{}{
		"project":     req.Project,
		"secret_type": req.SecretType,
	}

	if req.TokenScopes != "" {
		data["token_scopes"] = req.TokenScopes
	} else if req.SecretType == "access_token" {
		data["token_scopes"] = c.config.GCP.DefaultTokenScopes
	}

	if req.Bindings != nil && len(req.Bindings) > 0 {
		data["bindings"] = req.Bindings
	}

	if req.TTL != "" {
		data["ttl"] = req.TTL
	}

	if req.MaxTTL != "" {
		data["max_ttl"] = req.MaxTTL
	}

	_, err := c.client.Logical().WriteWithContext(ctx, fmt.Sprintf("gcp/roleset/%s", name), data)
	if err != nil {
		return fmt.Errorf("failed to create roleset: %w", err)
	}

	c.logger.WithField("roleset", name).Info("GCP roleset created successfully")
	return nil
}

func (c *Client) GetToken(ctx context.Context, rolesetName string, ttl string) (*TokenResponse, error) {
	c.logger.WithField("roleset", rolesetName).Info("Generating GCP access token...")

	var data map[string]interface{}
	if ttl != "" {
		data = map[string]interface{}{
			"ttl": ttl,
		}
	}

	var secret *api.Secret
	var err error

	if data != nil {
		secret, err = c.client.Logical().WriteWithContext(ctx, fmt.Sprintf("gcp/token/%s", rolesetName), data)
	} else {
		secret, err = c.client.Logical().ReadWithContext(ctx, fmt.Sprintf("gcp/token/%s", rolesetName))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("no token data returned")
	}

	response := &TokenResponse{
		Token:            secret.Data["token"].(string),
		TokenTTL:         secret.Data["token_ttl"].(string),
		ExpiresAtSeconds: int64(secret.Data["expires_at_seconds"].(float64)),
	}

	c.logger.WithField("roleset", rolesetName).Info("GCP access token generated successfully")
	return response, nil
}

func (c *Client) GetServiceAccountKey(ctx context.Context, rolesetName string) (*ServiceAccountKeyResponse, error) {
	c.logger.WithField("roleset", rolesetName).Info("Generating GCP service account key...")

	secret, err := c.client.Logical().ReadWithContext(ctx, fmt.Sprintf("gcp/key/%s", rolesetName))
	if err != nil {
		return nil, fmt.Errorf("failed to get service account key: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("no key data returned")
	}

	response := &ServiceAccountKeyResponse{
		PrivateKeyData: secret.Data["private_key_data"].(string),
		KeyAlgorithm:   secret.Data["key_algorithm"].(string),
		KeyType:        secret.Data["key_type"].(string),
		KeyID:          secret.Data["key_id"].(string),
	}

	c.logger.WithField("roleset", rolesetName).Info("GCP service account key generated successfully")
	return response, nil
}

func (c *Client) ListRolesets(ctx context.Context) ([]string, error) {
	c.logger.Info("Listing GCP rolesets...")

	secret, err := c.client.Logical().ListWithContext(ctx, "gcp/roleset")
	if err != nil {
		return nil, fmt.Errorf("failed to list rolesets: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return []string{}, nil
	}

	keys, ok := secret.Data["keys"].([]interface{})
	if !ok {
		return []string{}, nil
	}

	rolesets := make([]string, len(keys))
	for i, key := range keys {
		rolesets[i] = key.(string)
	}

	return rolesets, nil
}

func (c *Client) DeleteRoleset(ctx context.Context, name string) error {
	c.logger.WithField("roleset", name).Info("Deleting GCP roleset...")

	_, err := c.client.Logical().DeleteWithContext(ctx, fmt.Sprintf("gcp/roleset/%s", name))
	if err != nil {
		return fmt.Errorf("failed to delete roleset: %w", err)
	}

	c.logger.WithField("roleset", name).Info("GCP roleset deleted successfully")
	return nil
}

func (c *Client) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	health, err := c.client.Sys().HealthWithContext(ctx)
	if err != nil {
		return fmt.Errorf("vault health check failed: %w", err)
	}

	if !health.Initialized || health.Sealed {
		return fmt.Errorf("vault is not ready: initialized=%v, sealed=%v", health.Initialized, health.Sealed)
	}

	return nil
}
