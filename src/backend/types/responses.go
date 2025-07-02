package types

// OpenFoodFactsResponse represents the response from the OpenFoodFacts API
type OpenFoodFactsResponse struct {
	Product struct {
		ProductName string `json:"product_name"`
		Nutriments  struct {
			EnergyKcal100g    interface{} `json:"energy-kcal_100g"`
			EnergyKj100g      interface{} `json:"energy-kj_100g"`
			Proteins100g      interface{} `json:"proteins_100g"`
			Carbohydrates100g interface{} `json:"carbohydrates_100g"`
			Fat100g           interface{} `json:"fat_100g"`
		} `json:"nutriments"`
		ServingQuantity     interface{} `json:"serving_quantity"`
		ServingQuantityUnit string      `json:"serving_quantity_unit"`
	} `json:"product"`
	Status interface{} `json:"status"`
}

// ApiResponse represents the response from the API
// NutritionCalculationResponse contains the calculated nutrition values
type NutritionCalculationResponse struct {
	Calories int `json:"calories"` // Daily calorie target
	Proteins int `json:"proteins"` // Daily protein target in grams
	Carbs    int `json:"carbs"`    // Daily carbs target in grams
	Fat      int `json:"fat"`      // Daily fat target in grams
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}
