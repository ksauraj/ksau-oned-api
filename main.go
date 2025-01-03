package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"ksau-oned-api/azure" // Adjust the import path
)

func main() {
	// Define command-line flags
	configPath := flag.String("config", "rclone.conf", "Path to the rclone config file")
	filePath := flag.String("file", "", "Path to the local file to upload")
	remotePath := flag.String("remote", "", "Remote path on OneDrive to upload the file")
	chunkSize := flag.Int64("chunk-size", 1048576, "Chunk size for uploads (in bytes)")
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

	// Initialize AzureClient using the rclone config
	client, err := azure.NewAzureClientFromRcloneConfig(*configPath)
	if err != nil {
		fmt.Println("Failed to initialize client:", err)
		return
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}

	// Prepare upload parameters
	params := azure.UploadParams{
		FilePath:        *filePath,
		RemoteFilePath:  *remotePath,
		ChunkSize:       *chunkSize,
		ParallelChunks:  *parallelChunks,
		MaxRetries:      *maxRetries,
		RetryDelay:      *retryDelay,
		AccessToken:     client.AccessToken,
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

