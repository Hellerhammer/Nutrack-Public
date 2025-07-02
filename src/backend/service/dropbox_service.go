package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"nutrack/backend/data"
	"nutrack/backend/messaging"
	"nutrack/backend/settings"
)

const (
	blockSize           = 4 * 1024 * 1024 // 4MB blocks
	dropboxAPIBase      = "https://api.dropboxapi.com/2"
	dropboxContentBase  = "https://content.dropboxapi.com/2"
	dbFileName          = "nutrack.db"
	checkInterval       = 5 * time.Minute  // Interval between remote hash checks
	regularSyncInterval = 5 * time.Minute  // Regular interval for checking remote changes
	initialBackoff      = 1 * time.Minute  // Initial backoff time after an error
	maxBackoff          = 60 * time.Minute // Maximum backoff time
	backoffFactor       = 2.0              // Multiplier for exponential backoff
)

type DropboxMetadata struct {
	ContentHash    string `json:"content_hash,omitempty"`
	PathDisplay    string `json:"path_display"`
	Size           int64  `json:"size"`
	ServerModified string `json:"server_modified,omitempty"`
}

type OperationResult struct {
	Success bool
	Status  string // "uploaded" | "upToDate" | "remoteNewer" | "downloaded"
}

var (
	uploadTimer     *time.Timer
	syncTimer       *time.Timer
	cleanupTimer    *time.Timer
	timerMutex      sync.Mutex
	currentBackoff  time.Duration = initialBackoff // Current backoff duration
	randSource      rand.Source   = rand.NewSource(time.Now().UnixNano())
	randGen         *rand.Rand    = rand.New(randSource)
	lastCleanupDate string        = ""
)

// ExchangeToken exchanges an authorization code for an access token
// Uses PKCE flow which can work without client_secret if the Dropbox app is configured for it
func (s *FoodService) ExchangeToken(code, codeVerifier string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", GetSecret("DROPBOX_CLIENT_ID"))
	data.Set("code_verifier", codeVerifier)

	resp, err := http.PostForm("https://api.dropboxapi.com/oauth2/token", data)
	if err != nil {
		return nil, fmt.Errorf("error making token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("error parsing JSON response: %w, body: %s", err, string(body))
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access token received in response: %s", string(body))
	}

	// Use ExpiresIn from response, default to 4 hours if not provided
	expiresIn := time.Duration(tokenResp.ExpiresIn) * time.Second
	if expiresIn == 0 {
		expiresIn = 4 * time.Hour
	}

	// Set expiration time to 90% of the actual expiration time for safety margin
	expiresAt := time.Now().Add(expiresIn * 9 / 10)

	// Store tokens securely
	if err := s.tokenStore.SaveTokens(&DropboxTokens{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    expiresAt,
	}); err != nil {
		return nil, fmt.Errorf("failed to save tokens: %v", err)
	}

	return &tokenResp, nil
}

// GetAuthenticationStatus returns whether the user is authenticated with Dropbox
func (s *FoodService) GetAuthenticationStatus() (bool, error) {
	tokens, err := s.tokenStore.LoadTokens()
	if err != nil {
		return false, fmt.Errorf("error loading tokens: %w", err)
	}

	if tokens == nil || tokens.AccessToken == "" {
		return false, nil
	}

	// Try to validate the token
	_, err = s.ensureValidToken(tokens)
	if err != nil {
		// If we can't refresh the token, user is not authenticated
		return false, nil
	}

	return true, nil
}

// Logout removes the stored tokens and disables auto-sync
func (s *FoodService) Logout() error {
	if err := s.SetAutoSync(false); err != nil {
		return fmt.Errorf("failed to disable auto-sync: %v", err)
	}
	return s.tokenStore.DeleteTokens()
}

// calculateDropboxHash calculates the content hash according to Dropbox specification
// https://www.dropbox.com/developers/reference/content-hash
func (s *FoodService) calculateDropboxHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	var blockHashes [][]byte

	for {
		block := make([]byte, blockSize)
		n, err := file.Read(block)
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("error reading file: %w", err)
		}
		if n == 0 {
			break
		}

		blockHash := sha256.Sum256(block[:n])
		blockHashes = append(blockHashes, blockHash[:])
	}

	if len(blockHashes) == 0 {
		// Empty file
		return hex.EncodeToString(sha256.New().Sum(nil)), nil
	}

	// Combine block hashes
	combinedHash := sha256.New()
	for _, blockHash := range blockHashes {
		combinedHash.Write(blockHash)
	}

	return hex.EncodeToString(combinedHash.Sum(nil)), nil
}

