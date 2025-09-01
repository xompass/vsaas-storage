package vsaasstorage

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// FileSystemProvider implements the StorageProvider interface for local filesystem
type FileSystemProvider struct {
	config *StorageConfig
}

// NewFileSystemProvider creates a new filesystem provider
func NewFileSystemProvider(config *StorageConfig) (*FileSystemProvider, error) {
	if config.FileSystem == nil {
		return nil, NewStorageError(ErrorCodeInvalidConfig, "filesystem configuration is required")
	}

	// Create base directory if it doesn't exist and createDirs is true
	if config.FileSystem.CreateDirs {
		if err := os.MkdirAll(config.FileSystem.BasePath, 0755); err != nil {
			return nil, NewStorageErrorWithCause(ErrorCodeInternalError, "failed to create base directory", err)
		}
	}

	return &FileSystemProvider{
		config: config,
	}, nil
}

// Upload uploads a file to the filesystem
func (p *FileSystemProvider) Upload(ctx context.Context, path string, reader io.Reader, metadata *FileMetadata) (*FileInfo, error) {
	fullPath, err := p.getFullPath(path)
	if err != nil {
		return nil, err
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, NewProviderError("filesystem", ErrorCodeUploadFailed, "failed to create directory", err)
	}

	// Create the file
	file, err := os.Create(fullPath)
	if err != nil {
		return nil, NewProviderError("filesystem", ErrorCodeUploadFailed, "failed to create file", err)
	}
	defer file.Close()

	// Copy data and calculate size and hash
	hash := md5.New()
	size, err := io.Copy(io.MultiWriter(file, hash), reader)
	if err != nil {
		os.Remove(fullPath) // Clean up on error
		return nil, NewProviderError("filesystem", ErrorCodeUploadFailed, "failed to write file", err)
	}

	// Set file permissions if specified
	if p.config.FileSystem.Permissions != "" {
		if perm, err := strconv.ParseUint(p.config.FileSystem.Permissions, 8, 32); err == nil {
			os.Chmod(fullPath, os.FileMode(perm))
		}
	}

	// Get file info
	stat, err := file.Stat()
	if err != nil {
		return nil, NewProviderError("filesystem", ErrorCodeInternalError, "failed to get file stats", err)
	}

	// Determine content type
	contentType := "application/octet-stream"
	if metadata != nil && metadata.ContentType != "" {
		contentType = metadata.ContentType
	} else {
		contentType = mime.TypeByExtension(filepath.Ext(path))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}

	modTime := stat.ModTime()
	return &FileInfo{
		Path:         path,
		Name:         filepath.Base(path),
		Size:         size,
		ContentType:  contentType,
		ETag:         fmt.Sprintf("%x", hash.Sum(nil)),
		LastModified: &modTime,
		IsDirectory:  false,
	}, nil
}

// Download downloads a file from the filesystem
func (p *FileSystemProvider) Download(ctx context.Context, path string) (io.ReadCloser, *FileInfo, error) {
	fullPath, err := p.getFullPath(path)
	if err != nil {
		return nil, nil, err
	}

	// Check if file exists
	stat, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, FileNotFoundError(path)
		}
		return nil, nil, NewProviderError("filesystem", ErrorCodeDownloadFailed, "failed to stat file", err)
	}

	if stat.IsDir() {
		return nil, nil, NewStorageErrorWithPath(ErrorCodeInvalidPath, "path is a directory", path)
	}

	// Open file
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, nil, NewProviderError("filesystem", ErrorCodeDownloadFailed, "failed to open file", err)
	}

	// Get content type
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	modTime := stat.ModTime()
	fileInfo := &FileInfo{
		Path:         path,
		Name:         filepath.Base(path),
		Size:         stat.Size(),
		ContentType:  contentType,
		LastModified: &modTime,
		IsDirectory:  false,
	}

	return file, fileInfo, nil
}

