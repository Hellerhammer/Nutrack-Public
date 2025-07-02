package settings

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Settings contains the local settings of the application
type Settings struct {
	AutoSyncDropbox                bool             `json:"auto_sync_dropbox"`
	LastHashCheck                  int64            `json:"last_hash_check"`
	StoredHash                     string           `json:"stored_hash"`
	Synced                         bool             `json:"synced"`
	WeightTracking                 bool             `json:"weight_tracking"`
	ActiveScanner                  *ScannerSettings `json:"active_scanner,omitempty"`
	ActiveProfileID                string           `json:"active_profile_id,omitempty"`
	AutoRecalculateNutritionValues bool             `json:"auto_recalculate_nutrition_values,omitempty"`
}

// ScannerSettings contains the settings for the active scanner
type ScannerSettings struct {
	VendorID  string `json:"vendor_id"`
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
}

// Store manages the persistent settings
type Store struct {
	filePath string
	mutex    sync.RWMutex
	settings *Settings
	dirty    bool
}

var (
	settingsStore *Store
	settingsOnce  sync.Once
)

// GetStore returns a singleton instance of the Settings Store
func GetStore() (*Store, error) {
	var initErr error
	settingsOnce.Do(func() {
		// Create settings directory if it doesn't exist
		settingsDir := "/app/data/settings"
		if err := os.MkdirAll(settingsDir, 0755); err != nil {
			initErr = fmt.Errorf("error creating settings directory: %w", err)
			return
		}

		settingsStore = &Store{
			filePath: filepath.Join(settingsDir, "settings.json"),
			settings: &Settings{},
		}

		// Load initial settings
		if err := settingsStore.loadFromFile(); err != nil {
			if !os.IsNotExist(err) {
				initErr = fmt.Errorf("error loading settings: %w", err)
				return
			}
			// If file doesn't exist, use default settings
			settingsStore.settings = &Settings{
				AutoSyncDropbox:                false,
				LastHashCheck:                  0,
				StoredHash:                     "",
				Synced:                         false,
				WeightTracking:                 false,
				AutoRecalculateNutritionValues: false,
			}
			// Save default settings
			if err := settingsStore.Save(settingsStore.settings); err != nil {
				initErr = fmt.Errorf("error saving default settings: %w", err)
				return
			}
		}
	})

	log.Printf("Loaded settings: %+v\n", settingsStore.settings)
	return settingsStore, initErr
}

// loadFromFile loads the settings from the file
func (s *Store) loadFromFile() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	settings := &Settings{}
	if err := json.Unmarshal(data, settings); err != nil {
		return fmt.Errorf("error unmarshaling settings: %w", err)
	}

	s.settings = settings
	s.dirty = false
	log.Printf("Settings loaded from file %s: %+v", s.filePath, settings)
	return nil
}

// Save stores the settings in memory and optionally on the disk
func (s *Store) Save(settings *Settings) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	log.Printf("Saving settings: %+v", settings)
	s.settings = settings
	s.dirty = true

	return s.saveToFile()
}

// saveToFile writes the current settings to the disk
func (s *Store) saveToFile() error {
	if !s.dirty {
		return nil
	}

	data, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling settings: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("error writing settings file: %w", err)
	}

	log.Printf("Settings written to file %s", s.filePath)
	s.dirty = false
	return nil
}

// Load loads the settings from memory
func (s *Store) Load() (*Settings, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Return a copy to prevent concurrent modifications
	settingsCopy := *s.settings
	log.Printf("Loading settings from memory: %+v", settingsCopy)
	return &settingsCopy, nil
}

// SaveToFile forces the writing of the current settings to the disk
func (s *Store) SaveToFile() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.saveToFile()
}

// MarkAsUnsynced marks the database as unsynchronized in memory
func (s *Store) MarkAsUnsynced() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Only log if the status changes
	if s.settings.Synced {
		log.Printf("Sync status changed: SYNCED -> UNSYNCED")
	}

	s.settings.Synced = false
	s.dirty = true

	if err := s.saveToFile(); err != nil {
		log.Printf("Error saving settings after marking as unsynced: %v", err)
	}
}

// MarkAsSynced marks the database as synchronized and stores the hash
func (s *Store) MarkAsSynced(hash string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	log.Printf("Sync status changed: UNSYNCED -> SYNCED (Hash: %s)", hash)
	s.settings.Synced = true
	s.settings.StoredHash = hash
	s.dirty = true

	if err := s.saveToFile(); err != nil {
		log.Printf("Error saving settings after marking as synced: %v", err)
	}
}