// ensureValidToken checks if the current token is valid and refreshes it if needed
func (s *FoodService) ensureValidToken(tokens *DropboxTokens) (*DropboxTokens, error) {
	if tokens == nil || tokens.AccessToken == "" {
		return nil, fmt.Errorf("no valid tokens available")
	}

	// Check if token will expire in the next 30 minutes
	timeUntilExpiry := time.Until(tokens.ExpiresAt)
	log.Printf("Token expires in %v", timeUntilExpiry)

	if timeUntilExpiry > 30*time.Minute {
		return tokens, nil
	}

	// If we get here, the token needs to be refreshed
	if tokens.RefreshToken == "" {
		log.Printf("No refresh token available in tokens object")
		return nil, fmt.Errorf("no refresh token available")
	}

	log.Printf("Attempting to refresh token")
	// Try to refresh the token
	newTokens, err := s.refreshAccessToken(tokens.RefreshToken)
	if err != nil {
		log.Printf("Error refreshing token: %v", err)
		return nil, fmt.Errorf("error refreshing token: %w", err)
	}

	// Use ExpiresIn from response, default to 4 hours if not provided
	expiresIn := time.Duration(newTokens.ExpiresIn) * time.Second
	if expiresIn == 0 {
		log.Printf("No ExpiresIn received, defaulting to 4 hours")
		expiresIn = 4 * time.Hour
	}

	expiresAt := time.Now().Add(expiresIn)

	// Ensure we keep the refresh token if a new one wasn't provided
	refreshToken := newTokens.RefreshToken
	if refreshToken == "" {
		log.Printf("No new refresh token received, keeping existing one")
		refreshToken = tokens.RefreshToken
	}

	tokensToSave := &DropboxTokens{
		AccessToken:  newTokens.AccessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}

	log.Printf("Saving new tokens with expiry at %v", expiresAt)
	if err := s.tokenStore.SaveTokens(tokensToSave); err != nil {
		log.Printf("Error saving refreshed tokens: %v", err)
		return nil, fmt.Errorf("error saving refreshed tokens: %w", err)
	}

	return tokensToSave, nil
}

