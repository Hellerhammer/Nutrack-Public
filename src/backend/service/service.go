package service

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"nutrack/backend/data"
	"nutrack/backend/settings"
	"nutrack/backend/types"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type FoodService struct {
	lastCheckTime time.Time
	tokenStore    *TokenStore
	settingsStore *settings.Store
	activeCmd     *exec.Cmd
	mutex         sync.Mutex
	activeProfile string
	lastChecked   time.Time // For day change monitoring
}

func NewFoodService() (*FoodService, error) {
	tokenStore, err := GetTokenStore()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize token store: %v", err)
	}

	settingsStore, err := settings.GetStore()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize settings store: %v", err)
	}

	service := &FoodService{
		tokenStore:    tokenStore,
		settingsStore: settingsStore,
		lastCheckTime: time.Now().Add(-checkInterval),
		lastChecked:   time.Time{},
	}

	// Start the day change monitor in a separate goroutine
	go service.onDayChangeMonitor()

	// Initialize scanner if one is set as active
	settings, err := settingsStore.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load settings: %v", err)
	}

	if settings.ActiveScanner != nil {
		if err := service.SetActiveScanner(settings.ActiveScanner.Path); err != nil {
			return nil, fmt.Errorf("failed to initialize active scanner: %v", err)
		}
	}

	// List available devices on startup
	if _, err := service.ListDevices(); err != nil {
		fmt.Printf("Warning: Failed to list devices on startup: %v\n", err)
	}

	// Load active profile from settings
	if settings.ActiveProfileID != "" {
		service.mutex.Lock()
		service.activeProfile = settings.ActiveProfileID
		service.mutex.Unlock()
		log.Printf("Loaded active profile from settings: %s", settings.ActiveProfileID)
	} else {
		// If no active profile is set in settings, try to set the first available profile
		go func() {
			// Get all profiles
			profiles, err := data.GetAllProfiles()
			if err != nil {
				log.Printf("Error fetching profiles on startup: %v", err)
				return
			}

			// If there are profiles but no active profile is set
			if len(profiles) > 0 && service.GetActiveProfile() == "" {
				// Set the first profile as active
				if err := service.SetActiveProfile(profiles[0].ID); err != nil {
					log.Printf("Error setting default active profile: %v", err)
				} else {
					log.Printf("Automatically set active profile to: %s (%s)", profiles[0].Name, profiles[0].ID)
				}
			}
		}()
	}

	return service, nil
}

func GetSecret(key string) string {
	fmt.Printf("[GetSecret] Searching for key: %s\n", key)

	// Try to read from Docker secret
	secretPath := fmt.Sprintf("/run/secrets/%s", strings.ToLower(key))
	fmt.Printf("[GetSecret] Trying Docker secret path: %s\n", secretPath)
	if content, err := os.ReadFile(secretPath); err == nil {
		value := strings.TrimSpace(string(content))
		fmt.Printf("[GetSecret] Successfully read Docker secret (length: %d)\n", len(value))
		return value
	} else {
		fmt.Printf("[GetSecret] Failed to read Docker secret: %v\n", err)
	}

	// Fallback to environment variable
	fmt.Printf("[GetSecret] Trying environment variable: %s\n", key)
	if value := os.Getenv(key); value != "" {
		fmt.Printf("[GetSecret] Found in environment variables\n")
		return value
	}

	fmt.Printf("[GetSecret] No value found for %s\n", key)
	return ""
}

