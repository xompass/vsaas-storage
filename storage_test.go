package vsaasstorage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileSystemProvider(t *testing.T) {
	// Setup test directory
	testDir := filepath.Join(os.TempDir(), "vsaas-storage-test")
	defer os.RemoveAll(testDir)

	config := &StorageConfig{
		Name:     "TestStorage",
		Provider: "filesystem",
		FileSystem: &FileSystemConfig{
			BasePath:   testDir,
			CreateDirs: true,
		},
		SignedURL: &SignedURLConfig{
			Enabled:   true,
			ExpiresIn: 5 * time.Minute,
			SecretKey: "test-secret-key",
		},
	}

	storage, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	ctx := context.Background()

	t.Run("Upload and Download", func(t *testing.T) {
		// Test upload
		content := "Hello, World!"
		reader := strings.NewReader(content)

		metadata := &FileMetadata{
			ContentType: "text/plain",
		}

		fileInfo, err := storage.Upload(ctx, "test/hello.txt", reader, metadata)
		if err != nil {
			t.Fatalf("Upload failed: %v", err)
		}

		if fileInfo.Name != "hello.txt" {
			t.Errorf("Expected name 'hello.txt', got '%s'", fileInfo.Name)
		}

		if fileInfo.Size != int64(len(content)) {
			t.Errorf("Expected size %d, got %d", len(content), fileInfo.Size)
		}

		// Test download
		downloadReader, downloadInfo, err := storage.Download(ctx, "test/hello.txt")
		if err != nil {
			t.Fatalf("Download failed: %v", err)
		}
		defer downloadReader.Close()

		if downloadInfo.Name != "hello.txt" {
			t.Errorf("Expected name 'hello.txt', got '%s'", downloadInfo.Name)
		}

		// Read content
		buf := make([]byte, len(content))
		n, err := downloadReader.Read(buf)
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}

		if string(buf[:n]) != content {
			t.Errorf("Expected content '%s', got '%s'", content, string(buf[:n]))
		}
	})

	t.Run("Exists", func(t *testing.T) {
		exists, err := storage.Exists(ctx, "test/hello.txt")
		if err != nil {
			t.Fatalf("Exists check failed: %v", err)
		}

		if !exists {
			t.Error("File should exist")
		}

		exists, err = storage.Exists(ctx, "test/nonexistent.txt")
		if err != nil {
			t.Fatalf("Exists check failed: %v", err)
		}

		if exists {
			t.Error("File should not exist")
		}
	})

	t.Run("GetInfo", func(t *testing.T) {
		fileInfo, err := storage.GetInfo(ctx, "test/hello.txt")
		if err != nil {
			t.Fatalf("GetInfo failed: %v", err)
		}

		if fileInfo.Name != "hello.txt" {
			t.Errorf("Expected name 'hello.txt', got '%s'", fileInfo.Name)
		}

		if fileInfo.IsDirectory {
			t.Error("File should not be marked as directory")
		}
	})

	t.Run("List", func(t *testing.T) {
		// Upload another file
		reader := strings.NewReader("Another file")
		_, err := storage.Upload(ctx, "test/another.txt", reader, nil)
		if err != nil {
			t.Fatalf("Upload failed: %v", err)
		}

		// List directory
		files, err := storage.List(ctx, "test")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(files) != 2 {
			t.Errorf("Expected 2 files, got %d", len(files))
		}

		fileNames := make(map[string]bool)
		for _, file := range files {
			fileNames[file.Name] = true
		}

		if !fileNames["hello.txt"] || !fileNames["another.txt"] {
			t.Error("Expected files not found in list")
		}
	})

	t.Run("Copy", func(t *testing.T) {
		err := storage.Copy(ctx, "test/hello.txt", "test/hello_copy.txt")
		if err != nil {
			t.Fatalf("Copy failed: %v", err)
		}

		// Verify copy exists
		exists, err := storage.Exists(ctx, "test/hello_copy.txt")
		if err != nil {
			t.Fatalf("Exists check failed: %v", err)
		}

		if !exists {
			t.Error("Copied file should exist")
		}

		// Verify original still exists
		exists, err = storage.Exists(ctx, "test/hello.txt")
		if err != nil {
			t.Fatalf("Exists check failed: %v", err)
		}

		if !exists {
			t.Error("Original file should still exist")
		}
	})

	t.Run("Move", func(t *testing.T) {
		err := storage.Move(ctx, "test/hello_copy.txt", "test/hello_moved.txt")
		if err != nil {
			t.Fatalf("Move failed: %v", err)
		}

		// Verify moved file exists
		exists, err := storage.Exists(ctx, "test/hello_moved.txt")
		if err != nil {
			t.Fatalf("Exists check failed: %v", err)
		}

		if !exists {
			t.Error("Moved file should exist")
		}

		// Verify original doesn't exist
		exists, err = storage.Exists(ctx, "test/hello_copy.txt")
		if err != nil {
			t.Fatalf("Exists check failed: %v", err)
		}

		if exists {
			t.Error("Original file should not exist after move")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		err := storage.Delete(ctx, "test/hello_moved.txt")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify file doesn't exist
		exists, err := storage.Exists(ctx, "test/hello_moved.txt")
		if err != nil {
			t.Fatalf("Exists check failed: %v", err)
		}

		if exists {
			t.Error("File should not exist after deletion")
		}
	})

	t.Run("DeleteDirectory", func(t *testing.T) {
		err := storage.DeleteDirectory(ctx, "test")
		if err != nil {
			t.Fatalf("DeleteDirectory failed: %v", err)
		}

		// Verify directory doesn't exist
		_, err = storage.List(ctx, "test")
		if err == nil {
			t.Error("Directory should not exist after deletion")
		}
	})

	t.Run("SignedURL", func(t *testing.T) {
		// Upload a test file first
		reader := strings.NewReader("Test content for signed URL")
		_, err := storage.Upload(ctx, "signed/test.txt", reader, nil)
		if err != nil {
			t.Fatalf("Upload failed: %v", err)
		}

		// Generate signed URL
		signedURL, err := storage.GenerateSignedURL(ctx, "signed/test.txt", SignedURLOperationGet, 5*time.Minute)
		if err != nil {
			t.Fatalf("GenerateSignedURL failed: %v", err)
		}

		if signedURL == "" {
			t.Error("Signed URL should not be empty")
		}

		// Test token validation
		if fsProvider, ok := storage.provider.(*FileSystemProvider); ok {
			err = fsProvider.ValidateSignedToken(signedURL, "signed/test.txt", SignedURLOperationGet)
			if err != nil {
				t.Errorf("Token validation failed: %v", err)
			}

			// Test invalid path
			err = fsProvider.ValidateSignedToken(signedURL, "wrong/path.txt", SignedURLOperationGet)
			if err == nil {
				t.Error("Token validation should fail for wrong path")
			}

			// Test invalid operation
			err = fsProvider.ValidateSignedToken(signedURL, "signed/test.txt", SignedURLOperationPut)
			if err == nil {
				t.Error("Token validation should fail for wrong operation")
			}
		}
	})
}