// Delete deletes a file from the filesystem
func (p *FileSystemProvider) Delete(ctx context.Context, path string) error {
	fullPath, err := p.getFullPath(path)
	if err != nil {
		return err
	}

	// Check if file exists
	if _, err := os.Stat(fullPath); err != nil {
		if os.IsNotExist(err) {
			return FileNotFoundError(path)
		}
		return NewProviderError("filesystem", ErrorCodeDeleteFailed, "failed to stat file", err)
	}

	// Delete file
	if err := os.Remove(fullPath); err != nil {
		return NewProviderError("filesystem", ErrorCodeDeleteFailed, "failed to delete file", err)
	}

	return nil
}

// Exists checks if a file exists in the filesystem
func (p *FileSystemProvider) Exists(ctx context.Context, path string) (bool, error) {
	fullPath, err := p.getFullPath(path)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, NewProviderError("filesystem", ErrorCodeInternalError, "failed to check file existence", err)
	}

	return true, nil
}

// GetInfo gets information about a file
func (p *FileSystemProvider) GetInfo(ctx context.Context, path string) (*FileInfo, error) {
	fullPath, err := p.getFullPath(path)
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, FileNotFoundError(path)
		}
		return nil, NewProviderError("filesystem", ErrorCodeInternalError, "failed to get file info", err)
	}

	contentType := "application/octet-stream"
	if !stat.IsDir() {
		contentType = mime.TypeByExtension(filepath.Ext(path))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}

	modTime := stat.ModTime()
	return &FileInfo{
		Path:         path,
		Name:         filepath.Base(path),
		Size:         stat.Size(),
		ContentType:  contentType,
		LastModified: &modTime,
		IsDirectory:  stat.IsDir(),
	}, nil
}

// List lists files in a directory
func (p *FileSystemProvider) List(ctx context.Context, path string) ([]*FileInfo, error) {
	fullPath, err := p.getFullPath(path)
	if err != nil {
		return nil, err
	}

	// Check if directory exists
	stat, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, DirectoryNotFoundError(path)
		}
		return nil, NewProviderError("filesystem", ErrorCodeListFailed, "failed to stat directory", err)
	}

	if !stat.IsDir() {
		return nil, NewStorageErrorWithPath(ErrorCodeInvalidPath, "path is not a directory", path)
	}

	// Read directory
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, NewProviderError("filesystem", ErrorCodeListFailed, "failed to read directory", err)
	}

	var files []*FileInfo
	for _, entry := range entries {
		entryPath := filepath.Join(path, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue // Skip entries we can't stat
		}

		contentType := "application/octet-stream"
		if !info.IsDir() {
			contentType = mime.TypeByExtension(filepath.Ext(entry.Name()))
			if contentType == "" {
				contentType = "application/octet-stream"
			}
		}

		modTime := info.ModTime()
		files = append(files, &FileInfo{
			Path:         entryPath,
			Name:         entry.Name(),
			Size:         info.Size(),
			ContentType:  contentType,
			LastModified: &modTime,
			IsDirectory:  info.IsDir(),
		})
	}

	return files, nil
}

// DeleteDirectory deletes a directory and all its contents recursively
func (p *FileSystemProvider) DeleteDirectory(ctx context.Context, path string) error {
	fullPath, err := p.getFullPath(path)
	if err != nil {
		return err
	}

	// Check if directory exists
	stat, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DirectoryNotFoundError(path)
		}
		return NewProviderError("filesystem", ErrorCodeDeleteFailed, "failed to stat directory", err)
	}

	if !stat.IsDir() {
		return NewStorageErrorWithPath(ErrorCodeInvalidPath, "path is not a directory", path)
	}

	// Remove directory and all its contents
	if err := os.RemoveAll(fullPath); err != nil {
		return NewProviderError("filesystem", ErrorCodeDeleteFailed, "failed to delete directory", err)
	}

	return nil
}

// Copy copies a file from source to destination
func (p *FileSystemProvider) Copy(ctx context.Context, srcPath, dstPath string) error {
	srcFullPath, err := p.getFullPath(srcPath)
	if err != nil {
		return err
	}

	dstFullPath, err := p.getFullPath(dstPath)
	if err != nil {
		return err
	}

	// Open source file
	src, err := os.Open(srcFullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return FileNotFoundError(srcPath)
		}
		return NewProviderError("filesystem", ErrorCodeCopyFailed, "failed to open source file", err)
	}
	defer src.Close()

	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(dstFullPath), 0755); err != nil {
		return NewProviderError("filesystem", ErrorCodeCopyFailed, "failed to create destination directory", err)
	}

	// Create destination file
	dst, err := os.Create(dstFullPath)
	if err != nil {
		return NewProviderError("filesystem", ErrorCodeCopyFailed, "failed to create destination file", err)
	}
	defer dst.Close()

	// Copy data
	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(dstFullPath) // Clean up on error
		return NewProviderError("filesystem", ErrorCodeCopyFailed, "failed to copy file data", err)
	}

	return nil
}