func (s *FoodService) GetProductData(barcode string) (*data.PersistentFoodItem, error) {
	if err := ValidateBarcode(barcode); err != nil {
		return nil, err
	}

	// URL encode the barcode to handle special characters and spaces
	encodedBarcode := url.QueryEscape(barcode)
	resp, err := http.Get("https://world.openfoodfacts.org/api/v3/product/" + encodedBarcode + "?fields=code,product_name,nutriments,serving_quantity,serving_quantity_unit")
	if err != nil {
		// Instead of returning error, create default item for non-barcode items
		return &data.PersistentFoodItem{
			Barcode:             barcode,
			Name:                barcode, // Use the barcode as name for now
			CaloriesPer100g:     0,
			ProteinPer100g:      0,
			CarbsPer100g:        0,
			FatPer100g:          0,
			ServingQuantity:     100,
			ServingQuantityUnit: "g",
		}, nil
	}
	defer resp.Body.Close()

	parseFloat := func(v interface{}) float64 {
		switch value := v.(type) {
		case float64:
			return value
		case string:
			f, _ := strconv.ParseFloat(value, 64)
			return f
		default:
			fmt.Println("Unknown type:", value)
			return 0
		}
	}

	var offResponse types.OpenFoodFactsResponse
	if err := json.NewDecoder(resp.Body).Decode(&offResponse); err != nil {
		return nil, fmt.Errorf("failed to parse OpenFoodFacts response: %v", err)
	}

	if offResponse.Status != "success" && offResponse.Status != 1 {
		return nil, fmt.Errorf("no product found in OpenFoodFacts")
	}

	calories := parseFloat(offResponse.Product.Nutriments.EnergyKcal100g)
	if calories == 0 {
		// If kcal is not available, calculate from kJ
		kj := parseFloat(offResponse.Product.Nutriments.EnergyKj100g)
		calories = kj * 0.239006 // Convert kJ to kcal
		calories = math.Round(calories)
	}

	return &data.PersistentFoodItem{
		Barcode:             barcode,
		Name:                offResponse.Product.ProductName,
		CaloriesPer100g:     calories,
		ProteinPer100g:      parseFloat(offResponse.Product.Nutriments.Proteins100g),
		CarbsPer100g:        parseFloat(offResponse.Product.Nutriments.Carbohydrates100g),
		FatPer100g:          parseFloat(offResponse.Product.Nutriments.Fat100g),
		ServingQuantity:     parseFloat(offResponse.Product.ServingQuantity),
		ServingQuantityUnit: offResponse.Product.ServingQuantityUnit,
	}, nil
}

