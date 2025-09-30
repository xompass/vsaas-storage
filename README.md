# VSAAS Storage

VSAAS Storage es un módulo genérico para manejo de archivos que proporciona una interfaz unificada para diferentes proveedores de almacenamiento (filesystem, S3, etc.) con soporte nativo para vsaas-rest.

## Características

- **Interfaz genérica**: Una sola API para todos los proveedores de almacenamiento
- **Múltiples proveedores**: Soporte para filesystem local y S3 (extensible)
- **Configuración unificada**: Inspirada en LoopBack 3 datasources
- **Integración con vsaas-rest**: Handlers listos para usar
- **URLs firmadas**: Soporte para URLs temporales con validación JWT
- **Operaciones completas**: CRUD, copy, move, list, delete directory
- **Streaming de archivos**: Manejo eficiente de archivos grandes
- **Múltiples instancias**: Soporte para diferentes configuraciones simultáneas

## Instalación

```bash
go get github.com/xompass/vsaas-storage
```

## Configuración

### Filesystem Provider

```go
config := &vsaasstorage.StorageConfig{
    Name:     "LocalStorage",
    Provider: "filesystem",
    FileSystem: &vsaasstorage.FileSystemConfig{
        BasePath:    "/var/uploads",
        CreateDirs:  true,
        Permissions: "0755",
    },
    SignedURL: &vsaasstorage.SignedURLConfig{
        Enabled:   true,
        ExpiresIn: 30 * time.Minute,
        SecretKey: "your-secret-key",
    },
}
```

### S3 Provider

```go
config := &vsaasstorage.StorageConfig{
    Name:     "S3Storage",
    Provider: "s3",
    S3: &vsaasstorage.S3Config{
        Region:          "us-west-1",
        Bucket:          "my-bucket",
        AccessKeyID:     "your-access-key",
        SecretAccessKey: "your-secret-key",
        UseSSL:          true,
        ForcePathStyle:  false,
        DefaultUploadParams: map[string]interface{}{
            "CacheControl": "max-age=300",
        },
        MaxRetries: 3,
    },
    SignedURL: &vsaasstorage.SignedURLConfig{
        Enabled:   true,
        ExpiresIn: 1800 * time.Second, // 30 minutos
    },
}
```

## Uso Básico

### Crear una instancia de Storage

```go
storage, err := vsaasstorage.New(config)
if err != nil {
    log.Fatal(err)
}
```

### Operaciones de archivos

```go
ctx := context.Background()

// Upload
metadata := &vsaasstorage.FileMetadata{
    ContentType: "image/jpeg",
    CacheControl: "max-age=3600",
}
fileInfo, err := storage.Upload(ctx, "uploads/avatar.jpg", fileReader, metadata)

// Download
reader, fileInfo, err := storage.Download(ctx, "uploads/avatar.jpg")
if err == nil {
    defer reader.Close()
    // Procesar el archivo
}

// Delete
err = storage.Delete(ctx, "uploads/avatar.jpg")

// Check existence
exists, err := storage.Exists(ctx, "uploads/avatar.jpg")

// Get file info
fileInfo, err := storage.GetInfo(ctx, "uploads/avatar.jpg")

// List directory
files, err := storage.List(ctx, "uploads/")

// Delete directory (recursive)
err = storage.DeleteDirectory(ctx, "uploads/old/")

// Copy file
err = storage.Copy(ctx, "uploads/avatar.jpg", "backups/avatar.jpg")

// Move file
err = storage.Move(ctx, "temp/avatar.jpg", "uploads/avatar.jpg")
```

### URLs Firmadas

```go
// Generate signed URL
signedURL, err := storage.GenerateSignedURL(
    ctx,
    "uploads/avatar.jpg",
    vsaasstorage.SignedURLOperationGet,
    30*time.Minute,
)
```

## Funciones de Upload Mejoradas

### UploadFromCtx - Upload desde contexto vsaas-rest

Esta función procesa automáticamente todos los archivos subidos desde un contexto de vsaas-rest y los sube al directorio especificado.

```go
func customUploadHandler(c *rest.EndpointContext) error {
    // El usuario especifica explícitamente el directorio destino
    destinationDir := "/user-documents"

    // Control dinámico basado en contexto
    if userID := c.EchoCtx.QueryParam("user_id"); userID != "" {
        destinationDir = fmt.Sprintf("/users/%s/documents", userID)
    }

    // Subir archivos al directorio especificado
    results, err := storage.UploadFromCtx(c.Context(), c, destinationDir)
    if err != nil {
        return err
    }

    // results es []*UploadedFileResult con tipado fuerte
    return c.JSON(map[string]interface{}{
        "message": "Files uploaded successfully",
        "files":   results,
    })
}
```