// Move moves a file from source to destination
func (p *FileSystemProvider) Move(ctx context.Context, srcPath, dstPath string) error {
	srcFullPath, err := p.getFullPath(srcPath)
	if err != nil {
		return err
	}

	dstFullPath, err := p.getFullPath(dstPath)
	if err != nil {
		return err
	}

	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(dstFullPath), 0755); err != nil {
		return NewProviderError("filesystem", ErrorCodeMoveFailed, "failed to create destination directory", err)
	}

	// Try to rename first (most efficient if on same filesystem)
	if err := os.Rename(srcFullPath, dstFullPath); err != nil {
		// If rename fails, try copy + delete
		if err := p.Copy(ctx, srcPath, dstPath); err != nil {
			return err
		}
		if err := p.Delete(ctx, srcPath); err != nil {
			// If delete fails, try to clean up the copy
			p.Delete(ctx, dstPath)
			return err
		}
	}

	return nil
}

// GenerateSignedURL generates a signed URL for filesystem operations
func (p *FileSystemProvider) GenerateSignedURL(ctx context.Context, path string, operation SignedURLOperation, expiresIn time.Duration) (string, error) {
	signedConfig := p.config.GetSignedURLConfig()
	if !signedConfig.Enabled {
		return "", NewStorageError(ErrorCodeSignedURLFailed, "signed URLs are not enabled")
	}

	if signedConfig.SecretKey == "" {
		return "", NewStorageError(ErrorCodeSignedURLFailed, "secret key is required for signed URLs")
	}

	// Create JWT token
	claims := jwt.MapClaims{
		"path": path,
		"op":   string(operation),
		"exp":  time.Now().Add(expiresIn).Unix(),
		"iat":  time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(signedConfig.SecretKey))
	if err != nil {
		return "", NewProviderError("filesystem", ErrorCodeSignedURLFailed, "failed to sign token", err)
	}

	// Return the token (the actual URL construction is handled by the application)
	return tokenString, nil
}

// ValidateSignedToken validates a signed token for filesystem operations
func (p *FileSystemProvider) ValidateSignedToken(tokenString, path string, operation SignedURLOperation) error {
	signedConfig := p.config.GetSignedURLConfig()
	if !signedConfig.Enabled {
		return NewStorageError(ErrorCodeSignedURLFailed, "signed URLs are not enabled")
	}

	if signedConfig.SecretKey == "" {
		return NewStorageError(ErrorCodeSignedURLFailed, "secret key is required for signed URLs")
	}

	// Parse and validate token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(signedConfig.SecretKey), nil
	})

	if err != nil {
		return InvalidTokenError("invalid token: " + err.Error())
	}

	if !token.Valid {
		return InvalidTokenError("token is not valid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return InvalidTokenError("invalid token claims")
	}

	// Validate path
	tokenPath, ok := claims["path"].(string)
	if !ok || tokenPath != path {
		return InvalidTokenError("token path does not match requested path")
	}

	// Validate operation
	tokenOp, ok := claims["op"].(string)
	if !ok || tokenOp != string(operation) {
		return InvalidTokenError("token operation does not match requested operation")
	}

	// Check expiration
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return TokenExpiredError()
		}
	}

	return nil
}

// getFullPath constructs the full filesystem path
func (p *FileSystemProvider) getFullPath(path string) (string, error) {
	// Clean and validate path
	cleanPath := filepath.Clean(path)

	// Prevent path traversal attacks
	if strings.Contains(cleanPath, "..") {
		return "", InvalidPathError(path)
	}

	// Remove leading slash if present
	cleanPath = strings.TrimPrefix(cleanPath, "/")

	return filepath.Join(p.config.FileSystem.BasePath, cleanPath), nil
}