func (s *FoodService) GetAllFoodItems() ([]data.PersistentFoodItem, error) {
	if err := s.SyncToDropbox(false); err != nil {
		return nil, fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	foodItems, err := data.GetAllFoodItems()
	if err != nil {
		fmt.Println("Error getting all food items:", err)
		return nil, err
	}
	return foodItems, nil
}

func (s *FoodService) GetServingQuantityByBarcode(barcode string) (float64, error) {
	if err := s.SyncToDropbox(false); err != nil {
		return 0, fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	if err := ValidateBarcode(barcode); err != nil {
		return 0, err
	}

	servingQuantity, err := data.GetServingQuantityByBarcode(barcode)
	if err != nil {
		return 0, err
	}
	return servingQuantity, nil
}

func (s *FoodService) UpdateFoodItem(barcode string, updateData map[string]interface{}) error {
	if err := s.SyncToDropbox(false); err != nil {
		return fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	if err := ValidateBarcode(barcode); err != nil {
		return err
	}

	for field, value := range updateData {
		err := ValidateField(field, value)
		if err != nil {
			return err
		}
	}

	if err := data.UpdateFoodItem(barcode, updateData); err != nil {
		return err
	}
	s.ScheduleDelayedUpload()
	return nil
}

func (s *FoodService) DeleteFoodItem(barcode string) error {
	if err := s.SyncToDropbox(false); err != nil {
		return fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	if err := ValidateBarcode(barcode); err != nil {
		return err
	}

	if err := data.DeleteFoodItem(barcode); err != nil {
		return err
	}
	s.ScheduleDelayedUpload()
	return nil
}

func (s *FoodService) ResetFoodItem(barcode string) error {
	if err := s.SyncToDropbox(false); err != nil {
		return fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	if err := ValidateBarcode(barcode); err != nil {
		return err
	}

	newFoodData, err := s.GetProductData(barcode)
	if err != nil {
		return err
	}

	ModifyPersistentFoodItem(newFoodData)

	updateData := map[string]interface{}{
		"name":                  newFoodData.Name,
		"energy-kcal_100g":      newFoodData.CaloriesPer100g,
		"proteins_100g":         newFoodData.ProteinPer100g,
		"carbohydrates_100g":    newFoodData.CarbsPer100g,
		"fat_100g":              newFoodData.FatPer100g,
		"serving_quantity":      newFoodData.ServingQuantity,
		"serving_quantity_unit": newFoodData.ServingQuantityUnit,
	}

	err = data.UpdateFoodItem(barcode, updateData)
	if err != nil {
		return err
	}
	return nil
}

func (s *FoodService) CheckAndInsertFoodItem(barcode string) error {
	if err := s.SyncToDropbox(false); err != nil {
		return fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	if err := ValidateBarcode(barcode); err != nil {
		return err
	}

	exists, err := data.CheckFoodItemExists(data.PersistentFoodItem{Barcode: barcode})
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	newFoodData, err := s.GetProductData(barcode)
	if err != nil {
		fmt.Printf("Failed to get product data for barcode %s: %v\n", barcode, err)
		newFoodData = &data.PersistentFoodItem{
			Barcode:             barcode,
			Name:                "",
			CaloriesPer100g:     0,
			ProteinPer100g:      0,
			CarbsPer100g:        0,
			FatPer100g:          0,
			ServingQuantity:     100,
			ServingQuantityUnit: "g",
		}
	}

	ModifyPersistentFoodItem(newFoodData)

	if err := ValidatePersistentFoodItem(*newFoodData); err != nil {
		return err
	}

	err = data.InsertFoodItem(*newFoodData)
	if err != nil {
		return err
	}
	s.ScheduleDelayedUpload()
	return nil
}

func (s *FoodService) ManuallyAddFoodItem(newItem data.PersistentFoodItem) error {
	if err := s.SyncToDropbox(false); err != nil {
		return fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	if err := ValidatePersistentFoodItem(newItem); err != nil {
		return err
	}

	ModifyPersistentFoodItem(&newItem)

	err := data.InsertFoodItem(newItem)
	if err != nil {
		return err
	}
	s.ScheduleDelayedUpload()
	return nil
}

func (s *FoodService) PostConsumedFoodItem(request types.ConsumedFoodItemRequest) error {
	if err := s.SyncToDropbox(request.ForceSync); err != nil {
		return fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	if request.Date == "" {
		request.Date = time.Now().Format("2006-01-02")
	}

	if request.ConsumedQuantity == 0 {
		request.ConsumedQuantity = 1
	}

	if request.ProfileID == "" {
		request.ProfileID = s.GetActiveProfile()

		if request.ProfileID == "" {
			return fmt.Errorf("no profile ID provided and no active profile set")
		}

		fmt.Printf("No profile ID provided, using active profile: %s\n", request.ProfileID)
	}

	if err := ValidateBarcode(request.Barcode); err != nil {
		return err
	}

	if err := ValidateDate(request.Date); err != nil {
		return err
	}

	if err := ValidateConsumedQuantity(request.ConsumedQuantity); err != nil {
		return err
	}

	servingQuantity, err := s.GetServingQuantityByBarcode(request.Barcode)
	if err != nil {
		return err
	}

	existingItem, err := data.GetConsumedFoodItemByBarcodeAndDate(request.Barcode, request.Date, request.ProfileID)
	if err != nil {
		return err
	}

	if existingItem != nil {
		existingItem.ConsumedQuantity += request.ConsumedQuantity
		err = data.UpdateConsumedFoodItem(existingItem.ID, map[string]interface{}{
			"consumed_quantity": existingItem.ConsumedQuantity,
			"profile_id":        request.ProfileID,
		})
		if err != nil {
			return err
		}
		s.ScheduleDelayedUpload()
		return nil
	}

	id := uuid.New().String()
	currentDate := time.Now()
	// Create the consumed food item with the current time
	newConsumedFoodItem := data.ConsumedFoodItem{
		ID:               id,
		Barcode:          request.Barcode,
		ConsumedQuantity: request.ConsumedQuantity,
		ServingQuantity:  servingQuantity,
		Date:             request.Date,
		InsertDate:       currentDate,
	}

	err = data.InsertConsumedFoodItem(newConsumedFoodItem, request.ProfileID)
	if err != nil {
		return err
	}
	s.ScheduleDelayedUpload()
	return nil
}

func (s *FoodService) DeleteConsumedFoodItem(id string) error {
	if err := s.SyncToDropbox(false); err != nil {
		return fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	err := data.DeleteConsumedFoodItem(id)
	if err != nil {
		return err
	}
	s.ScheduleDelayedUpload()
	return nil
}

func (s *FoodService) GetConsumedFoodItemsByDate(date string, profileID string) ([]data.ConsumedFoodItemWithDetails, error) {
	if err := s.SyncToDropbox(false); err != nil {
		return nil, fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	if err := ValidateDate(date); err != nil {
		return nil, err
	}

	// if no profile ID is provided, use the active profile
	if profileID == "" {
		profileID = s.GetActiveProfile()

		if profileID == "" {
			return nil, fmt.Errorf("no profile ID provided and no active profile set")
		}

		fmt.Printf("No profile ID provided, using active profile: %s\n", profileID)
	}

	consumedItems, err := data.GetConsumedFoodItemsByDate(date, profileID)
	if err != nil {
		return nil, err
	}

	return consumedItems, nil
}

func (s *FoodService) UpdateConsumedFoodItem(id string, updateData map[string]interface{}) error {

	fmt.Println("force_sync", updateData["force_sync"])
	if updateData["force_sync"] != nil {
		if err := s.SyncToDropbox(updateData["force_sync"].(bool)); err != nil {
			return fmt.Errorf("failed to sync with Dropbox: %v", err)
		}
	} else {
		if err := s.SyncToDropbox(false); err != nil {
			return fmt.Errorf("failed to sync with Dropbox: %v", err)
		}
	}
	for field, value := range updateData {
		err := ValidateField(field, value)
		if err != nil {
			return err
		}
	}

	err := data.UpdateConsumedFoodItem(id, updateData)
	if err != nil {
		return err
	}
	s.ScheduleDelayedUpload()
	return nil
}

func (s *FoodService) SearchFoodItems(query string) ([]data.PersistentFoodItem, error) {
	if err := s.SyncToDropbox(false); err != nil {
		return nil, fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	items, err := data.SearchFoodItems(query)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s *FoodService) SearchOpenFoodFacts(query string) (map[string]interface{}, error) {
	if len(query) < 3 {
		return nil, fmt.Errorf("search query must be at least 3 characters long")
	}

	// Check if query is a barcode
	isBarcode := true
	for _, c := range query {
		if c < '0' || c > '9' {
			isBarcode = false
			break
		}
	}

	var apiUrl string
	if isBarcode {
		apiUrl = fmt.Sprintf("https://world.openfoodfacts.org/api/v3/product/%s.json", query)
	} else {
		escapedQuery := url.QueryEscape(query)
		apiUrl = fmt.Sprintf("https://world.openfoodfacts.org/cgi/search.pl?search_terms=%s&search_simple=1&json=1&fields=code,product_name,nutriments,serving_quantity,serving_quantity_unit", escapedQuery)
	}

	resp, err := http.Get(apiUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data from OpenFoodFacts: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenFoodFacts API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse OpenFoodFacts response: %v", err)
	}

	// Process and validate the response
	if isBarcode {
		if status, ok := result["status"].(float64); !ok || status != 1 {
			return map[string]interface{}{"products": []interface{}{}}, nil
		}
	} else {
		if products, ok := result["products"].([]interface{}); ok {
			if len(products) == 0 {
				return map[string]interface{}{"products": []interface{}{}}, nil
			}
		} else {
			return nil, fmt.Errorf("invalid response format")
		}
	}

	return result, nil
}

func (s *FoodService) SaveUserSettings(settings data.UserSettings, profileID string) error {
	if err := s.SyncToDropbox(false); err != nil {
		return fmt.Errorf("failed to sync with Dropbox: %v", err)
	}

	// If no profile ID is provided, use the active profile
	if profileID == "" {
		profileID = s.GetActiveProfile()

		if profileID == "" {
			return fmt.Errorf("no profile ID provided and no active profile set")
		}

		fmt.Printf("No profile ID provided, using active profile: %s\n", profileID)
	}

	if err := ValidateUserSettings(settings); err != nil {
		println("error", err.Error())
		return err
	}

	// Save the user settings
	err := data.SaveUserSettings(settings, profileID)
	if err != nil {
		return err
	}

	// Only track weight if weight tracking is enabled
	isWeightTrackingEnabled := s.GetWeightTracking()
	if isWeightTrackingEnabled {
		err = data.InsertWeightTracking(profileID, settings.Weight)
		if err != nil {
			return fmt.Errorf("failed to track weight: %v", err)
		}
		fmt.Printf("Weight tracked for profile %s: %.2f kg\n", profileID, settings.Weight)
	}

	s.ScheduleDelayedUpload()
	return nil
}

func (s *FoodService) GetUserSettings(profileID string) (data.UserSettings, error) {
	if err := s.SyncToDropbox(false); err != nil {
		return data.UserSettings{}, fmt.Errorf("failed to sync with Dropbox: %v", err)
	}

	// If no profile ID is provided, use the active profile
	if profileID == "" {
		profileID = s.GetActiveProfile()

		if profileID == "" {
			return data.UserSettings{}, fmt.Errorf("no profile ID provided and no active profile set")
		}

		fmt.Printf("No profile ID provided, using active profile: %s\n", profileID)
	}

	settings, err := data.GetUserSettings(profileID)
	if err != nil {
		return data.UserSettings{}, err
	}
	return settings, nil
}

func (s *FoodService) CheckInsertAndConsume(request types.ConsumedFoodItemRequest) error {
	if err := s.SyncToDropbox(request.ForceSync); err != nil {
		fmt.Printf("Dropbox sync check failed: %v\n", err)
		return err
	}

	err := s.CheckAndInsertFoodItem(request.Barcode)
	if err != nil {
		fmt.Printf("CheckAndInsertFoodItem failed for barcode %s: %v\n", request.Barcode, err)
		return err
	}
	err = s.PostConsumedFoodItem(request)
	if err != nil {
		fmt.Printf("PostConsumedFoodItem failed: %v\n", err)
		return err
	}
	return nil
}

func (s *FoodService) CheckInsertAndConsumeBatch(request types.BatchConsumedFoodItemRequest) error {
	// First sync with Dropbox if necessary
	if err := s.SyncToDropbox(request.ForceSync); err != nil {
		fmt.Printf("Dropbox sync check failed: %v\n", err)
		return err
	}

	// Process each item individually
	for _, item := range request.Items {
		fmt.Printf("[CheckInsertAndConsumeBatch] Processing item: %v\n", item)
		// Check and insert the food item if it doesn't exist
		err := s.CheckAndInsertFoodItem(item.Barcode)
		if err != nil {
			fmt.Printf("CheckAndInsertFoodItem failed for barcode %s: %v\n", item.Barcode, err)
			return err
		}

		// Consume the food item
		err = s.PostConsumedFoodItem(item)
		if err != nil {
			fmt.Printf("PostConsumedFoodItem failed: %v\n", err)
			return err
		}
	}

	s.ScheduleDelayedUpload()

	return nil
}

func (s *FoodService) CreateDish(request types.DishRequest) error {
	if err := s.SyncToDropbox(false); err != nil {
		return fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	dishID := uuid.New().String()

	dish := data.Dish{
		ID:      dishID,
		Name:    request.Dish.Name,
		Barcode: request.Dish.Barcode,
	}

	items := make([]data.DishItem, len(request.Items))
	for i, item := range request.Items {
		items[i] = data.DishItem{
			DishID:   dishID,
			Barcode:  item.Barcode,
			Quantity: item.Quantity,
		}
	}

	err := data.CreateDish(dish, items)
	if err != nil {
		return err
	}
	s.ScheduleDelayedUpload()
	return nil
}

func (s *FoodService) DeleteDish(id string) error {
	if err := s.SyncToDropbox(false); err != nil {
		return fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	err := data.DeleteDish(id)
	if err != nil {
		return err
	}
	s.ScheduleDelayedUpload()
	return nil
}

func (s *FoodService) UpdateDish(id string, request types.DishRequest) error {
	if err := s.SyncToDropbox(false); err != nil {
		return fmt.Errorf("failed to sync with Dropbox: %v", err)
	}

	items := make([]data.DishItem, len(request.Items))
	for i, item := range request.Items {
		items[i] = data.DishItem{
			DishID:   id,
			Barcode:  item.Barcode,
			Quantity: item.Quantity,
		}
	}

	err := data.UpdateDish(id, request.Dish.Name, request.Dish.Barcode, items)
	if err != nil {
		return err
	}
	s.ScheduleDelayedUpload()
	return nil
}

func (s *FoodService) GetDish(id string) (data.Dish, []data.DishItem, error) {
	if err := s.SyncToDropbox(false); err != nil {
		return data.Dish{}, nil, fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	dish, items, err := data.GetDishWithItems(id)
	if err != nil {
		return data.Dish{}, nil, err
	}
	return dish, items, nil
}

func (s *FoodService) GetAllDishes() ([]data.DishWithDetailedItems, error) {
	if err := s.SyncToDropbox(false); err != nil {
		return nil, fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	dishes, err := data.GetAllDishes()
	if err != nil {
		return nil, err
	}

	return dishes, nil
}

func (s *FoodService) ConvertDishToFoodItem(id string) error {
	if err := s.SyncToDropbox(false); err != nil {
		return fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	err := data.InsertDishAsFoodItem(id)
	if err != nil {
		return err
	}
	s.ScheduleDelayedUpload()
	return nil
}

func (s *FoodService) CreateProfile(name string) (string, error) {
	if err := s.SyncToDropbox(false); err != nil {
		return "", fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	if name == "" {
		return "", fmt.Errorf("profile name is required")
	}

	profile := data.Profile{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: time.Now(),
	}

	err := data.AddProfile(profile)
	if err != nil {
		return "", fmt.Errorf("failed to create profile: %v", err)
	}
	s.ScheduleDelayedUpload()
	return profile.ID, nil
}

func (s *FoodService) GetProfile(profileID string) (data.Profile, error) {
	if err := s.SyncToDropbox(false); err != nil {
		return data.Profile{}, fmt.Errorf("failed to sync with Dropbox: %v", err)
	}

	if profileID == "" {
		profileID = s.GetActiveProfile()

		if profileID == "" {
			return data.Profile{}, fmt.Errorf("no profile ID provided and no active profile set")
		}

		fmt.Printf("No profile ID provided, using active profile: %s\n", profileID)
	}

	profile, err := data.GetProfile(profileID)
	if err != nil {
		return data.Profile{}, fmt.Errorf("failed to get profile: %v", err)
	}

	return profile, nil
}

func (s *FoodService) GetAllProfiles() ([]data.Profile, error) {
	if err := s.SyncToDropbox(false); err != nil {
		return nil, fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	profiles, err := data.GetAllProfiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get profiles: %v", err)
	}

	return profiles, nil
}

func (s *FoodService) UpdateProfile(profileID string, name string) error {
	if err := s.SyncToDropbox(false); err != nil {
		return fmt.Errorf("failed to sync with Dropbox: %v", err)
	}

	// If no profile ID is provided, use the active profile
	if profileID == "" {
		profileID = s.GetActiveProfile()

		if profileID == "" {
			return fmt.Errorf("no profile ID provided and no active profile set")
		}

		fmt.Printf("No profile ID provided, using active profile: %s\n", profileID)
	}

	if name == "" {
		return fmt.Errorf("profile name is required")
	}

	err := data.UpdateProfile(profileID, name)
	if err != nil {
		return fmt.Errorf("failed to update profile: %v", err)
	}
	s.ScheduleDelayedUpload()
	return nil
}

func (s *FoodService) DeleteProfile(profileID string) error {
	if err := s.SyncToDropbox(false); err != nil {
		return fmt.Errorf("failed to sync with Dropbox: %v", err)
	}
	if profileID == "" {
		return fmt.Errorf("profile ID is required")
	}

	err := data.DeleteProfile(profileID)
	if err != nil {
		return fmt.Errorf("failed to delete profile: %v", err)
	}
	s.ScheduleDelayedUpload()
	return nil
}

func (s *FoodService) ListScanners() ([]ScannerDevice, error) {
	return s.ListDevices()
}

func (s *FoodService) StopScanner() error {
	return s.StopListening()
}

func (s *FoodService) CleanupOldConsumedFoodItems() error {
	// Sync with Dropbox first to ensure we have the latest data
	if err := s.SyncToDropbox(false); err != nil {
		return fmt.Errorf("failed to sync with Dropbox before cleanup: %v", err)
	}

	// Calculate date three months ago in format YYYY-MM-DD
	threeMonthsAgo := time.Now().AddDate(0, -3, 0).Format("2006-01-02")

	// Delete old consumed food items
	rowsAffected, err := data.DeleteConsumedFoodItemsOlderThan(threeMonthsAgo)
	if err != nil {
		return fmt.Errorf("failed to delete old consumed food items: %v", err)
	}

	// If items were deleted, schedule an upload to Dropbox
	if rowsAffected > 0 {
		log.Printf("Deleted %d consumed food items older than %s", rowsAffected, threeMonthsAgo)
		s.ScheduleDelayedUpload()
	}

	return nil
}

// SetActiveProfile sets the active profile
func (s *FoodService) SetActiveProfile(profileID string) error {
	// Check if the profile exists
	profile, err := s.GetProfile(profileID)
	if err != nil {
		return fmt.Errorf("failed to get profile: %v", err)
	}

	if profile.ID == "" {
		return fmt.Errorf("profile with ID %s not found", profileID)
	}

	s.activeProfile = profileID

	// Save the active profile in the settings
	settings, err := s.settingsStore.Load()
	if err != nil {
		return fmt.Errorf("failed to load settings: %v", err)
	}

	settings.ActiveProfileID = profileID
	if err := s.settingsStore.Save(settings); err != nil {
		return fmt.Errorf("failed to save active profile to settings: %v", err)
	}

	fmt.Printf("Active profile set to: %s (%s)\n", profile.Name, profileID)
	return nil
}

// GetActiveProfile returns the ID of the active profile
func (s *FoodService) GetActiveProfile() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// If an active profile is already in memory, return it
	if s.activeProfile != "" {
		return s.activeProfile
	}

	// Try to load the active profile from settings
	settings, err := s.settingsStore.Load()
	if err != nil {
		fmt.Printf("Error loading settings for active profile: %v\n", err)
		return ""
	}

	// Update the in-memory value
	s.activeProfile = settings.ActiveProfileID
	return s.activeProfile
}

// GetWeightTracking returns the current weight tracking state from persistent storage
func (s *FoodService) GetWeightTracking() bool {
	settings, err := s.settingsStore.Load()
	if err != nil {
		log.Printf("Failed to load settings: %v, using default value false", err)
		return false
	}

	return settings.WeightTracking
}

// SetWeightTracking sets the weight tracking state and persists it
func (s *FoodService) SetWeightTracking(enabled bool) error {
	settings, err := s.settingsStore.Load()
	if err != nil {
		return fmt.Errorf("failed to load settings: %v", err)
	}

	settings.WeightTracking = enabled
	if err := s.settingsStore.Save(settings); err != nil {
		return fmt.Errorf("failed to save settings: %v", err)
	}

	return nil
}

// GetAutoRecalculateNutritionValues returns the current auto recalculate nutrition values state from persistent storage
func (s *FoodService) GetAutoRecalculateNutritionValues() bool {
	settings, err := s.settingsStore.Load()
	if err != nil {
		log.Printf("Failed to load settings: %v, using default value false", err)
		return false
	}

	return settings.AutoRecalculateNutritionValues
}

// SetAutoRecalculateNutritionValues sets the auto recalculate nutrition values state and persists it
func (s *FoodService) SetAutoRecalculateNutritionValues(enabled bool) error {
	settings, err := s.settingsStore.Load()
	if err != nil {
		return fmt.Errorf("failed to load settings: %v", err)
	}

	settings.AutoRecalculateNutritionValues = enabled
	if err := s.settingsStore.Save(settings); err != nil {
		return fmt.Errorf("failed to save settings: %v", err)
	}

	return nil
}

// CalculateAge calculates the age in years from a birthdate string in YYYY-MM-DD format
func CalculateAge(birthDate string) int {
	// Parse the birthdate
	birth, err := time.Parse("2006-01-02", birthDate)
	if err != nil {
		log.Printf("Error parsing birthdate '%s': %v", birthDate, err)
		return 0
	}

	// Get current time
	now := time.Now()

	// Calculate the difference in years
	years := now.Year() - birth.Year()

	// If birthday hasn't occurred yet this year, subtract one year
	birthdayThisYear := time.Date(now.Year(), birth.Month(), birth.Day(), 0, 0, 0, 0, time.UTC)
	if now.Before(birthdayThisYear) {
		years--
	}

	// Ensure age is not negative
	if years < 0 {
		years = 0
	}

	return years
}

// CalculateNutrition calculates the daily nutrition targets based on user metrics
func (s *FoodService) CalculateNutrition(
	weight float64,
	height float64,
	age int,
	gender string,
	activityLevel int,
	weeklyWeightChange float64,
) *types.NutritionCalculationResponse {
	var bmr float64
	switch gender {
	case "male":
		bmr = 10*weight + 6.25*height - 5*float64(age) + 5
	case "female":
		bmr = 10*weight + 6.25*height - 5*float64(age) - 161
	default: // for 'other' use average of male and female formulas
		bmr = 10*weight + 6.25*height - 5*float64(age) - 78
	}

	activityFactors := []float64{1.2, 1.375, 1.55, 1.725, 1.9}
	maintenanceCalories := bmr * activityFactors[activityLevel]

	// Convert weekly weight change to daily calorie adjustment
	// 1 kg of body fat is approximately 7700 calories
	// So to lose/gain 1 kg per week, we need a daily deficit/surplus of 1100 calories
	dailyCalorieAdjustment := weeklyWeightChange * 1100
	adjustedCalories := maintenanceCalories + dailyCalorieAdjustment

	proteins := weight * 2                                 // 2g protein per kg body weight
	fat := (adjustedCalories * 0.3) / 9                    // 30% of calories from fat
	carbs := (adjustedCalories - (proteins*4 + fat*9)) / 4 // Rest from carbs

	fmt.Printf("Calories: %f, Proteins: %f, Carbs: %f, Fat: %f\n", adjustedCalories, proteins, carbs, fat)

	return &types.NutritionCalculationResponse{
		Calories: int(math.Round(adjustedCalories)),
		Proteins: int(math.Round(proteins)),
		Carbs:    int(math.Round(carbs)),
		Fat:      int(math.Round(fat)),
	}
}

// CalculateNutritionFromCaloriesAndWeight calculates macronutrients based on target calories and weight
func (s *FoodService) CalculateNutritionFromCaloriesAndWeight(
	calories float64,
	weight float64,
) *types.NutritionCalculationResponse {
	proteins := weight * 2                         // 2g protein per kg body weight
	fat := (calories * 0.3) / 9                    // 30% of calories from fat
	carbs := (calories - (proteins*4 + fat*9)) / 4 // Rest from carbs

	return &types.NutritionCalculationResponse{
		Calories: int(math.Round(calories)),
		Proteins: int(math.Round(proteins)),
		Carbs:    int(math.Round(carbs)),
		Fat:      int(math.Round(fat)),
	}
}

// onDayChangeMonitor runs in a separate goroutine to check for day changes
// and trigger nutrition recalculation when needed
func (s *FoodService) onDayChangeMonitor() {
	// Run immediately on startup
	s.checkAndRecalculate()

	// Then run every hour to check if it's a new day
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.checkAndRecalculate()
	}
}

// checkAndRecalculate checks if we need to recalculate nutrition values
// based on the current date and settings
func (s *FoodService) checkAndRecalculate() {
	now := time.Now()

	// If we've already checked today, skip
	if s.lastChecked.Year() == now.Year() &&
		s.lastChecked.YearDay() == now.YearDay() {
		return
	}

	// Only check once per hour at most
	if time.Since(s.lastChecked) < time.Hour {
		return
	}

	s.lastChecked = now

	// Check if auto-recalculation is enabled
	enabled := s.GetAutoRecalculateNutritionValues()
	if !enabled {
		log.Println("Auto-recalculation of nutrition values is disabled")
		return
	}

	log.Println("Performing daily nutrition values check...")

	// Get all profiles
	profiles, err := s.GetAllProfiles()
	if err != nil {
		log.Printf("Error getting profiles for nutrition recalculation: %v", err)
		return
	}

	// Recalculate for each profile
	for _, profile := range profiles {
		s.recalculateForProfile(profile.ID)
	}
}

// recalculateForProfile recalculates nutrition values for a specific profile
func (s *FoodService) recalculateForProfile(profileID string) {
	// Get current user settings
	settings, err := s.GetUserSettings(profileID)
	if err != nil {
		log.Printf("Error getting user settings for profile %s: %v", profileID, err)
		return
	}

	// Only recalculate if we have all required data
	if settings.Weight > 0 && settings.Height > 0 && settings.BirthDate != "" && settings.Gender != "" {
		log.Printf("Recalculating nutrition values for profile %s", profileID)

		// Calculate age from birthdate
		age := CalculateAge(settings.BirthDate)

		// Calculate new nutrition values
		calculationResponse := s.CalculateNutrition(settings.Weight, settings.Height, age, settings.Gender, settings.ActivityLevel, settings.WeeklyWeightChange)
		if calculationResponse == nil {
			log.Printf("Error calculating nutrition values for profile %s", profileID)
			return
		}

		// Save settings to trigger recalculation (the existing SaveUserSettings handles the calculation)
		err = s.SaveUserSettings(settings, profileID)
		if err != nil {
			log.Printf("Error saving recalculated nutrition values for profile %s: %v", profileID, err)
		}
	} else {
		log.Printf("Skipping nutrition recalculation for profile %s - missing required data", profileID)
	}
}
