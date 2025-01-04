package azure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// AzureClient represents the Azure connection with credentials
type AzureClient struct {
	ClientID     string
	ClientSecret string
	AccessToken  string
	RefreshToken string
	Expiration   time.Time
	DriveID      string
	DriveType    string
	mu           sync.Mutex
}

// NewAzureClientFromRcloneConfigData initializes the AzureClient from embedded rclone config data
func NewAzureClientFromRcloneConfigData(configData []byte, remoteConfig string) (*AzureClient, error) {
	//fmt.Println("Reading rclone config from embedded data for remote:", remoteConfig)
	configMap, err := ParseRcloneConfigData(configData, remoteConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rclone config: %v", err)
	}

	var client AzureClient

	client.ClientID = configMap["client_id"]
	client.ClientSecret = configMap["client_secret"]

	// Extract token information
	var tokenData struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Expiry       string `json:"expiry"`
	}
	err = json.Unmarshal([]byte(configMap["token"]), &tokenData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token JSON: %v", err)
	}

	client.AccessToken = tokenData.AccessToken
	client.RefreshToken = tokenData.RefreshToken

	expiration, err := time.Parse(time.RFC3339, tokenData.Expiry)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token expiration time: %v", err)
	}
	client.Expiration = expiration

	client.DriveID = configMap["drive_id"]
	client.DriveType = configMap["drive_type"]

	return &client, nil
}

// ParseRcloneConfigData parses the rclone configuration data and extracts key-value pairs for the specified remote
func ParseRcloneConfigData(configData []byte, remoteConfig string) (map[string]string, error) {
	//fmt.Println("Parsing rclone config data for remote:", remoteConfig)
	content := string(configData)
	lines := strings.Split(content, "\n")
	configMap := make(map[string]string)

	var currentSection string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.Trim(line, "[]")
			continue
		}

		if currentSection == remoteConfig {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				configMap[key] = value
			}
		}
	}

	if len(configMap) == 0 {
		return nil, fmt.Errorf("no configuration found for remote: %s", remoteConfig)
	}

	return configMap, nil
}

// EnsureTokenValid checks and refreshes the access token if expired
func (client *AzureClient) EnsureTokenValid(httpClient *http.Client) error {
	client.mu.Lock()
	defer client.mu.Unlock()

	if time.Now().Before(client.Expiration) {
		return nil
	}

	tokenURL := "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	data := url.Values{}
	data.Set("client_id", client.ClientID)
	data.Set("client_secret", client.ClientSecret)
	data.Set("refresh_token", client.RefreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("failed to refresh token, status code: %v", res.StatusCode)
	}

	var responseData struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	err = json.NewDecoder(res.Body).Decode(&responseData)
	if err != nil {
		return err
	}

	client.AccessToken = responseData.AccessToken
	client.RefreshToken = responseData.RefreshToken
	client.Expiration = time.Now().Add(time.Duration(responseData.ExpiresIn) * time.Second)

	return nil
}

// Upload uploads a file to OneDrive using parallel chunk uploads
func (client *AzureClient) Upload(httpClient *http.Client, params UploadParams) (string, error) {
	fmt.Println("Starting file upload with upload session...")

	// Ensure the access token is valid
	if err := client.EnsureTokenValid(httpClient); err != nil {
		return "", err
	}

	// Create an upload session
	uploadURL, err := client.createUploadSession(httpClient, params.RemoteFilePath, client.AccessToken)
	if err != nil {
		return "", fmt.Errorf("failed to create upload session: %v", err)
	}
	fmt.Println("Upload session created successfully.")

	// Open the file to upload
	file, err := os.Open(params.FilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Get file information
	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %v", err)
	}
	fileSize := fileInfo.Size()
	fmt.Printf("File size: %d bytes\n", fileSize)

	// Define chunk size and calculate the number of chunks
	chunkSize := params.ChunkSize
	numChunks := (fileSize + chunkSize - 1) / chunkSize

	// Create a worker pool for parallel uploads
	var wg sync.WaitGroup
	chunkChan := make(chan int64, numChunks)
	errChan := make(chan error, numChunks)

	// Start workers
	for i := 0; i < params.ParallelChunks; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for start := range chunkChan {
				end := start + chunkSize - 1
				if end >= fileSize {
					end = fileSize - 1
				}

				// Read the current chunk from the file
				chunk := make([]byte, end-start+1)
				_, err := file.ReadAt(chunk, start)
				if err != nil && err != io.EOF {
					errChan <- fmt.Errorf("failed to read chunk %d-%d: %v", start, end, err)
					continue
				}

				// Retry logic for chunk upload
				for retry := 0; retry < params.MaxRetries; retry++ {
					success, err := client.uploadChunk(httpClient, uploadURL, chunk, start, end, fileSize)
					if success {
						break
					}

					fmt.Printf("Error uploading chunk %d-%d: %v\n", start, end, err)
					fmt.Printf("Retrying chunk upload (attempt %d/%d)...\n", retry+1, params.MaxRetries)
					time.Sleep(params.RetryDelay)
				}
			}
		}()
	}

	// Send chunk start positions to the workers
	for start := int64(0); start < fileSize; start += chunkSize {
		chunkChan <- start
	}
	close(chunkChan)

	// Wait for all workers to finish
	wg.Wait()

	// Check for errors
	select {
	case err := <-errChan:
		return "", fmt.Errorf("failed to upload file: %v", err)
	default:
		fileID, err := client.getFileID(httpClient, params.RemoteFilePath)
		if err != nil {
			return "", fmt.Errorf("failed to fetch file ID: %v", err)
		}

		return fileID, nil
	}

}

