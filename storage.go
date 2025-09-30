package vsaasstorage

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	rest "github.com/xompass/vsaas-rest"
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

// UploadedFileResult represents the result of uploading a file
type UploadedFileResult struct {
	FieldName    string     `json:"field_name"`
	OriginalName string     `json:"original_name"`
	Filename     string     `json:"filename"`
	Path         string     `json:"path"`
	Size         int64      `json:"size"`
	ContentType  string     `json:"content_type"`
	ETag         string     `json:"etag,omitempty"`
	LastModified *time.Time `json:"last_modified,omitempty"`
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

// generateUniqueFilename generates a unique filename to avoid conflicts
func generateUniqueFilename(originalFilename string) string {
	// Get file extension
	ext := filepath.Ext(originalFilename)
	nameWithoutExt := strings.TrimSuffix(originalFilename, ext)

	// Generate a short unique identifier (8 characters)
	uniqueID := make([]byte, 4)
	rand.Read(uniqueID)
	uniqueStr := fmt.Sprintf("%x", uniqueID)

	// Combine: originalname_uniqueid.ext
	if ext != "" {
		return fmt.Sprintf("%s_%s%s", nameWithoutExt, uniqueStr, ext)
	}
	return fmt.Sprintf("%s_%s", nameWithoutExt, uniqueStr)
}

// UploadFromCtx processes file uploads from a vsaas-rest context and uploads them to the specified destination directory
func (s *Storage) UploadFromCtx(ctx context.Context, c *rest.EndpointContext, destinationDir string, destinationFilename ...string) ([]*UploadedFileResult, error) {
	// Check if there are uploaded files
	allFiles := c.GetAllUploadedFiles()
	if len(allFiles) == 0 {
		return nil, NewStorageError(ErrorCodeUploadFailed, "No files uploaded")
	}

	var results []*UploadedFileResult

	// Process each uploaded file
	for fieldName, files := range allFiles {
		for _, uploadedFile := range files {
			result, err := s.UploadFromUploadedFile(ctx, uploadedFile, fieldName, destinationDir, destinationFilename...)
			if err != nil {
				return nil, err
			}
			results = append(results, result)
		}
	}

	return results, nil
}

// UploadFromUploadedFile processes a single uploaded file and uploads it to the specified destination directory
func (s *Storage) UploadFromUploadedFile(ctx context.Context, uploadedFile *rest.UploadedFile, fieldName, destinationDir string, destinationFileName ...string) (*UploadedFileResult, error) {
	// Generate unique filename to avoid conflicts

	fileName := ""
	if len(destinationFileName) > 0 && destinationFileName[0] != "" {
		ext := filepath.Ext(uploadedFile.Filename)
		fileName = destinationFileName[0] + ext
	} else {
		fileName = generateUniqueFilename(uploadedFile.Filename)
	}

	// Construct the full file path with unique filename
	filePath := fmt.Sprintf("%s/%s", strings.TrimSuffix(destinationDir, "/"), fileName)

	// Open the uploaded file
	fileReader, err := os.Open(uploadedFile.Path)
	if err != nil {
		return nil, NewStorageErrorWithCause(ErrorCodeUploadFailed, "Failed to open uploaded file", err)
	}
	defer fileReader.Close()

	// Prepare metadata
	metadata := &FileMetadata{
		ContentType: uploadedFile.MimeType,
	}

	// Upload to storage
	fileInfo, err := s.Upload(ctx, filePath, fileReader, metadata)
	if err != nil {
		return nil, err
	}

	// Create result structure
	result := &UploadedFileResult{
		FieldName:    fieldName,
		OriginalName: uploadedFile.OriginalName,
		Filename:     fileName, // Use the unique filename generated
		Path:         fileInfo.Path,
		Size:         fileInfo.Size,
		ContentType:  fileInfo.ContentType,
		ETag:         fileInfo.ETag,
		LastModified: fileInfo.LastModified,
	}

	return result, nil
}

// StreamFile streams a file directly to the HTTP response, handling signed URLs, tokens, and direct downloads
func (s *Storage) StreamFile(c *rest.EndpointContext, path string) error {
	// Check for token validation (signed URL access)
	if token := c.EchoCtx.QueryParam("token"); token != "" {
		return s.handleTokenDownload(c, path, token)
	}

	// Check for signed URL request
	if c.EchoCtx.QueryParam("signed_url") == "true" {
		return s.handleSignedURLRequest(c, path)
	}

	// Regular download
	return s.handleDirectDownload(c, path)
}
