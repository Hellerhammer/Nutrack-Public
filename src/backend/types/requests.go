package types

// BarcodeRequest contains the request for a barcode scan
type BarcodeRequest struct {
	Barcode string `json:"barcode"`
}

// NutritionCalculationRequest contains the request for nutrition calculation
type NutritionCalculationRequest struct {
	Weight             float64 `json:"weight"`             // in kg
	Height             float64 `json:"height"`             // in cm
	Age                int     `json:"age"`                // in years
	Gender             string  `json:"gender"`             // "male", "female", or "other"
	ActivityLevel      int     `json:"activityLevel"`      // 0-4 (sedentary to very active)
	WeeklyWeightChange float64 `json:"weeklyWeightChange"` // in kg per week
}

type NutritionCalculationFromCaloriesAndWeightRequest struct {
	Calories float64 `json:"calories"`
	Weight   float64 `json:"weight"`
}

// TokenRequest contains the request for a token
type TokenRequest struct {
	Code         string `json:"code" binding:"required"`
	CodeVerifier string `json:"codeVerifier" binding:"required"`
}

// ConsumedFoodItemRequest contains the request for a consumed food item
type ConsumedFoodItemRequest struct {
	Barcode          string  `json:"barcode"`
	ConsumedQuantity float64 `json:"consumed_quantity"`
	Date             string  `json:"date"`
	ProfileID        string  `json:"profile_id"`
	ForceSync        bool    `json:"force_sync"`
}

// BatchConsumedFoodItemRequest contains multiple consumed food items
type BatchConsumedFoodItemRequest struct {
	Items     []ConsumedFoodItemRequest `json:"items"`
	ForceSync bool                      `json:"force_sync"`
}

// DropboxAutosyncRequest contains the request for dropbox autosync
type DropboxAutosyncRequest struct {
	Enabled bool `json:"enabled"`
}

// ActiveProfileRequest sets the active profile
type ActiveProfileRequest struct {
	ProfileID string `json:"profile_id"`
}

// ActiveScannerRequest sets the active scanner
type ActiveScannerRequest struct {
	Path *string `json:"path"` // Use pointer to handle null values
}

// WeightTrackingRequest sets the weight tracking status
type WeightTrackingRequest struct {
	Enabled bool `json:"enabled"`
}

// AutoRecalculateNutritionValuesRequest sets the auto recalculate nutrition values status
type AutoRecalculateNutritionValuesRequest struct {
	Enabled bool `json:"enabled"`
}

// ForceSyncRequest contains the request for a force sync
type ForceSyncRequest struct {
	Force bool `json:"force"`
}

// DishRequest contains the request for a dish
type DishRequest struct {
	Dish struct {
		Name    string  `json:"name"`
		Barcode *string `json:"barcode,omitempty"`
	} `json:"dish"`
	Items []struct {
		Barcode  string  `json:"barcode"`
		Quantity float64 `json:"quantity"`
	} `json:"items"`
}
