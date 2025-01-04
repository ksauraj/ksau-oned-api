package main

import (
	"embed"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ksauraj/ksau-oned-api/azure" // Adjust the import path
	"github.com/rclone/rclone/backend/onedrive/quickxorhash"
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

// formatBytes converts bytes to a human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.3f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// QuickXorHash calculates the QuickXorHash for a file using the quickxorhash package
func QuickXorHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Create a new QuickXorHash instance
	hash := quickxorhash.New()

	// Copy the file content into the hash
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %v", err)
	}

	// Get the hash as a Base64-encoded string
	hashBytes := hash.Sum(nil)
	hashString := base64.StdEncoding.EncodeToString(hashBytes)

	return hashString, nil
}

// getQuickXorHashWithRetry retries fetching the quickXorHash until it succeeds or max retries are reached
func getQuickXorHashWithRetry(client *azure.AzureClient, httpClient *http.Client, fileID string, maxRetries int, retryDelay time.Duration) (string, error) {
	for retry := 0; retry < maxRetries; retry++ {
		remoteHash, err := client.GetQuickXorHash(httpClient, fileID)
		if err == nil {
			return remoteHash, nil
		}

		// Log the error and wait before retrying
		fmt.Printf("Attempt %d/%d: Failed to retrieve remote QuickXorHash: %v\n", retry+1, maxRetries, err)
		time.Sleep(retryDelay)
	}

	return "", fmt.Errorf("failed to retrieve remote QuickXorHash after %d retries", maxRetries)
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
	skipHash := flag.Bool("skip-hash", false, "Skip QuickXorHash verification (default: false)")
	hashRetries := flag.Int("hash-retries", 5, "Maximum number of retries for fetching QuickXorHash (default: 5)")
	hashRetryDelay := flag.Duration("hash-retry-delay", 10*time.Second, "Delay between QuickXorHash retries (default: 10s)")

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

	fileID, err := client.Upload(httpClient, params)
	if err != nil {
		fmt.Println("Failed to upload file:", err)
		return
	}

	fmt.Printf("File ID: %s\n", fileID)

	if fileID != "" {
		fmt.Println("File uploaded successfully.")
		fmt.Printf("File ID: %s\n", fileID)

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

		// Skip hash verification if requested
		if *skipHash {
			fmt.Println("Skipping QuickXorHash verification.")
			return
		}

		// Calculate the local QuickXorHash
		localHash, err := QuickXorHash(*filePath)
		if err != nil {
			fmt.Printf("Failed to calculate local QuickXorHash: %v\n", err)
			return
		}

		// Retrieve the remote QuickXorHash with retries
		remoteHash, err := getQuickXorHashWithRetry(client, httpClient, fileID, *hashRetries, *hashRetryDelay)
		if err != nil {
			fmt.Printf("Failed to retrieve remote QuickXorHash: %v\n", err)
			return
		}
		fmt.Printf("Remote File ID: %s\n", fileID)
		fmt.Printf("Remote QuickXorHash: %s\n", remoteHash)

		// Compare the hashes
		if localHash != remoteHash {
			fmt.Printf("Local File Path: %s\n", *filePath)
			fmt.Printf("Local File Size: %d bytes\n", fileSize)
			fmt.Printf("Local QuickXorHash: %s\n", localHash)
			fmt.Println("QuickXorHash mismatch: File integrity verification failed.")
		} else {
			fmt.Println("QuickXorHash match: File integrity verified.")
		}
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
