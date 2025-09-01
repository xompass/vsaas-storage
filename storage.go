package vsaasstorage

import (
	"context"
	"io"
	"time"
)

// StorageProvider defines the interface that all storage providers must implement
type StorageProvider interface {
	// File operations
	Upload(ctx context.Context, path string, reader io.Reader, metadata *FileMetadata) (*FileInfo, error)
	Download(ctx context.Context, path string) (io.ReadCloser, *FileInfo, error)
	Delete(ctx context.Context, path string) error
	Exists(ctx context.Context, path string) (bool, error)
	GetInfo(ctx context.Context, path string) (*FileInfo, error)

	// Directory operations
	List(ctx context.Context, path string) ([]*FileInfo, error)
	DeleteDirectory(ctx context.Context, path string) error

	// Advanced operations
	Copy(ctx context.Context, srcPath, dstPath string) error
	Move(ctx context.Context, srcPath, dstPath string) error

	// Signed URLs
	GenerateSignedURL(ctx context.Context, path string, operation SignedURLOperation, expiresIn time.Duration) (string, error)
}

// Storage is the main storage instance that wraps a provider
type Storage struct {
	provider StorageProvider
	config   *StorageConfig
}

// FileInfo contains information about a file
type FileInfo struct {
	Path         string            `json:"path"`
	Name         string            `json:"name"`
	Size         int64             `json:"size"`
	ContentType  string            `json:"content_type"`
	ETag         string            `json:"etag,omitempty"`
	LastModified *time.Time        `json:"last_modified,omitempty"`
	IsDirectory  bool              `json:"is_directory"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// FileMetadata contains metadata for file uploads
type FileMetadata struct {
	ContentType     string            `json:"content_type,omitempty"`
	CacheControl    string            `json:"cache_control,omitempty"`
	ContentEncoding string            `json:"content_encoding,omitempty"`
	CustomMetadata  map[string]string `json:"custom_metadata,omitempty"`
}

// SignedURLOperation defines the type of operation for signed URLs
type SignedURLOperation string

const (
	SignedURLOperationGet    SignedURLOperation = "GET"
	SignedURLOperationPut    SignedURLOperation = "PUT"
	SignedURLOperationDelete SignedURLOperation = "DELETE"
)

// New creates a new Storage instance with the given configuration
func New(config *StorageConfig) (*Storage, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	var provider StorageProvider
	var err error

	switch config.Provider {
	case "filesystem":
		provider, err = NewFileSystemProvider(config)
	case "s3":
		provider, err = NewS3Provider(config)
	default:
		return nil, &StorageError{
			Code:    ErrorCodeInvalidProvider,
			Message: "unsupported provider: " + config.Provider,
		}
	}

	if err != nil {
		return nil, err
	}

	return &Storage{
		provider: provider,
		config:   config,
	}, nil
}

// Upload uploads a file to the storage
func (s *Storage) Upload(ctx context.Context, path string, reader io.Reader, metadata *FileMetadata) (*FileInfo, error) {
	return s.provider.Upload(ctx, path, reader, metadata)
}

// Download downloads a file from the storage
func (s *Storage) Download(ctx context.Context, path string) (io.ReadCloser, *FileInfo, error) {
	return s.provider.Download(ctx, path)
}

// Delete deletes a file from the storage
func (s *Storage) Delete(ctx context.Context, path string) error {
	return s.provider.Delete(ctx, path)
}

// Exists checks if a file exists in the storage
func (s *Storage) Exists(ctx context.Context, path string) (bool, error) {
	return s.provider.Exists(ctx, path)
}

// GetInfo gets information about a file
func (s *Storage) GetInfo(ctx context.Context, path string) (*FileInfo, error) {
	return s.provider.GetInfo(ctx, path)
}

// List lists files in a directory
func (s *Storage) List(ctx context.Context, path string) ([]*FileInfo, error) {
	return s.provider.List(ctx, path)
}

// DeleteDirectory deletes a directory and all its contents recursively
func (s *Storage) DeleteDirectory(ctx context.Context, path string) error {
	return s.provider.DeleteDirectory(ctx, path)
}

// Copy copies a file from source to destination
func (s *Storage) Copy(ctx context.Context, srcPath, dstPath string) error {
	return s.provider.Copy(ctx, srcPath, dstPath)
}

// Move moves a file from source to destination
func (s *Storage) Move(ctx context.Context, srcPath, dstPath string) error {
	return s.provider.Move(ctx, srcPath, dstPath)
}

// GenerateSignedURL generates a signed URL for the given operation
func (s *Storage) GenerateSignedURL(ctx context.Context, path string, operation SignedURLOperation, expiresIn time.Duration) (string, error) {
	return s.provider.GenerateSignedURL(ctx, path, operation, expiresIn)
}

// GetConfig returns the storage configuration
func (s *Storage) GetConfig() *StorageConfig {
	return s.config
}
