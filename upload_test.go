package vsaasstorage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rest "github.com/xompass/vsaas-rest"
)

func TestUploadFromUploadedFile(t *testing.T) {
	// Create temporary directory for testing
	tmpDir := filepath.Join(os.TempDir(), "vsaas-storage-upload-test")
	defer os.RemoveAll(tmpDir)

	// Create storage instance
	config := &StorageConfig{
		Name:     "TestStorage",
		Provider: "filesystem",
		FileSystem: &FileSystemConfig{
			BasePath:    tmpDir,
			CreateDirs:  true,
			Permissions: "0755",
		},
	}

	storage, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	t.Run("UploadFromUploadedFile_Success", func(t *testing.T) {
		// Create a test file
		testContent := "Hello, World!"
		testFile := filepath.Join(tmpDir, "test-upload.txt")
		err := os.WriteFile(testFile, []byte(testContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Create uploaded file
		uploadedFile := &rest.UploadedFile{
			Path:         testFile,
			Filename:     "uploaded-file.txt",
			OriginalName: "original-file.txt",
			MimeType:     "text/plain",
		}

		// Test upload
		result, err := storage.UploadFromUploadedFile(
			context.Background(),
			uploadedFile,
			"test_field",
			"/documents",
		)

		if err != nil {
			t.Fatalf("UploadFromUploadedFile failed: %v", err)
		}

		// Verify result structure
		if result.FieldName != "test_field" {
			t.Errorf("Expected field name 'test_field', got '%s'", result.FieldName)
		}

		if result.OriginalName != "original-file.txt" {
			t.Errorf("Expected original name 'original-file.txt', got '%s'", result.OriginalName)
		}

		// Verify filename is unique (contains original name + unique suffix)
		if !strings.HasPrefix(result.Filename, "uploaded-file_") || !strings.HasSuffix(result.Filename, ".txt") {
			t.Errorf("Expected filename to follow pattern 'uploaded-file_*.txt', got '%s'", result.Filename)
		}

		if result.ContentType != "text/plain" {
			t.Errorf("Expected content type 'text/plain', got '%s'", result.ContentType)
		}

		if result.Size != int64(len(testContent)) {
			t.Errorf("Expected size %d, got %d", len(testContent), result.Size)
		}

		// Verify file was uploaded correctly (use the actual generated path)
		expectedPathPrefix := "/documents/uploaded-file_"
		expectedPathSuffix := ".txt"
		if !strings.HasPrefix(result.Path, expectedPathPrefix) || !strings.HasSuffix(result.Path, expectedPathSuffix) {
			t.Errorf("Expected path to follow pattern '/documents/uploaded-file_*.txt', got '%s'", result.Path)
		}

		// Verify file exists and content is correct (use the actual path from result)
		exists, err := storage.Exists(context.Background(), result.Path)
		if err != nil {
			t.Fatalf("Failed to check if file exists: %v", err)
		}
		if !exists {
			t.Error("Uploaded file does not exist")
		}

		// Verify content (use the actual path from result)
		reader, _, err := storage.Download(context.Background(), result.Path)
		if err != nil {
			t.Fatalf("Failed to download file: %v", err)
		}
		defer reader.Close()

		content, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("Failed to read file content: %v", err)
		}

		if string(content) != testContent {
			t.Errorf("Expected content '%s', got '%s'", testContent, string(content))
		}
	})

	t.Run("DestinationDir_PathHandling", func(t *testing.T) {
		// Test that destination directory paths are handled correctly
		testFile := filepath.Join(tmpDir, "path-test.txt")
		err := os.WriteFile(testFile, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		uploadedFile := &rest.UploadedFile{
			Path:         testFile,
			Filename:     "test.txt",
			OriginalName: "test.txt",
			MimeType:     "text/plain",
		}

		testCases := []struct {
			name           string
			destinationDir string
			expectedPrefix string
			expectedSuffix string
		}{
			{"Without trailing slash", "/docs", "/docs/test_", ".txt"},
			{"With trailing slash", "/docs/", "/docs/test_", ".txt"},
			{"Root directory", "/", "/test_", ".txt"},
			{"Multiple levels", "/users/123/documents", "/users/123/documents/test_", ".txt"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := storage.UploadFromUploadedFile(
					context.Background(),
					uploadedFile,
					"test_field",
					tc.destinationDir,
				)

				if err != nil {
					t.Fatalf("Upload failed: %v", err)
				}

				// Verify path follows expected pattern
				if !strings.HasPrefix(result.Path, tc.expectedPrefix) || !strings.HasSuffix(result.Path, tc.expectedSuffix) {
					t.Errorf("Expected path to match pattern '%s*%s', got '%s'", tc.expectedPrefix, tc.expectedSuffix, result.Path)
				}

				// Clean up for next test
				_ = storage.Delete(context.Background(), result.Path)
			})
		}
	})

	t.Run("UploadFromUploadedFile_FileNotFound", func(t *testing.T) {
		// Test with non-existent file
		uploadedFile := &rest.UploadedFile{
			Path:         "/non-existent-file.txt",
			Filename:     "test.txt",
			OriginalName: "test.txt",
			MimeType:     "text/plain",
		}

		result, err := storage.UploadFromUploadedFile(
			context.Background(),
			uploadedFile,
			"test_field",
			"/test",
		)

		if err == nil {
			t.Error("Expected error when uploaded file does not exist")
		}

		if result != nil {
			t.Error("Expected nil result when upload fails")
		}

		// Check error type
		if storageErr, ok := err.(*StorageError); ok {
			if storageErr.Code != ErrorCodeUploadFailed {
				t.Errorf("Expected error code %s, got %s", ErrorCodeUploadFailed, storageErr.Code)
			}
		} else {
			t.Error("Expected StorageError type")
		}
	})
}

// Test that the handler can be created (we can't easily test execution without a full vsaas-rest setup)
func TestNewUploadHandler(t *testing.T) {
	// Create temporary directory for testing
	tmpDir := filepath.Join(os.TempDir(), "vsaas-storage-handler-test")
	defer os.RemoveAll(tmpDir)

	// Create storage instance
	config := &StorageConfig{
		Name:     "TestStorage",
		Provider: "filesystem",
		FileSystem: &FileSystemConfig{
			BasePath:    tmpDir,
			CreateDirs:  true,
			Permissions: "0755",
		},
	}

	storage, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	t.Run("Handler_Creation", func(t *testing.T) {
		// Test that handler can be created with different destination directories
		destinationDirs := []string{
			"/uploads",
			"/user-documents",
			"/tmp/files",
			"/",
		}

		for _, dir := range destinationDirs {
			handler := storage.UploadHandler(dir)
			if handler == nil {
				t.Errorf("Handler is nil for destination directory: %s", dir)
			}
		}
	})
}

// Test UploadedFileResult structure
func TestUploadedFileResult(t *testing.T) {
	result := &UploadedFileResult{
		FieldName:    "test_field",
		OriginalName: "original.txt",
		Filename:     "uploaded.txt",
		Path:         "/uploads/uploaded.txt",
		Size:         1024,
		ContentType:  "text/plain",
		ETag:         "abc123",
	}

	// Test that all fields can be set and accessed
	if result.FieldName != "test_field" {
		t.Errorf("FieldName not set correctly")
	}

	if result.OriginalName != "original.txt" {
		t.Errorf("OriginalName not set correctly")
	}

	if result.Filename != "uploaded.txt" {
		t.Errorf("Filename not set correctly")
	}

	if result.Path != "/uploads/uploaded.txt" {
		t.Errorf("Path not set correctly")
	}

	if result.Size != 1024 {
		t.Errorf("Size not set correctly")
	}

	if result.ContentType != "text/plain" {
		t.Errorf("ContentType not set correctly")
	}

	if result.ETag != "abc123" {
		t.Errorf("ETag not set correctly")
	}
}

// Test handling of duplicate filenames
func TestDuplicateFilenameHandling(t *testing.T) {
	// Create temporary directory for testing
	tmpDir := filepath.Join(os.TempDir(), "vsaas-storage-duplicate-test")
	defer os.RemoveAll(tmpDir)

	// Create storage instance
	config := &StorageConfig{
		Name:     "TestStorage",
		Provider: "filesystem",
		FileSystem: &FileSystemConfig{
			BasePath:    tmpDir,
			CreateDirs:  true,
			Permissions: "0755",
		},
	}

	storage, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	t.Run("DuplicateFilenames_SameField", func(t *testing.T) {
		// Create two test files with the same name but different content
		testFile1 := filepath.Join(tmpDir, "test-upload-1.txt")
		testFile2 := filepath.Join(tmpDir, "test-upload-2.txt")

		content1 := "Content of file 1"
		content2 := "Content of file 2"

		err := os.WriteFile(testFile1, []byte(content1), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file 1: %v", err)
		}

		err = os.WriteFile(testFile2, []byte(content2), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file 2: %v", err)
		}

		// Create two uploaded files with the SAME filename
		uploadedFile1 := &rest.UploadedFile{
			Path:         testFile1,
			Filename:     "duplicate.txt", // Same filename
			OriginalName: "original1.txt",
			MimeType:     "text/plain",
		}

		uploadedFile2 := &rest.UploadedFile{
			Path:         testFile2,
			Filename:     "duplicate.txt", // Same filename
			OriginalName: "original2.txt",
			MimeType:     "text/plain",
		}

		// Upload both files
		result1, err := storage.UploadFromUploadedFile(
			context.Background(),
			uploadedFile1,
			"test_field",
			"/documents",
		)
		if err != nil {
			t.Fatalf("First upload failed: %v", err)
		}

		result2, err := storage.UploadFromUploadedFile(
			context.Background(),
			uploadedFile2,
			"test_field",
			"/documents",
		)
		if err != nil {
			t.Fatalf("Second upload failed: %v", err)
		}

		// Verify that filenames are different (unique)
		if result1.Filename == result2.Filename {
			t.Errorf("Expected different filenames, but both are: %s", result1.Filename)
		}

		// Verify that paths are different
		if result1.Path == result2.Path {
			t.Errorf("Expected different paths, but both are: %s", result1.Path)
		}

		// Verify both files exist
		exists1, err := storage.Exists(context.Background(), result1.Path)
		if err != nil {
			t.Fatalf("Failed to check if file 1 exists: %v", err)
		}
		if !exists1 {
			t.Error("File 1 does not exist")
		}

		exists2, err := storage.Exists(context.Background(), result2.Path)
		if err != nil {
			t.Fatalf("Failed to check if file 2 exists: %v", err)
		}
		if !exists2 {
			t.Error("File 2 does not exist")
		}

		// Verify content is preserved correctly
		reader1, _, err := storage.Download(context.Background(), result1.Path)
		if err != nil {
			t.Fatalf("Failed to download file 1: %v", err)
		}
		defer reader1.Close()

		actualContent1, err := io.ReadAll(reader1)
		if err != nil {
			t.Fatalf("Failed to read file 1 content: %v", err)
		}

		if string(actualContent1) != content1 {
			t.Errorf("File 1 content mismatch. Expected '%s', got '%s'", content1, string(actualContent1))
		}

		reader2, _, err := storage.Download(context.Background(), result2.Path)
		if err != nil {
			t.Fatalf("Failed to download file 2: %v", err)
		}
		defer reader2.Close()

		actualContent2, err := io.ReadAll(reader2)
		if err != nil {
			t.Fatalf("Failed to read file 2 content: %v", err)
		}

		if string(actualContent2) != content2 {
			t.Errorf("File 2 content mismatch. Expected '%s', got '%s'", content2, string(actualContent2))
		}

		// Verify that generated filenames follow expected pattern (originalname_uniqueid.ext)
		expectedPattern1 := "duplicate_"
		expectedPattern2 := ".txt"

		if !strings.HasPrefix(result1.Filename, expectedPattern1) || !strings.HasSuffix(result1.Filename, expectedPattern2) {
			t.Errorf("File 1 name doesn't follow expected pattern. Got: %s", result1.Filename)
		}

		if !strings.HasPrefix(result2.Filename, expectedPattern1) || !strings.HasSuffix(result2.Filename, expectedPattern2) {
			t.Errorf("File 2 name doesn't follow expected pattern. Got: %s", result2.Filename)
		}

		// Verify original names are preserved
		if result1.OriginalName != "original1.txt" {
			t.Errorf("Expected original name 'original1.txt', got '%s'", result1.OriginalName)
		}

		if result2.OriginalName != "original2.txt" {
			t.Errorf("Expected original name 'original2.txt', got '%s'", result2.OriginalName)
		}
	})

	t.Run("FilenameGeneration_NoExtension", func(t *testing.T) {
		// Test unique filename generation for files without extension
		testFile := filepath.Join(tmpDir, "noext-test.txt")
		err := os.WriteFile(testFile, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		uploadedFile := &rest.UploadedFile{
			Path:         testFile,
			Filename:     "README", // No extension
			OriginalName: "README",
			MimeType:     "text/plain",
		}

		result, err := storage.UploadFromUploadedFile(
			context.Background(),
			uploadedFile,
			"test_field",
			"/docs",
		)
		if err != nil {
			t.Fatalf("Upload failed: %v", err)
		}

		// Verify filename has unique suffix added
		expectedPrefix := "README_"
		if !strings.HasPrefix(result.Filename, expectedPrefix) {
			t.Errorf("Expected filename to start with '%s', got '%s'", expectedPrefix, result.Filename)
		}

		// Verify no double extension was added
		if strings.Contains(result.Filename, ".") {
			t.Errorf("Expected no extension in filename, got '%s'", result.Filename)
		}
	})
}
