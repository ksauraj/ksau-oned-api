package main

import (
	"embed"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"ksau-oned-api/azure" // Adjust the import path
)

//go:embed rclone.conf
var configFile embed.FS

// Constants for dynamic chunk size selection
const (
	smallFileSize  = 100 * 1024 * 1024  // 100 MB
	mediumFileSize = 500 * 1024 * 1024  // 500 MB
	largeFileSize  = 1024 * 1024 * 1024 // 1 GB
)

func main() {
	// Define command-line flags
	filePath := flag.String("file", "", "Path to the local file to upload")
	remotePath := flag.String("remote", "", "Remote path on OneDrive to upload the file")
	chunkSize := flag.Int64("chunk-size", 0, "Chunk size for uploads (in bytes). If 0, it will be dynamically selected based on file size.")
	parallelChunks := flag.Int("parallel", 1, "Number of parallel chunks to upload")
	maxRetries := flag.Int("retries", 3, "Maximum number of retries for uploading chunks")
	retryDelay := flag.Duration("retry-delay", 5*time.Second, "Delay between retries")

	flag.Parse()

	// Check if the file and remote flags are provided
	if *filePath == "" || *remotePath == "" {
		fmt.Println("Error: both -file and -remote flags are required")
		flag.Usage()
		return
	}

	// Get file size
	fileInfo, err := os.Stat(*filePath)
	if err != nil {
		fmt.Println("Failed to get file info:", err)
		return
	}
	fileSize := fileInfo.Size()

	// Dynamically select chunk size if not specified by the user
	if *chunkSize == 0 {
		*chunkSize = getChunkSize(fileSize)
		fmt.Printf("Selected chunk size: %d bytes (based on file size: %d bytes)\n", *chunkSize, fileSize)
	} else {
		fmt.Printf("Using user-specified chunk size: %d bytes\n", *chunkSize)
	}

	// Read the embedded config file
	configData, err := configFile.ReadFile("rclone.conf")
	if err != nil {
		fmt.Println("Failed to read embedded config file:", err)
		return
	}

	// Initialize AzureClient using the embedded config
	client, err := azure.NewAzureClientFromRcloneConfigData(configData)
	if err != nil {
		fmt.Println("Failed to initialize client:", err)
		return
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}

	// Prepare upload parameters
	params := azure.UploadParams{
		FilePath:       *filePath,
		RemoteFilePath: *remotePath,
		ChunkSize:      *chunkSize,
		ParallelChunks: *parallelChunks,
		MaxRetries:     *maxRetries,
		RetryDelay:     *retryDelay,
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

// getChunkSize dynamically selects a chunk size based on the file size
func getChunkSize(fileSize int64) int64 {
	switch {
	case fileSize <= smallFileSize:
		return 2 * 1024 * 1024 // 2 MB for small files
	case fileSize <= mediumFileSize:
		return 4 * 1024 * 1024 // 4 MB for medium files
	case fileSize <= largeFileSize:
		return 8 * 1024 * 1024 // 8 MB for large files
	default:
		return 16 * 1024 * 1024 // 16 MB for very large files
	}
}
