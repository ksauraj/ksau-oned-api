# ksau-oned-api

[![Go Version](https://img.shields.io/badge/go-1.23.4-blue)](https://golang.org/doc/go1.23)

## Overview

`ksau-oned-api` is a OneDrive file upload API implemented in Go. It supports large file uploads using Microsoft's Graph API with chunked upload sessions. The package is under development and aims to provide robust and efficient file management on OneDrive.

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
   go build
   ```

## Usage

### Command-Line Flags

The `main.go` provides an example for using the API. Below are the command-line flags you can use:

- `-file`: Path to the local file to upload (required).
- `-remote`: Remote path on OneDrive where the file will be uploaded (required).
- `-config`: Path to the rclone configuration file (default: `rclone.conf`).
- `-chunk-size`: Chunk size for uploads (in bytes, default: `1048576` or 1 MB).
- `-parallel`: Number of parallel chunks to upload (default: `1`).
- `-retries`: Maximum number of retries for uploading chunks (default: `3`).
- `-retry-delay`: Delay between retries (default: `5s`).

### Example Command

```sh
go run main.go -config rclone.conf -file /path/to/local/file -remote "remote/path/on/OneDrive"
```

### Example Output

```plaintext
Starting file upload...
Upload parameters: {FilePath:/path/to/local/file RemoteFilePath:remote/path/on/OneDrive ChunkSize:1048576 MaxRetries:3 RetryDelay:5s Parallel:1}
File size: 10485760 bytes
Uploading chunk: 0-1048575 (1048576 bytes)
Chunk uploaded successfully.
...
File uploaded successfully.
```

## Features

- **Chunked Upload**: Handles large files by splitting them into manageable chunks.
- **Retry Logic**: Retries failed uploads for resilience.
- **Configurable Parameters**: Customize chunk size, retries, and parallelism.

## Development

### Main API Logic

The core API functionality is implemented in `azure/azure.go`. The main file (`main.go`) serves as an example for how to use the API.

### Key Functions

1. `Upload`: Manages the entire file upload process, including chunking and retries.
2. `createUploadSession`: Creates a session for uploading large files.
3. `uploadChunk`: Uploads individual file chunks to the server.

### How to Contribute

1. Fork the repository.
2. Create a feature branch:
   ```sh
   git checkout -b feature-name
   ```
3. Commit your changes:
   ```sh
   git commit -m "Description of changes"
   ```
4. Push to your branch:
   ```sh
   git push origin feature-name
   ```
5. Create a pull request.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

Developed by **Sauraj**.

