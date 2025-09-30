package vsaasstorage

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	rest "github.com/xompass/vsaas-rest"
	"github.com/xompass/vsaas-rest/http_errors"
)

// UploadHandler creates a handler function for file uploads using vsaas-rest
func (s *Storage) UploadHandler(destinationDir string) func(c *rest.EndpointContext) error {
	return func(c *rest.EndpointContext) error {
		// Use the new UploadFromCtx function
		results, err := s.UploadFromCtx(c.Context(), c, destinationDir)
		if err != nil {
			if storageErr, ok := err.(*StorageError); ok {
				switch storageErr.Code {
				case ErrorCodeUploadFailed:
					return http_errors.BadRequestError(storageErr.Message)
				default:
					return http_errors.InternalServerError(storageErr.Message)
				}
			}
			return http_errors.InternalServerError("Failed to upload files: " + err.Error())
		}

		return c.JSON(map[string]interface{}{
			"message": "Files uploaded successfully",
			"files":   results,
		})
	}
}

// DownloadHandler creates a handler function for file downloads
func (s *Storage) DownloadHandler() func(c *rest.EndpointContext) error {
	return func(c *rest.EndpointContext) error {
		path := c.EchoCtx.Param("path")
		if path == "" {
			path = c.EchoCtx.QueryParam("path")
		}

		if path == "" {
			return http_errors.BadRequestError("File path is required")
		}

		return s.StreamFile(c, path)
	}
}

// handleSignedURLRequest handles the generation of signed URLs
func (s *Storage) handleSignedURLRequest(c *rest.EndpointContext, path string) error {
	// Get expiration time from query params or use default
	expiresInStr := c.EchoCtx.QueryParam("expires_in")
	expiresIn := s.config.GetSignedURLConfig().ExpiresIn

	if expiresInStr != "" {
		if seconds, err := strconv.Atoi(expiresInStr); err == nil {
			expiresIn = time.Duration(seconds) * time.Second
		}
	}

	// Generate signed URL
	signedURL, err := s.GenerateSignedURL(c.Context(), path, SignedURLOperationGet, expiresIn)
	if err != nil {
		return http_errors.InternalServerError("Failed to generate signed URL: " + err.Error())
	}

	// For filesystem provider, construct the actual URL
	if s.config.Provider == "filesystem" {
		// The signed URL for filesystem is just the token, we need to construct the full URL
		req := c.EchoCtx.Request()
		scheme := "http"
		if req.TLS != nil {
			scheme = "https"
		}
		baseURL := fmt.Sprintf("%s://%s%s", scheme, req.Host, req.URL.Path)
		fullURL := fmt.Sprintf("%s?token=%s", baseURL, signedURL)

		// Return 301 redirect
		return c.EchoCtx.Redirect(http.StatusMovedPermanently, fullURL)
	}

	// For other providers (S3), return the signed URL directly
	return c.EchoCtx.Redirect(http.StatusMovedPermanently, signedURL)
}

// handleTokenDownload handles download with token validation
func (s *Storage) handleTokenDownload(c *rest.EndpointContext, path, token string) error {
	// Validate token (only for filesystem provider)
	if s.config.Provider == "filesystem" {
		if fsProvider, ok := s.provider.(*FileSystemProvider); ok {
			if err := fsProvider.ValidateSignedToken(token, path, SignedURLOperationGet); err != nil {
				return http_errors.UnauthorizedError("Invalid or expired token")
			}
		}
	}

	return s.handleDirectDownload(c, path)
}

// handleDirectDownload handles direct file download
func (s *Storage) handleDirectDownload(c *rest.EndpointContext, path string) error {
	// Check if file exists
	exists, err := s.Exists(c.Context(), path)
	if err != nil {
		return http_errors.InternalServerError("Failed to check file existence: " + err.Error())
	}
	if !exists {
		return http_errors.NotFoundError("File not found")
	}

	// Download file
	reader, fileInfo, err := s.Download(c.Context(), path)
	if err != nil {
		return http_errors.InternalServerError("Failed to download file: " + err.Error())
	}
	defer reader.Close()

	// Set headers
	c.EchoCtx.Response().Header().Set("Content-Type", fileInfo.ContentType)
	c.EchoCtx.Response().Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size, 10))
	c.EchoCtx.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileInfo.Name))

	if fileInfo.ETag != "" {
		c.EchoCtx.Response().Header().Set("ETag", fileInfo.ETag)
	}

	if fileInfo.LastModified != nil {
		c.EchoCtx.Response().Header().Set("Last-Modified", fileInfo.LastModified.Format(http.TimeFormat))
	}

	// Stream file content
	_, err = io.Copy(c.EchoCtx.Response().Writer, reader)
	if err != nil {
		return http_errors.InternalServerError("Failed to stream file: " + err.Error())
	}

	return nil
}

// DeleteHandler creates a handler function for file deletion
func (s *Storage) DeleteHandler() func(c *rest.EndpointContext) error {
	return func(c *rest.EndpointContext) error {
		path := c.EchoCtx.Param("path")
		if path == "" {
			path = c.EchoCtx.QueryParam("path")
		}

		if path == "" {
			return http_errors.BadRequestError("File path is required")
		}

		// Check if it's a directory deletion request
		if c.EchoCtx.QueryParam("recursive") == "true" {
			err := s.DeleteDirectory(c.Context(), path)
			if err != nil {
				return http_errors.InternalServerError("Failed to delete directory: " + err.Error())
			}

			return c.JSON(map[string]string{
				"message": "Directory deleted successfully",
				"path":    path,
			})
		}

		// Regular file deletion
		err := s.Delete(c.Context(), path)
		if err != nil {
			if storageErr, ok := err.(*StorageError); ok && storageErr.Code == ErrorCodeFileNotFound {
				return http_errors.NotFoundError("File not found")
			}
			return http_errors.InternalServerError("Failed to delete file: " + err.Error())
		}

		return c.JSON(map[string]string{
			"message": "File deleted successfully",
			"path":    path,
		})
	}
}

// ListHandler creates a handler function for listing files in a directory
func (s *Storage) ListHandler() func(c *rest.EndpointContext) error {
	return func(c *rest.EndpointContext) error {
		path := c.EchoCtx.Param("path")
		if path == "" {
			path = c.EchoCtx.QueryParam("path")
		}

		if path == "" {
			path = "/" // Default to root
		}

		files, err := s.List(c.Context(), path)
		if err != nil {
			if storageErr, ok := err.(*StorageError); ok && storageErr.Code == ErrorCodeDirectoryNotFound {
				return http_errors.NotFoundError("Directory not found")
			}
			return http_errors.InternalServerError("Failed to list files: " + err.Error())
		}

		return c.JSON(map[string]interface{}{
			"path":  path,
			"files": files,
			"count": len(files),
		})
	}
}

// InfoHandler creates a handler function for getting file information
func (s *Storage) InfoHandler() func(c *rest.EndpointContext) error {
	return func(c *rest.EndpointContext) error {
		path := c.EchoCtx.Param("path")
		if path == "" {
			path = c.EchoCtx.QueryParam("path")
		}

		if path == "" {
			return http_errors.BadRequestError("File path is required")
		}

		fileInfo, err := s.GetInfo(c.Context(), path)
		if err != nil {
			if storageErr, ok := err.(*StorageError); ok && storageErr.Code == ErrorCodeFileNotFound {
				return http_errors.NotFoundError("File not found")
			}
			return http_errors.InternalServerError("Failed to get file info: " + err.Error())
		}

		return c.JSON(fileInfo)
	}
}