### UploadFromUploadedFile - Procesamiento individual

Para casos donde necesitas procesar archivos individualmente con lógica personalizada:

```go
// Procesar archivos uno por uno
allFiles := c.GetAllUploadedFiles()
var results []*vsaasstorage.UploadedFileResult

for fieldName, files := range allFiles {
    for _, uploadedFile := range files {
        // Control granular por archivo
        destinationDir := determineDestinationForFile(uploadedFile)

        result, err := storage.UploadFromUploadedFile(
            c.Context(),
            uploadedFile,
            fieldName,
            destinationDir,
        )
        if err != nil {
            return err
        }

        results = append(results, result)
    }
}
```

### UploadedFileResult - Estructura de respuesta

Las nuevas funciones retornan una estructura tipada en lugar de mapas genéricos:

```go
type UploadedFileResult struct {
    FieldName    string     `json:"field_name"`    // Nombre del campo del formulario
    OriginalName string     `json:"original_name"` // Nombre original del archivo
    Filename     string     `json:"filename"`      // Nombre del archivo subido
    Path         string     `json:"path"`          // Ruta donde se almacenó
    Size         int64      `json:"size"`          // Tamaño en bytes
    ContentType  string     `json:"content_type"`  // Tipo MIME
    ETag         string     `json:"etag"`          // ETag del archivo
    LastModified *time.Time `json:"last_modified"` // Fecha de modificación
}
```

### UploadHandler Simplificado

El handler de upload ahora es más simple y explícito:

```go
// Antes: El path se inferí­a de parámetros URL
uploadHandler := storage.UploadHandler("/base")

// Después: El usuario especifica explícitamente el directorio
uploadHandler := storage.UploadHandler("/user-documents")

// Registrar el handler
app.POST("/upload", uploadHandler)
```

**Beneficios:**

- Control explícito del directorio destino
- Respuestas con tipado fuerte
- Lógica reutilizable extraída de handlers
- Facilita testing y implementaciones personalizadas
- **Nombres únicos automáticos**: Evita sobrescritura de archivos con el mismo nombre

### Manejo de Nombres Únicos

El sistema genera automáticamente nombres únicos para evitar colisiones:

```go
// Archivo original: "documento.pdf"
// Archivo generado: "documento_a1b2c3d4.pdf"

result.OriginalName // "mi_documento.pdf" (preservado)
result.Filename     // "documento_a1b2c3d4.pdf" (único)
result.Path         // "/uploads/documento_a1b2c3d4.pdf"
```

## Integración con vsaas-rest

### Configurar endpoints

```go
package main

import (
    "github.com/xompass/vsaas-storage"
    rest "github.com/xompass/vsaas-rest"
)

func main() {
    // Crear storage
    storage, err := vsaasstorage.New(config)
    if err != nil {
        panic(err)
    }

    // Crear app
    app := rest.NewRestApp(rest.RestAppOptions{
        Name: "File Server",
        Port: 8080,
    })

    // Configurar endpoints
    api := app.Group("/api/v1")
    files := api.Group("/files")

    // Upload endpoint
    uploadEndpoint := &rest.Endpoint{
        Name:    "UploadFiles",
        Method:  rest.MethodPOST,
        Path:    "/upload/*path",
        Handler: storage.UploadHandler("uploads"),
        FileUploadConfig: &rest.FileUploadConfig{
            MaxFileSize: 10 * 1024 * 1024, // 10MB
            UploadPath:  "./temp",
        },
    }

    // Download endpoint
    downloadEndpoint := &rest.Endpoint{
        Name:    "DownloadFile",
        Method:  rest.MethodGET,
        Path:    "/*path",
        Handler: storage.DownloadHandler(),
    }

    // Delete endpoint
    deleteEndpoint := &rest.Endpoint{
        Name:    "DeleteFile",
        Method:  rest.MethodDELETE,
        Path:    "/*path",
        Handler: storage.DeleteHandler(),
    }

    // List endpoint
    listEndpoint := &rest.Endpoint{
        Name:    "ListFiles",
        Method:  rest.MethodGET,
        Path:    "/list/*path",
        Handler: storage.ListHandler(),
    }

    // Info endpoint
    infoEndpoint := &rest.Endpoint{
        Name:    "GetFileInfo",
        Method:  rest.MethodGET,
        Path:    "/info/*path",
        Handler: storage.InfoHandler(),
    }

    // Registrar endpoints
    app.RegisterEndpoint(uploadEndpoint, files)
    app.RegisterEndpoint(downloadEndpoint, files)
    app.RegisterEndpoint(deleteEndpoint, files)
    app.RegisterEndpoint(listEndpoint, files)
    app.RegisterEndpoint(infoEndpoint, files)

    // Iniciar servidor
    app.Start()
}
```