// getRemoteFileMetadata retrieves metadata for a file from Dropbox
func (s *FoodService) getRemoteFileMetadata(tokens *DropboxTokens, path string) (*DropboxMetadata, error) {
	args := map[string]interface{}{
		"path": path,
	}
	argsJson, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("error marshaling args: %w", err)
	}

	req, err := http.NewRequest("POST", dropboxAPIBase+"/files/get_metadata", bytes.NewBuffer(argsJson))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	// Handle 409 path/not_found error
	if resp.StatusCode == http.StatusConflict {
		var dropboxError struct {
			ErrorSummary string `json:"error_summary"`
			Error        struct {
				Tag  string `json:".tag"`
				Path struct {
					Tag string `json:".tag"`
				} `json:"path"`
			} `json:"error"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&dropboxError); err != nil {
			return nil, fmt.Errorf("error decoding error response: %w", err)
		}

		// Check if this is a "not_found" error
		if dropboxError.Error.Tag == "path" && dropboxError.Error.Path.Tag == "not_found" {
			return nil, nil // File doesn't exist
		}

		// If it's a different kind of error, return it
		return nil, fmt.Errorf("metadata request failed: %s", dropboxError.ErrorSummary)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("metadata request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var metadata DropboxMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &metadata, nil
}

// hashComparison checks if the local file needs to be uploaded to Dropbox
func (s *FoodService) hashComparison(tokens *DropboxTokens, localPath, remotePath string) (bool, string, string, error) {
	localHash, err := s.calculateDropboxHash(localPath)
	if err != nil {
		return false, "", "", fmt.Errorf("error calculating local hash: %w", err)
	}

	metadata, err := s.getRemoteFileMetadata(tokens, remotePath)
	if err != nil {
		return false, "", "", fmt.Errorf("error getting remote file metadata: %w", err)
	}

	if metadata == nil {
		return true, localHash, "", nil // File doesn't exist on Dropbox
	}

	// Compare hashes
	return metadata.ContentHash != localHash, localHash, metadata.ContentHash, nil
}

// UploadDatabase uploads the local database file to Dropbox
func (s *FoodService) UploadDatabase() (*OperationResult, error) {

	tokens, err := s.GetValidTokens()
	if err != nil {
		return nil, fmt.Errorf("failed to get valid tokens: %v", err)
	}
	dbPath := data.GetDBPath()

	// Check if we need to upload
	needsUpload, localHash, _, err := s.hashComparison(tokens, dbPath, "/"+dbFileName)
	if err != nil {
		return nil, fmt.Errorf("error checking if upload is needed: %w", err)
	}

	if !needsUpload {
		// Auch wenn kein Upload nötig ist, setzen wir den Status auf SYNCED
		log.Printf("No upload needed, marking as synced with hash: %s", localHash)
		s.settingsStore.MarkAsSynced(localHash)
		if err := s.settingsStore.SaveToFile(); err != nil {
			return nil, fmt.Errorf("error saving settings: %w", err)
		}
		return &OperationResult{Success: true, Status: "upToDate"}, nil
	}

	// Upload file
	file, err := os.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	args := map[string]interface{}{
		"path": "/" + dbFileName,
		"mode": "overwrite",
	}
	argsJson, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("error marshaling args: %w", err)
	}

	req, err := http.NewRequest("POST", dropboxContentBase+"/files/upload", file)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Dropbox-API-Arg", string(argsJson))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Store the local hash after successful upload
	log.Printf("Upload successful, marking as synced with hash: %s", localHash)
	s.settingsStore.MarkAsSynced(localHash)
	if err := s.settingsStore.SaveToFile(); err != nil {
		return nil, fmt.Errorf("error saving settings: %w", err)
	}

	return &OperationResult{Success: true, Status: "uploaded"}, nil
}

// DownloadDatabase downloads the database file from Dropbox
func (s *FoodService) DownloadDatabase() (*OperationResult, error) {

	tokens, err := s.GetValidTokens()
	if err != nil {
		return nil, fmt.Errorf("failed to get valid tokens: %v", err)
	}
	dbPath := data.GetDBPath()

	// Check if we need to download
	needsDownload, _, remoteHash, err := s.hashComparison(tokens, dbPath, "/"+dbFileName)
	if err != nil {
		return nil, fmt.Errorf("error checking if download is needed: %w", err)
	}

	if remoteHash == "" {
		return &OperationResult{Success: false, Status: "notFound"}, nil
	}

	if !needsDownload {
		log.Printf("No download needed, marking as synced with hash: %s", remoteHash)
		s.settingsStore.MarkAsSynced(remoteHash)
		if err := s.settingsStore.SaveToFile(); err != nil {
			return nil, fmt.Errorf("error saving settings: %w", err)
		}
		return &OperationResult{Success: true, Status: "upToDate"}, nil
	}

	// Download file
	args := map[string]interface{}{
		"path": "/" + dbFileName,
	}
	argsJson, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("error marshaling args: %w", err)
	}

	req, err := http.NewRequest("POST", dropboxContentBase+"/files/download", nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	req.Header.Set("Dropbox-API-Arg", string(argsJson))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("download failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "nutrack-*.db")
	if err != nil {
		return nil, fmt.Errorf("error creating temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Copy the response body to the temp file
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error copying response to temp file: %w", err)
	}

	// Close the temp file before moving it
	tempFile.Close()

	// Instead of using os.Rename, copy the file contents
	srcFile, err := os.Open(tempFile.Name())
	if err != nil {
		return nil, fmt.Errorf("error opening temp file: %w", err)
	}
	defer srcFile.Close()

	// Create the target file
	dstFile, err := os.Create(dbPath)
	if err != nil {
		return nil, fmt.Errorf("error creating target file: %w", err)
	}
	defer dstFile.Close()

	// Copy the contents
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return nil, fmt.Errorf("error copying file contents: %w", err)
	}

	// Ensure all data is written to disk
	err = dstFile.Sync()
	if err != nil {
		return nil, fmt.Errorf("error syncing file to disk: %w", err)
	}

	// Close both files before removing the temp file
	srcFile.Close()
	dstFile.Close()

	// Remove the temporary file
	os.Remove(tempFile.Name())

	// Store the remote hash after successful download
	log.Printf("Download successful, marking as synced with hash: %s", remoteHash)
	s.settingsStore.MarkAsSynced(remoteHash)
	if err := s.settingsStore.SaveToFile(); err != nil {
		return nil, fmt.Errorf("error saving settings: %w", err)
	}

	// Notify UI components that data has been updated
	messaging.BroadcastMessage("REMOTE_FILE_UPDATED")
	// Small delay to ensure messages are properly processed
	time.Sleep(50 * time.Millisecond)
	messaging.BroadcastMessage("consumed_food_items_updated")
	time.Sleep(50 * time.Millisecond)
	messaging.BroadcastMessage("food_items_updated")

	return &OperationResult{
		Success: true,
		Status:  "downloaded",
	}, nil
}

// GetValidTokens loads tokens from the store and ensures they are valid
func (s *FoodService) GetValidTokens() (*DropboxTokens, error) {
	tokens, err := s.tokenStore.LoadTokens()
	if err != nil {
		return nil, fmt.Errorf("error loading tokens: %w", err)
	}

	tokens, err = s.ensureValidToken(tokens)
	if err != nil {
		println("error ensuring valid token: ", err)
		return nil, fmt.Errorf("error ensuring valid token: %w", err)
	}

	return tokens, nil
}

// refreshAccessToken refreshes an expired access token
// Uses PKCE flow which can work without client_secret if the Dropbox app is configured for it
func (s *FoodService) refreshAccessToken(refreshToken string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", GetSecret("DROPBOX_CLIENT_ID"))

	// Only include client_secret if it's available (backward compatibility)
	clientSecret := GetSecret("DROPBOX_CLIENT_SECRET")
	if clientSecret != "" {
		data.Set("client_secret", clientSecret)
	}

	resp, err := http.PostForm("https://api.dropboxapi.com/oauth2/token", data)
	if err != nil {
		return nil, fmt.Errorf("error making refresh token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("refresh token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &tokenResp, nil
}

// SetAutoSync sets the auto-sync state and persists it
func (s *FoodService) SetAutoSync(enabled bool) error {
	settings, err := s.settingsStore.Load()
	if err != nil {
		return fmt.Errorf("failed to load settings: %v", err)
	}

	settings.AutoSyncDropbox = enabled
	if err := s.settingsStore.Save(settings); err != nil {
		return fmt.Errorf("failed to save settings: %v", err)
	}

	return nil
}

// GetAutoSync returns the current auto-sync state from persistent storage
func (s *FoodService) GetAutoSync() bool {
	settings, err := s.settingsStore.Load()
	if err != nil {
		log.Printf("Failed to load settings: %v, using default value false", err)
		return false
	}

	return settings.AutoSyncDropbox
}

// shouldSync determines if enough time has passed since the last remote hash check
func (s *FoodService) shouldSync() (bool, error) {
	settings, err := s.settingsStore.Load()
	if err != nil {
		log.Printf("Error loading settings while checking last hash time: %v", err)
		return true, nil
	}

	if settings.LastHashCheck == 0 {
		return true, nil
	}

	now := time.Now().UnixMilli()
	return (now - settings.LastHashCheck) >= checkInterval.Milliseconds(), nil
}

// updateLastCheckTime updates the timestamp of the last remote hash check
func (s *FoodService) updateLastCheckTime() error {
	settingsData, err := s.settingsStore.Load()
	if err != nil {
		settingsData = &settings.Settings{LastHashCheck: 0}
	}

	settingsData.LastHashCheck = time.Now().UnixMilli()
	if err := s.settingsStore.Save(settingsData); err != nil {
		return fmt.Errorf("error saving settings: %w", err)
	}

	return nil
}

// SyncToDropbox checks the remote file hash and resolves any conflicts
func (s *FoodService) SyncToDropbox(force bool) error {
	log.Println("Checking if sync is needed")

	if !s.GetAutoSync() {
		log.Println("Auto-sync is disabled")
		return nil
	}

	if !force {
		shouldCheck, err := s.shouldSync()
		if err != nil {
			log.Printf("Error checking if sync is needed: %v", err)
			return fmt.Errorf("error checking if sync is needed: %v", err)
		}

		if !shouldCheck {
			log.Println("Not enough time has passed since last check")
			return nil
		}
	}

	log.Println("syncing to dropbox")
	tokens, err := s.GetValidTokens()
	if err != nil {
		log.Println("No valid tokens available:", err)
		return fmt.Errorf("no valid tokens available: %v", err)
	}

	metadata, err := s.getRemoteFileMetadata(tokens, "/"+dbFileName)
	if err != nil {
		log.Println("Error getting remote file metadata:", err)
		return fmt.Errorf("failed to get remote file metadata: %v", err)
	}

	if metadata == nil || metadata.ContentHash == "" {
		log.Println("No remote file found or no content hash available")
		return nil
	}

	remoteHash := metadata.ContentHash

	settingsData, err := s.settingsStore.Load()
	if err != nil {
		log.Printf("Error loading settings: %v", err)
		settingsData = &settings.Settings{AutoSyncDropbox: false}
	} else if settingsData == nil {
		settingsData = &settings.Settings{AutoSyncDropbox: false}
	}

	log.Printf("Remote hash: %s", remoteHash)
	log.Printf("Local hash: %s", settingsData.StoredHash)
	log.Printf("Synced: %v", settingsData.Synced)

	if settingsData.StoredHash == "" {
		log.Println("No local hash found, performing initial download")
		_, err := s.DownloadDatabase()
		return err
	} else if settingsData.StoredHash == remoteHash {
		if !settingsData.Synced {
			log.Println("Remote hash matches local hash but local is not synced, uploading ...")
			_, err := s.UploadDatabase()
			return err
		} else {
			log.Println("Remote hash matches local hash and local is synced, no sync needed")
			return nil
		}
	} else if settingsData.StoredHash != remoteHash {
		if !settingsData.Synced {
			log.Println("Remote file changed but local is not synced, showing conflict dialog")
			messaging.BroadcastMessage("SHOW_SYNC_CONFLICT")
			return fmt.Errorf("remote file changed but local is not synced")
		} else {
			log.Println("Remote file changed and local is synced, downloading...")
			result, err := s.DownloadDatabase()
			log.Printf("Download result: %+v, error: %v", result, err)
			return err
		}
	}

	// Update the last check time after successful completion
	if err := s.updateLastCheckTime(); err != nil {
		log.Printf("Warning: Failed to update last check time: %v", err)
	}

	return nil
}

// ScheduleDelayedUpload schedules a database upload to happen after 10 seconds.
// If called again before the upload happens, the timer is reset.
// The upload will only be scheduled if auto-sync is enabled.
func (s *FoodService) ScheduleDelayedUpload() {
	// Only schedule upload if auto-sync is enabled
	if !s.GetAutoSync() {
		return
	}

	timerMutex.Lock()
	defer timerMutex.Unlock()

	// If there's an existing timer, stop it
	if uploadTimer != nil {
		uploadTimer.Stop()
	}

	// Create new timer
	uploadTimer = time.AfterFunc(10*time.Second, func() {
		result, err := s.UploadDatabase()
		if err != nil {
			log.Printf("Scheduled upload failed: %v", err)
			return
		}
		if result.Success {
			log.Printf("Scheduled upload completed: %s", result.Status)
		}
	})
}

// checkRemoteChanged checks if remote Dropbox file has changed compared to our last known hash
func (s *FoodService) checkRemoteChanged(tokens *DropboxTokens) (bool, error) {
	remotePath := "/" + dbFileName
	metadata, err := s.getRemoteFileMetadata(tokens, remotePath)
	if err != nil {
		return false, fmt.Errorf("error checking remote file: %v", err)
	}
	if metadata == nil {
		return false, nil
	}

	remoteHash := metadata.ContentHash
	if remoteHash == "" {
		return false, fmt.Errorf("no content hash available in remote metadata")
	}

	// Load settings to get the stored hash and synced status
	settingsData, err := s.settingsStore.Load()
	if err != nil {
		log.Printf("Error loading settings: %v", err)
		return true, nil // If we can't load settings, assume we need to download
	} else if settingsData == nil {
		return true, nil // If no settings, assume we need to download
	}

	log.Printf("Remote hash: %s", remoteHash)
	log.Printf("Local stored hash: %s", settingsData.StoredHash)
	log.Printf("Local synced status: %v", settingsData.Synced)

	// If we have no stored hash, we need to download
	if settingsData.StoredHash == "" {
		return true, nil
	}

	// If hashes match and we're synced, no need to download
	if settingsData.StoredHash == remoteHash && settingsData.Synced {
		return false, nil
	}

	// If hashes don't match and we're synced, we should download
	if settingsData.StoredHash != remoteHash && settingsData.Synced {
		return true, nil
	}

	// If hashes don't match and we're not synced, this is a conflict
	// In the auto-sync monitor context, we'll skip downloading
	if settingsData.StoredHash != remoteHash && !settingsData.Synced {
		log.Println("Conflict detected: remote hash changed but local is not synced")
		return false, nil
	}

	return false, nil
}

// StartAutoSyncMonitor starts a background process that periodically checks
// for changes in the remote Dropbox file and downloads it if necessary.
// This function should be called once during application startup.
func (s *FoodService) StartAutoSyncMonitor() {
	// Stop any existing timers
	timerMutex.Lock()
	if syncTimer != nil {
		syncTimer.Stop()
	}
	if cleanupTimer != nil {
		cleanupTimer.Stop()
	}
	timerMutex.Unlock()

	// Reset backoff on start
	currentBackoff = initialBackoff

	// Function to check for remote changes and schedule next check
	var checkForChanges func()
	checkForChanges = func() {
		var nextInterval time.Duration = regularSyncInterval
		var apiLimitHit bool = false

		// Only proceed if auto-sync is enabled
		if !s.GetAutoSync() {
			log.Println("Auto-sync is disabled, not checking for remote changes")
			// Schedule next check anyway to see if auto-sync gets enabled
			timerMutex.Lock()
			syncTimer = time.AfterFunc(regularSyncInterval, checkForChanges)
			timerMutex.Unlock()
			return
		}

		log.Println("Checking for remote file changes...")

		// Get valid tokens
		tokens, err := s.GetValidTokens()
		if err != nil {
			log.Printf("Error getting valid tokens: %v", err)
			if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "too many requests") {
				apiLimitHit = true
			}
			// Use backoff for next check
			nextInterval = s.calculateNextBackoff(apiLimitHit)
			log.Printf("Will retry in %v", nextInterval)
			timerMutex.Lock()
			syncTimer = time.AfterFunc(nextInterval, checkForChanges)
			timerMutex.Unlock()
			return
		}

		// Check if remote file has changed
		hasChanged, err := s.checkRemoteChanged(tokens)
		if err != nil {
			log.Printf("Error checking if remote file has changed: %v", err)
			if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "too many requests") {
				apiLimitHit = true
			}
			// Use backoff for next check
			nextInterval = s.calculateNextBackoff(apiLimitHit)
			log.Printf("Will retry in %v", nextInterval)
			timerMutex.Lock()
			syncTimer = time.AfterFunc(nextInterval, checkForChanges)
			timerMutex.Unlock()
			return
		}

		if hasChanged {
			log.Println("Remote file has changed, downloading...")
			result, err := s.DownloadDatabase()
			if err != nil {
				log.Printf("Error downloading database: %v", err)
				if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "too many requests") {
					apiLimitHit = true
					// Use backoff for next check
					nextInterval = s.calculateNextBackoff(apiLimitHit)
				} else {
					// For other errors, use a shorter backoff
					nextInterval = s.calculateNextBackoff(false)
				}
			} else {
				log.Printf("Download result: %+v", result)
				// Notify the user about the download
				messaging.BroadcastMessage("REMOTE_FILE_UPDATED")
				// Small delay to ensure messages are properly processed
				time.Sleep(50 * time.Millisecond)
				messaging.BroadcastMessage("consumed_food_items_updated")
				time.Sleep(50 * time.Millisecond)
				messaging.BroadcastMessage("food_items_updated")
				// Reset backoff on successful download
				currentBackoff = initialBackoff
			}
		} else {
			log.Println("Remote file has not changed, no download needed")
			// Reset backoff on successful check
			currentBackoff = initialBackoff
		}

		// Update the last check time
		if err := s.updateLastCheckTime(); err != nil {
			log.Printf("Warning: Failed to update last check time: %v", err)
		}

		// Schedule next check
		log.Printf("Next check scheduled in %v", nextInterval)
		timerMutex.Lock()
		syncTimer = time.AfterFunc(nextInterval, checkForChanges)
		timerMutex.Unlock()
	}

	// Function to run daily cleanup of old consumed food items
	var runDailyCleanup func()
	runDailyCleanup = func() {
		// Get today's date
		today := time.Now().Format("2006-01-02")

		// Only run cleanup once per day
		if today != lastCleanupDate {
			log.Println("Running daily cleanup of old consumed food items...")

			// Cleanup consumed food items older than three months
			if err := s.CleanupOldConsumedFoodItems(); err != nil {
				log.Printf("Error during cleanup of old consumed food items: %v", err)
			} else {
				log.Println("Daily cleanup completed successfully")
				// Update last cleanup date
				lastCleanupDate = today
			}
		}

		// Calculate time until next cleanup (tomorrow at 00:01)
		now := time.Now()
		tomorrow := now.AddDate(0, 0, 1)
		nextCleanup := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 1, 0, 0, tomorrow.Location())
		duration := nextCleanup.Sub(now)

		// Schedule next cleanup
		log.Printf("Next cleanup scheduled at %v (in %v)", nextCleanup.Format("2006-01-02 15:04:05"), duration)
		timerMutex.Lock()
		cleanupTimer = time.AfterFunc(duration, runDailyCleanup)
		timerMutex.Unlock()
	}

	// Start the first check
	timerMutex.Lock()
	syncTimer = time.AfterFunc(5*time.Second, checkForChanges) // First check after 5 seconds
	timerMutex.Unlock()

	// Start the cleanup timer immediately
	go runDailyCleanup()

	log.Println("Auto-sync monitor and daily cleanup started")
}

// calculateNextBackoff calculates the next backoff interval using exponential backoff
// If apiLimitHit is true, it uses a more aggressive backoff
// Adds jitter to prevent synchronized retry attempts
func (s *FoodService) calculateNextBackoff(apiLimitHit bool) time.Duration {
	applyJitter := func(duration time.Duration) time.Duration {
		// Add random jitter between -15% and +15%
		jitterFactor := 0.85 + (0.3 * randGen.Float64())
		return time.Duration(float64(duration) * jitterFactor)
	}

	if apiLimitHit {
		// For API limit errors, use a more aggressive backoff
		currentBackoff = time.Duration(float64(currentBackoff) * backoffFactor)
		if currentBackoff > maxBackoff {
			currentBackoff = maxBackoff
		}
		return applyJitter(currentBackoff)
	}

	// For other errors, use a milder backoff or reset if it's getting too high
	if currentBackoff > regularSyncInterval*2 {
		// If we've backed off significantly, start reducing the backoff
		currentBackoff = currentBackoff / 2
		if currentBackoff < regularSyncInterval {
			currentBackoff = regularSyncInterval
		}
	} else {
		// Otherwise, slightly increase backoff
		currentBackoff = time.Duration(float64(currentBackoff) * 1.5)
		if currentBackoff > regularSyncInterval*2 {
			currentBackoff = regularSyncInterval * 2
		}
	}

	return applyJitter(currentBackoff)
}
