package vsaasstorage

import (
	"errors"
	"time"
)

// StorageConfig represents the unified configuration for all storage providers
type StorageConfig struct {
	Name       string            `json:"name"`
	Provider   string            `json:"provider"` // "filesystem", "s3"
	FileSystem *FileSystemConfig `json:"filesystem,omitempty"`
	S3         *S3Config         `json:"s3,omitempty"`
	SignedURL  *SignedURLConfig  `json:"signedUrl,omitempty"`
}

// FileSystemConfig contains configuration for filesystem provider
type FileSystemConfig struct {
	BasePath    string `json:"basePath"`    // Base directory path
	CreateDirs  bool   `json:"createDirs"`  // Automatically create directories
	Permissions string `json:"permissions"` // File permissions (e.g., "0755")
}

// S3Config contains configuration for S3 provider
type S3Config struct {
	Region              string                 `json:"region"`
	Endpoint            string                 `json:"endpoint,omitempty"` // Custom endpoint (for MinIO, etc.)
	Bucket              string                 `json:"bucket"`
	AccessKeyID         string                 `json:"accessKeyId"`
	SecretAccessKey     string                 `json:"secretAccessKey"`
	SessionToken        string                 `json:"sessionToken,omitempty"`        // For temporary credentials
	UseSSL              bool                   `json:"useSSL"`                        // true for https, false for http
	ForcePathStyle      bool                   `json:"forcePathStyle"`                // Force path-style addressing
	DefaultUploadParams map[string]interface{} `json:"defaultUploadParams,omitempty"` // Default parameters for uploads
	MaxRetries          int                    `json:"maxRetries"`
	HTTPOptions         *HTTPOptions           `json:"httpOptions,omitempty"`
}

// HTTPOptions contains HTTP-specific options
type HTTPOptions struct {
	Timeout   int         `json:"timeout"`   // Timeout in milliseconds
	KeepAlive bool        `json:"keepAlive"` // Keep-alive connections
	Agent     interface{} `json:"agent,omitempty"`
}

// SignedURLConfig contains configuration for signed URLs
type SignedURLConfig struct {
	Enabled   bool          `json:"enabled"`
	ExpiresIn time.Duration `json:"expiresIn"` // Default expiration time
	SecretKey string        `json:"secretKey"` // Secret key for JWT signing (filesystem)
}

// Validate validates the storage configuration
func (c *StorageConfig) Validate() error {
	if c.Name == "" {
		return errors.New("storage name is required")
	}

	if c.Provider == "" {
		return errors.New("provider is required")
	}

	switch c.Provider {
	case "filesystem":
		if c.FileSystem == nil {
			return errors.New("filesystem configuration is required when provider is filesystem")
		}
		return c.FileSystem.Validate()
	case "s3":
		if c.S3 == nil {
			return errors.New("s3 configuration is required when provider is s3")
		}
		return c.S3.Validate()
	default:
		return errors.New("unsupported provider: " + c.Provider)
	}
}

// Validate validates the filesystem configuration
func (c *FileSystemConfig) Validate() error {
	if c.BasePath == "" {
		return errors.New("basePath is required for filesystem provider")
	}
	return nil
}

// Validate validates the S3 configuration
func (c *S3Config) Validate() error {
	if c.Region == "" {
		return errors.New("region is required for s3 provider")
	}
	if c.Bucket == "" {
		return errors.New("bucket is required for s3 provider")
	}
	if c.AccessKeyID == "" {
		return errors.New("accessKeyId is required for s3 provider")
	}
	if c.SecretAccessKey == "" {
		return errors.New("secretAccessKey is required for s3 provider")
	}
	return nil
}

// GetSignedURLConfig returns the signed URL configuration with defaults
func (c *StorageConfig) GetSignedURLConfig() *SignedURLConfig {
	if c.SignedURL == nil {
		return &SignedURLConfig{
			Enabled:   false,
			ExpiresIn: 30 * time.Minute,
		}
	}

	config := *c.SignedURL
	if config.ExpiresIn == 0 {
		config.ExpiresIn = 30 * time.Minute
	}

	return &config
}