// getFileID retrieves the file ID for a given remote path
func (client *AzureClient) getFileID(httpClient *http.Client, remotePath string) (string, error) {
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/%s", remotePath)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+client.AccessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch file metadata: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to fetch file metadata, status: %d, response: %s", resp.StatusCode, responseBody)
	}

	var metadata struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return "", fmt.Errorf("failed to parse metadata: %v", err)
	}

	if metadata.ID == "" {
		return "", fmt.Errorf("file ID not found in metadata")
	}

	return metadata.ID, nil
}

// createUploadSession creates an upload session for the file
func (client *AzureClient) createUploadSession(httpClient *http.Client, remotePath string, accessToken string) (string, error) {
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/%s:/createUploadSession", remotePath)
	requestBody := map[string]interface{}{
		"item": map[string]string{
			"@microsoft.graph.conflictBehavior": "rename",
		},
	}
	body, _ := json.Marshal(requestBody)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create upload session request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create upload session: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create upload session, status: %d, response: %s", resp.StatusCode, responseBody)
	}

	var response struct {
		UploadUrl string `json:"uploadUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to parse upload session response: %v", err)
	}

	return response.UploadUrl, nil
}

// uploadChunk uploads a single chunk of the file
func (client *AzureClient) uploadChunk(httpClient *http.Client, uploadURL string, chunk []byte, start, end, totalSize int64) (bool, error) {
	req, err := http.NewRequest("PUT", uploadURL, bytes.NewReader(chunk))
	if err != nil {
		return false, fmt.Errorf("failed to create chunk upload request: %v", err)
	}

	rangeHeader := fmt.Sprintf("bytes %d-%d/%d", start, end, totalSize)
	req.Header.Set("Content-Range", rangeHeader)

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to upload chunk: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusAccepted {
		return true, nil
	}

	responseBody, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("failed to upload chunk, status: %d, response: %s", resp.StatusCode, responseBody)
}

// itemByPath retrieves the metadata of a folder by its path
func itemByPath(httpClient *http.Client, accessToken, path string) (*DriveItem, error) {
	fmt.Println("Retrieving item by path:", path)
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/%s", path)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	fmt.Println("Item by path response status code:", res.StatusCode)

	if res.StatusCode < 200 || res.StatusCode > 299 {
		responseBody, _ := ioutil.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to retrieve item, status code: %v, response: %s", res.StatusCode, string(responseBody))
	}

	var item DriveItem
	err = json.NewDecoder(res.Body).Decode(&item)
	if err != nil {
		return nil, err
	}

	return &item, nil
}

// DriveItem represents a file or folder item in the drive
type DriveItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// UploadParams represents the parameters for the upload operation
type UploadParams struct {
	FilePath       string
	RemoteFilePath string
	ChunkSize      int64
	ParallelChunks int
	MaxRetries     int
	RetryDelay     time.Duration
	AccessToken    string
}

// DriveQuota represents the quota information for a drive
type DriveQuota struct {
	Total     int64 `json:"total"`
	Used      int64 `json:"used"`
	Remaining int64 `json:"remaining"`
	Deleted   int64 `json:"deleted"`
}

// GetDriveQuota fetches the quota information for the drive
func (client *AzureClient) GetDriveQuota(httpClient *http.Client) (*DriveQuota, error) {
	// Ensure the access token is valid
	if err := client.EnsureTokenValid(httpClient); err != nil {
		return nil, err
	}

	// Construct the URL to get the drive's quota information
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/quota")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create quota request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+client.AccessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quota information: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch quota information, status: %d, response: %s", resp.StatusCode, responseBody)
	}

	var quotaResponse struct {
		Total     int64 `json:"total"`
		Used      int64 `json:"used"`
		Remaining int64 `json:"remaining"`
		Deleted   int64 `json:"deleted"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&quotaResponse); err != nil {
		return nil, fmt.Errorf("failed to parse quota response: %v", err)
	}

	return &DriveQuota{
		Total:     quotaResponse.Total,
		Used:      quotaResponse.Used,
		Remaining: quotaResponse.Remaining,
		Deleted:   quotaResponse.Deleted,
	}, nil
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

// DisplayQuotaInfo displays the quota information for the drive
func DisplayQuotaInfo(remote string, quota *DriveQuota) {
	fmt.Printf("Remote: %s\n", remote)
	fmt.Printf("Total:   %s\n", formatBytes(quota.Total))
	fmt.Printf("Used:    %s\n", formatBytes(quota.Used))
	fmt.Printf("Free:    %s\n", formatBytes(quota.Remaining))
	fmt.Printf("Trashed: %s\n", formatBytes(quota.Deleted))
	fmt.Println()
}

// GetQuickXorHash retrieves the quickXorHash for a file from OneDrive
func (client *AzureClient) GetQuickXorHash(httpClient *http.Client, fileID string) (string, error) {
	// Ensure the access token is valid
	if err := client.EnsureTokenValid(httpClient); err != nil {
		return "", err
	}

	// Construct the URL to get the file's metadata
	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/items/%s", fileID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+client.AccessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch file metadata: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to fetch file metadata, status: %d, response: %s", resp.StatusCode, responseBody)
	}

	// Parse the response to extract the quickXorHash
	var metadata struct {
		File struct {
			Hashes struct {
				QuickXorHash string `json:"quickXorHash"`
			} `json:"hashes"`
		} `json:"file"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return "", fmt.Errorf("failed to parse metadata: %v", err)
	}

	if metadata.File.Hashes.QuickXorHash == "" {
		return "", fmt.Errorf("quickXorHash not found in metadata")
	}

	return metadata.File.Hashes.QuickXorHash, nil
}