func TestConfigValidation(t *testing.T) {
	t.Run("Valid filesystem config", func(t *testing.T) {
		config := &StorageConfig{
			Name:     "TestStorage",
			Provider: "filesystem",
			FileSystem: &FileSystemConfig{
				BasePath: "/tmp/test",
			},
		}

		err := config.Validate()
		if err != nil {
			t.Errorf("Valid config should not return error: %v", err)
		}
	})

	t.Run("Missing name", func(t *testing.T) {
		config := &StorageConfig{
			Provider: "filesystem",
			FileSystem: &FileSystemConfig{
				BasePath: "/tmp/test",
			},
		}

		err := config.Validate()
		if err == nil {
			t.Error("Config without name should return error")
		}
	})

	t.Run("Missing provider", func(t *testing.T) {
		config := &StorageConfig{
			Name: "TestStorage",
			FileSystem: &FileSystemConfig{
				BasePath: "/tmp/test",
			},
		}

		err := config.Validate()
		if err == nil {
			t.Error("Config without provider should return error")
		}
	})

	t.Run("Invalid provider", func(t *testing.T) {
		config := &StorageConfig{
			Name:     "TestStorage",
			Provider: "invalid",
		}

		err := config.Validate()
		if err == nil {
			t.Error("Config with invalid provider should return error")
		}
	})

	t.Run("Missing filesystem config", func(t *testing.T) {
		config := &StorageConfig{
			Name:     "TestStorage",
			Provider: "filesystem",
		}

		err := config.Validate()
		if err == nil {
			t.Error("Filesystem provider without config should return error")
		}
	})

	t.Run("Missing base path", func(t *testing.T) {
		config := &StorageConfig{
			Name:     "TestStorage",
			Provider: "filesystem",
			FileSystem: &FileSystemConfig{
				CreateDirs: true,
			},
		}

		err := config.Validate()
		if err == nil {
			t.Error("Filesystem config without base path should return error")
		}
	})
}

func TestStorageErrors(t *testing.T) {
	err := FileNotFoundError("/path/to/file.txt")
	if err.Code != ErrorCodeFileNotFound {
		t.Errorf("Expected error code %s, got %s", ErrorCodeFileNotFound, err.Code)
	}

	if err.Path != "/path/to/file.txt" {
		t.Errorf("Expected path '/path/to/file.txt', got '%s'", err.Path)
	}

	// Test error message
	expectedMsg := "[FILE_NOT_FOUND] /path/to/file.txt: file not found"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}
