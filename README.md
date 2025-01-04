# ksau-oned-api

[![Go Version](https://img.shields.io/badge/go-1.23.4-blue)](https://golang.org/doc/go1.23)

## Overview

`ksau-oned-api` is a robust and efficient OneDrive file upload API implemented in Go. It leverages Microsoft's Graph API to handle large file uploads using chunked upload sessions. The package is designed to be highly configurable, offering features like dynamic chunk size selection, parallel uploads, retry logic, and file integrity verification using QuickXorHash.

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

3. **Prepare the `rclone.conf` file**:  
   The `rclone.conf` file is required for authentication and configuration. It should be placed in the root directory of the project. The file must contain the necessary credentials for the OneDrive API, including `client_id`, `client_secret`, and `token` information. Below is the desired format for the `rclone.conf` file:

   ```ini
   [remote_name]
   type = onedrive
   client_id = YOUR_CLIENT_ID
   client_secret = YOUR_CLIENT_SECRET
   token = {"access_token":"YOUR_ACCESS_TOKEN","refresh_token":"YOUR_REFRESH_TOKEN","expiry":"TOKEN_EXPIRY_TIME"}
   drive_id = YOUR_DRIVE_ID
   drive_type = YOUR_DRIVE_TYPE
   ```

   Replace the placeholders (`YOUR_CLIENT_ID`, `YOUR_CLIENT_SECRET`, `YOUR_ACCESS_TOKEN`, `YOUR_REFRESH_TOKEN`, `TOKEN_EXPIRY_TIME`, `YOUR_DRIVE_ID`, and `YOUR_DRIVE_TYPE`) with your actual credentials.  
   
   **Important**: Ensure that the `client_id` and `client_secret` are present and valid, as they are required for authentication with Microsoft's Graph API.

4. **Build the project**:
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
- `-remote-config`: Name of the remote configuration section in `rclone.conf` (default: `oned`).
- `-chunk-size`: Chunk size for uploads (in bytes). If 0, it will be dynamically selected based on file size (default: `0`).
- `-parallel`: Number of parallel chunks to upload (default: `1`).
- `-retries`: Maximum number of retries for uploading chunks (default: `3`).
- `-retry-delay`: Delay between retries (default: `5s`).
- `-show-quota`: Display quota information for all remotes and exit.
- `-skip-hash`: Skip QuickXorHash verification (default: `false`).
- `-hash-retries`: Maximum number of retries for fetching QuickXorHash (default: `5`).
- `-hash-retry-delay`: Delay between QuickXorHash retries (default: `10s`).

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
Download URL: https://index.sauraj.eu.org/remote/folder/file.txt
Verifying file integrity...
QuickXorHash match: File integrity verified.
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
Download URL: https://index.sauraj.eu.org/remote/folder/custom_name.txt
Verifying file integrity...
QuickXorHash match: File integrity verified.
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
Download URL: https://index.sauraj.eu.org/remote/folder/largefile.zip
Verifying file integrity...
QuickXorHash match: File integrity verified.
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
Download URL: https://index.sauraj.eu.org/remote/folder/largefile.zip
Verifying file integrity...
QuickXorHash match: File integrity verified.
```

#### Parallel Uploads
```sh
./ksau-go -file /path/to/largefile.zip -remote "remote/folder" -parallel 4
```
Output:
```
Selected chunk size: 8388608 bytes (based on file size: 1200000000 bytes)
Remote file path: remote/folder/largefile.zip
File uploaded successfully.
Download URL: https://index.sauraj.eu.org/remote/folder/largefile.zip
Verifying file integrity...
QuickXorHash match: File integrity verified.
```

#### Display Quota Information
```sh
./ksau-go -show-quota
```
Output:
```
Remote: hakimionedrive
Total:   1.000 TiB
Used:    500.000 GiB
Free:    500.000 GiB
Trashed: 0.000 B

Remote: oned
Total:   1.000 TiB
Used:    300.000 GiB
Free:    700.000 GiB
Trashed: 0.000 B

Remote: saurajcf
Total:   1.000 TiB
Used:    200.000 GiB
Free:    800.000 GiB
Trashed: 0.000 B
```

#### Skip QuickXorHash Verification
```sh
./ksau-go -file /path/to/local/file.txt -remote "remote/folder" -skip-hash
```
Output:
```
Selected chunk size: 2097152 bytes (based on file size: 123456 bytes)
Remote file path: remote/folder/file.txt
File uploaded successfully.
Download URL: https://index.sauraj.eu.org/remote/folder/file.txt
Skipping QuickXorHash verification.
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
- **File Integrity Verification**: Verifies file integrity using QuickXorHash.
- **Configurable Parameters**: Customize chunk size, retries, parallelism, and more.
- **Quota Information**: Display quota information for all configured remotes.

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
	client, err := azure.NewAzureClientFromRcloneConfigData(configData, "oned")
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
	fileID, err := client.Upload(httpClient, params)
	if err != nil {
		fmt.Println("Failed to upload file:", err)
		return
	}

	if fileID != "" {
		fmt.Println("File uploaded successfully.")
	} else {
		fmt.Println("File upload failed.")
	}
}
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

> Developed by **Sauraj**.