### Uso de los endpoints

```bash
# Upload files
curl -X POST http://localhost:8080/api/v1/files/upload/avatars \
  -F "avatar=@profile.jpg" \
  -F "document=@resume.pdf"

# Download file
curl http://localhost:8080/api/v1/files/avatars/profile.jpg

# Download with signed URL
curl http://localhost:8080/api/v1/files/avatars/profile.jpg?signed_url=true

# Delete file
curl -X DELETE http://localhost:8080/api/v1/files/avatars/profile.jpg

# Delete directory
curl -X DELETE http://localhost:8080/api/v1/files/avatars?recursive=true

# List directory
curl http://localhost:8080/api/v1/files/list/avatars

# Get file info
curl http://localhost:8080/api/v1/files/info/avatars/profile.jpg
```

## Múltiples Instancias

```go
// Storage para avatars (filesystem)
avatarStorage, err := vsaasstorage.New(&vsaasstorage.StorageConfig{
    Name:     "AvatarStorage",
    Provider: "filesystem",
    FileSystem: &vsaasstorage.FileSystemConfig{
        BasePath: "/var/uploads/avatars",
    },
})

// Storage para documentos (S3)
documentStorage, err := vsaasstorage.New(&vsaasstorage.StorageConfig{
    Name:     "DocumentStorage",
    Provider: "s3",
    S3: &vsaasstorage.S3Config{
        Region: "us-east-1",
        Bucket: "documents-bucket",
        // ... otras configuraciones
    },
})

// Usar diferentes storages para diferentes endpoints
avatarEndpoint := &rest.Endpoint{
    Name:    "UploadAvatar",
    Method:  rest.MethodPOST,
    Path:    "/avatar",
    Handler: avatarStorage.UploadHandler("avatars"),
}

documentEndpoint := &rest.Endpoint{
    Name:    "UploadDocument",
    Method:  rest.MethodPOST,
    Path:    "/document",
    Handler: documentStorage.UploadHandler("documents"),
}
```

## Validación de Tokens (Filesystem)

Para el provider de filesystem, las URLs firmadas se validan automáticamente:

```go
// 1. Solicitar URL firmada
// GET /files/document.pdf?signed_url=true
// Response: 301 redirect to /files/document.pdf?token=eyJ0eXAiOiJKV1Q...

// 2. Acceso con token
// GET /files/document.pdf?token=eyJ0eXAiOiJKV1Q...
// El token se valida automáticamente antes de servir el archivo
```

## Estructura de FileInfo

```go
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
```

## Manejo de Errores

El módulo define códigos de error específicos:

```go
const (
    ErrorCodeFileNotFound      = "FILE_NOT_FOUND"
    ErrorCodePermissionDenied  = "PERMISSION_DENIED"
    ErrorCodeInvalidPath       = "INVALID_PATH"
    ErrorCodeUploadFailed      = "UPLOAD_FAILED"
    ErrorCodeDownloadFailed    = "DOWNLOAD_FAILED"
    // ... otros códigos
)
```

Ejemplo de manejo:

```go
if err != nil {
    if storageErr, ok := err.(*vsaasstorage.StorageError); ok {
        switch storageErr.Code {
        case vsaasstorage.ErrorCodeFileNotFound:
            // Archivo no encontrado
        case vsaasstorage.ErrorCodePermissionDenied:
            // Sin permisos
        default:
            // Otros errores
        }
    }
}
```

## Extensibilidad

Para agregar un nuevo provider, implementa la interfaz `StorageProvider`:

```go
type StorageProvider interface {
    Upload(ctx context.Context, path string, reader io.Reader, metadata *FileMetadata) (*FileInfo, error)
    Download(ctx context.Context, path string) (io.ReadCloser, *FileInfo, error)
    Delete(ctx context.Context, path string) error
    Exists(ctx context.Context, path string) (bool, error)
    GetInfo(ctx context.Context, path string) (*FileInfo, error)
    List(ctx context.Context, path string) ([]*FileInfo, error)
    DeleteDirectory(ctx context.Context, path string) error
    Copy(ctx context.Context, srcPath, dstPath string) error
    Move(ctx context.Context, srcPath, dstPath string) error
    GenerateSignedURL(ctx context.Context, path string, operation SignedURLOperation, expiresIn time.Duration) (string, error)
}
```

## Licencia

Ver archivo LICENSE para más detalles.
