package main

import (
	"embed"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ksauraj/ksau-oned-api/azure" // Adjust the import path
)

//go:embed rclone.conf
var configFile embed.FS

// Constants for dynamic chunk size selection
const (
	smallFileSize  = 100 * 1024 * 1024  // 100 MB
	mediumFileSize = 500 * 1024 * 1024  // 500 MB
	largeFileSize  = 1024 * 1024 * 1024 // 1 GB
)

// Root folders for each remote configuration (will soon move to config file)
var rootFolders = map[string]string{
	"hakimionedrive": "Public",
	"oned":           "",
	"saurajcf":       "MY_BOMT_STUFFS",
}

// Base URLs for each remote configuration (will soon move to config file)
var baseURLs = map[string]string{
	"hakimionedrive": "https://onedrive-vercel-index-kohl-eight-30.vercel.app",
	"oned":           "https://index.sauraj.eu.org",
	"saurajcf":       "https://my-index-azure.vercel.app",
}

func main() {
	// Define command-line flags
	filePath := flag.String("file", "", "Path to the local file to upload (required)")
	remoteFolder := flag.String("remote", "", "Remote folder on OneDrive to upload the file (required)")
	remoteFileName := flag.String("remote-name", "", "Optional: Remote filename (defaults to local filename if not provided)")
	remoteConfig := flag.String("remote-config", "oned", "Name of the remote configuration section in rclone.conf (default: 'oned')")
	chunkSize := flag.Int64("chunk-size", 0, "Chunk size for uploads (in bytes). If 0, it will be dynamically selected based on file size (default: 0)")
	parallelChunks := flag.Int("parallel", 1, "Number of parallel chunks to upload (default: 1)")
	maxRetries := flag.Int("retries", 3, "Maximum number of retries for uploading chunks (default: 3)")
	retryDelay := flag.Duration("retry-delay", 5*time.Second, "Delay between retries (default: 5s)")
	showQuota := flag.Bool("show-quota", false, "Display quota information for all remotes and exit")

	flag.Parse()

	// Read the embedded config file
	configData, err := configFile.ReadFile("rclone.conf")
	if err != nil {
		fmt.Println("Failed to read embedded config file:", err)
		return
	}

	// Initialize AzureClient for each remote configuration
	httpClient := &http.Client{Timeout: 10 * time.Second}

	if *showQuota {
		for remote := range rootFolders {
			client, err := azure.NewAzureClientFromRcloneConfigData(configData, remote)
			if err != nil {
				fmt.Printf("Failed to initialize client for remote '%s': %v\n", remote, err)
				continue
			}

			quota, err := client.GetDriveQuota(httpClient)
			if err != nil {
				fmt.Printf("Failed to fetch quota information for remote '%s': %v\n", remote, err)
				continue
			}

			azure.DisplayQuotaInfo(remote, quota)
		}
		return
	}

	// Check if the file and remote flags are provided
	if *filePath == "" || *remoteFolder == "" {
		fmt.Println("Error: both -file and -remote flags are required")
		flag.Usage()
		return
	}

	// Get file info
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

	// Determine the remote filename
	localFileName := filepath.Base(*filePath) // Get the local filename
	remoteFilePath := filepath.Join(*remoteFolder, localFileName)
	if *remoteFileName != "" {
		// If a custom remote filename is provided, use it
		remoteFilePath = filepath.Join(*remoteFolder, *remoteFileName)
	}

	// Add the root folder for the selected remote configuration
	rootFolder, exists := rootFolders[*remoteConfig]
	if !exists {
		fmt.Printf("Error: no root folder defined for remote-config '%s'\n", *remoteConfig)
		return
	}
	fullRemotePath := filepath.Join(rootFolder, remoteFilePath)
	fmt.Printf("Full remote path: %s\n", fullRemotePath)

	// Initialize AzureClient using the embedded config and specified remote section
	client, err := azure.NewAzureClientFromRcloneConfigData(configData, *remoteConfig)
	if err != nil {
		fmt.Println("Failed to initialize client:", err)
		return
	}

	// Prepare upload parameters
	params := azure.UploadParams{
		FilePath:       *filePath,
		RemoteFilePath: fullRemotePath,
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

		// Generate the download URL
		baseURL, exists := baseURLs[*remoteConfig]
		if !exists {
			fmt.Printf("Error: no base URL defined for remote-config '%s'\n", *remoteConfig)
			return
		}

		// Construct the URL path
		urlPath := filepath.Join(*remoteFolder, localFileName)
		if *remoteFileName != "" {
			urlPath = filepath.Join(*remoteFolder, *remoteFileName)
		}

		// Encode the URL path
		urlPath = strings.ReplaceAll(urlPath, " ", "%20")

		// Generate the full download URL
		downloadURL := fmt.Sprintf("%s/%s", baseURL, urlPath)
		fmt.Printf("Download URL: %s\n", downloadURL)
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
