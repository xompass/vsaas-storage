package vsaasstorage

import "fmt"

// ErrorCode represents the type of storage error
type ErrorCode string

const (
	ErrorCodeInvalidProvider   ErrorCode = "INVALID_PROVIDER"
	ErrorCodeInvalidConfig     ErrorCode = "INVALID_CONFIG"
	ErrorCodeFileNotFound      ErrorCode = "FILE_NOT_FOUND"
	ErrorCodeDirectoryNotFound ErrorCode = "DIRECTORY_NOT_FOUND"
	ErrorCodeFileAlreadyExists ErrorCode = "FILE_ALREADY_EXISTS"
	ErrorCodePermissionDenied  ErrorCode = "PERMISSION_DENIED"
	ErrorCodeInvalidPath       ErrorCode = "INVALID_PATH"
	ErrorCodeUploadFailed      ErrorCode = "UPLOAD_FAILED"
	ErrorCodeDownloadFailed    ErrorCode = "DOWNLOAD_FAILED"
	ErrorCodeDeleteFailed      ErrorCode = "DELETE_FAILED"
	ErrorCodeCopyFailed        ErrorCode = "COPY_FAILED"
	ErrorCodeMoveFailed        ErrorCode = "MOVE_FAILED"
	ErrorCodeListFailed        ErrorCode = "LIST_FAILED"
	ErrorCodeSignedURLFailed   ErrorCode = "SIGNED_URL_FAILED"
	ErrorCodeInvalidToken      ErrorCode = "INVALID_TOKEN"
	ErrorCodeTokenExpired      ErrorCode = "TOKEN_EXPIRED"
	ErrorCodeProviderError     ErrorCode = "PROVIDER_ERROR"
	ErrorCodeInternalError     ErrorCode = "INTERNAL_ERROR"
)

// StorageError represents a storage operation error
type StorageError struct {
	Code     ErrorCode `json:"code"`
	Message  string    `json:"message"`
	Provider string    `json:"provider,omitempty"`
	Path     string    `json:"path,omitempty"`
	Cause    error     `json:"-"`
}

// Error implements the error interface
func (e *StorageError) Error() string {
	if e.Provider != "" && e.Path != "" {
		return fmt.Sprintf("[%s:%s] %s: %s", e.Provider, e.Code, e.Path, e.Message)
	} else if e.Provider != "" {
		return fmt.Sprintf("[%s:%s] %s", e.Provider, e.Code, e.Message)
	} else if e.Path != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Path, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap implements the errors.Unwrap interface
func (e *StorageError) Unwrap() error {
	return e.Cause
}

// Is implements the errors.Is interface
func (e *StorageError) Is(target error) bool {
	if t, ok := target.(*StorageError); ok {
		return e.Code == t.Code
	}
	return false
}

// NewStorageError creates a new storage error
func NewStorageError(code ErrorCode, message string) *StorageError {
	return &StorageError{
		Code:    code,
		Message: message,
	}
}

// NewStorageErrorWithCause creates a new storage error with a cause
func NewStorageErrorWithCause(code ErrorCode, message string, cause error) *StorageError {
	return &StorageError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// NewStorageErrorWithPath creates a new storage error with a path
func NewStorageErrorWithPath(code ErrorCode, message, path string) *StorageError {
	return &StorageError{
		Code:    code,
		Message: message,
		Path:    path,
	}
}

// NewProviderError creates a new provider-specific error
func NewProviderError(provider string, code ErrorCode, message string, cause error) *StorageError {
	return &StorageError{
		Code:     code,
		Message:  message,
		Provider: provider,
		Cause:    cause,
	}
}

// Helper functions for common errors

func FileNotFoundError(path string) *StorageError {
	return NewStorageErrorWithPath(ErrorCodeFileNotFound, "file not found", path)
}

func DirectoryNotFoundError(path string) *StorageError {
	return NewStorageErrorWithPath(ErrorCodeDirectoryNotFound, "directory not found", path)
}

func FileAlreadyExistsError(path string) *StorageError {
	return NewStorageErrorWithPath(ErrorCodeFileAlreadyExists, "file already exists", path)
}

func PermissionDeniedError(path string) *StorageError {
	return NewStorageErrorWithPath(ErrorCodePermissionDenied, "permission denied", path)
}

func InvalidPathError(path string) *StorageError {
	return NewStorageErrorWithPath(ErrorCodeInvalidPath, "invalid path", path)
}

func InvalidTokenError(message string) *StorageError {
	return NewStorageError(ErrorCodeInvalidToken, message)
}

func TokenExpiredError() *StorageError {
	return NewStorageError(ErrorCodeTokenExpired, "token has expired")
}
