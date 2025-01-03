# ksau-oned-api

[![Go Version](https://img.shields.io/badge/go-1.23.4-blue)](https://golang.org/doc/go1.23)

## Overview

`ksau-oned-api` is a OneDrive file upload API implemented in Go. It supports large file uploads using Microsoft's Graph API with chunked upload sessions. The package is designed to be robust, efficient, and easy to use, with features like dynamic chunk size selection, parallel uploads, and retry logic.

## Repository Link

Visit the repository here: [ksau-oned-api](https://github.com/ksauraj/ksau-oned-api)

## Project Structure

```
ksau-oned-api
├── azure
│   └── azure.go      # Contains the main API logic for OneDrive integration
├── go.mod            # Go module configuration
├── main.go           # Example usage of the OneDrive API
└── rclone.conf       # Sample configuration file
```

## Installation

1. **Clone the repository**:
   ```sh
   git clone https://github.com/ksauraj/ksau-oned-api.git
   cd ksau-oned-api
   ```

2. **Install Go**: Ensure you have Go version 1.23.4 or later installed.

3. **Build the project**:
   ```sh
   go build -o ksau-go
   ```

   This will create an executable named `ksau-go` in the current directory.

## Usage

### Command-Line Flags

The `ksau-go` executable provides the following command-line flags:

- `-file`: Path to the local file to upload (required).
- `-remote`: Remote folder on OneDrive where the file will be uploaded (required).
- `-remote-name`: Optional: Remote filename (defaults to the local filename if not provided).
- `-chunk-size`: Chunk size for uploads (in bytes). If 0, it will be dynamically selected based on file size (default: `0`).
- `-parallel`: Number of parallel chunks to upload (default: `1`).
- `-retries`: Maximum number of retries for uploading chunks (default: `3`).
- `-retry-delay`: Delay between retries (default: `5s`).

### Example Commands

#### Basic Usage (Default Chunk Size)
```sh
./ksau-go -file /path/to/local/file.txt -remote "remote/folder"
```
Output:
```
Selected chunk size: 2097152 bytes (based on file size: 123456 bytes)
Remote file path: remote/folder/file.txt
File uploaded successfully.
```

#### Custom Remote Filename
```sh
./ksau-go -file /path/to/local/file.txt -remote "remote/folder" -remote-name "custom_name.txt"
```
Output:
```
Selected chunk size: 2097152 bytes (based on file size: 123456 bytes)
Remote file path: remote/folder/custom_name.txt
File uploaded successfully.
```

#### Large File with Dynamic Chunk Size
```sh
./ksau-go -file /path/to/largefile.zip -remote "remote/folder"
```
Output:
```
Selected chunk size: 8388608 bytes (based on file size: 1200000000 bytes)
Remote file path: remote/folder/largefile.zip
File uploaded successfully.
```

#### User-Specified Chunk Size
```sh
./ksau-go -file /path/to/largefile.zip -remote "remote/folder" -chunk-size 4194304
```
Output:
```
Using user-specified chunk size: 4194304 bytes
Remote file path: remote/folder/largefile.zip
File uploaded successfully.
```

#### Parallel Uploads ( Currently Broken )
```sh
./ksau-go -file /path/to/largefile.zip -remote "remote/folder" -parallel 4
```
Output:
```
Selected chunk size: 8388608 bytes (based on file size: 1200000000 bytes)
Remote file path: remote/folder/largefile.zip
File uploaded successfully.
```

### Dynamic Chunk Size Selection

The program dynamically selects the chunk size based on the file size if the `-chunk-size` flag is not provided:

| **File Size**         | **Chunk Size** |
|------------------------|----------------|
| ≤ 100 MB              | 2 MB           |
| ≤ 500 MB              | 4 MB           |
| ≤ 1 GB                | 8 MB           |
| > 1 GB                | 16 MB          |

### Features

- **Chunked Upload**: Handles large files by splitting them into manageable chunks.
- **Dynamic Chunk Size**: Automatically selects the optimal chunk size based on file size.
- **Parallel Uploads**: Supports uploading multiple chunks in parallel for faster uploads.
- **Retry Logic**: Retries failed uploads for resilience.
- **Configurable Parameters**: Customize chunk size, retries, parallelism, and more.

## Building Your Own Tool

To build your own tool using the `ksau-oned-api` package, follow these steps:

1. **Install the Package**:
   ```sh
   go get github.com/ksauraj/ksau-oned-api
   ```

2. **Import the Package**:
   ```go
   import "github.com/ksauraj/ksau-oned-api/azure"
   ```

3. **Initialize the Azure Client**:
   Use the `NewAzureClientFromRcloneConfigData` function to initialize the client with your configuration.

4. **Upload Files**:
   Use the `Upload` method to upload files with custom parameters.

### Example Code

```go
package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ksauraj/ksau-oned-api/azure"
)

func main() {
	// Initialize the Azure client
	configData := []byte(`your rclone config data here`)
	client, err := azure.NewAzureClientFromRcloneConfigData(configData)
	if err != nil {
		fmt.Println("Failed to initialize client:", err)
		return
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}

	// Prepare upload parameters
	params := azure.UploadParams{
		FilePath:       "/path/to/local/file.txt",
		RemoteFilePath: "remote/folder/file.txt",
		ChunkSize:      4 * 1024 * 1024, // 4 MB
		ParallelChunks: 2,
		MaxRetries:     3,
		RetryDelay:     5 * time.Second,
		AccessToken:    client.AccessToken,
	}

	// Upload the file
	success, err := client.Upload(httpClient, params)
	if err != nil {
		fmt.Println("Failed to upload file:", err)
		return
	}

	if success {
		fmt.Println("File uploaded successfully.")
	} else {
		fmt.Println("File upload failed.")
	}
}
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

Developed by **Sauraj**.

---
