package types

import (
	"time"
)

// PersistentFoodItem represents a food item in the database
type PersistentFoodItem struct {
	Barcode             string    `json:"barcode"`
	Name                string    `json:"name"`
	CaloriesPer100g     float64   `json:"energy-kcal_100g"`
	ProteinPer100g      float64   `json:"proteins_100g"`
	CarbsPer100g        float64   `json:"carbohydrates_100g"`
	FatPer100g          float64   `json:"fat_100g"`
	ServingQuantity     float64   `json:"serving_quantity"`
	ServingQuantityUnit string    `json:"serving_quantity_unit"`
	CreatedAt           time.Time `json:"created_at"`
	LastUpdated         time.Time `json:"last_updated"`
}

// ConsumedFoodItem represents a consumed food item
type ConsumedFoodItem struct {
	ID               string    `json:"id"`
	Barcode          string    `json:"barcode"`
	ConsumedQuantity float64   `json:"consumed_quantity"`
	ServingQuantity  float64   `json:"serving_quantity"`
	Date             string    `json:"date"`
	InsertDate       time.Time `json:"insert_date"`
}

// ConsumedFoodItemWithDetails represents a consumed food item with additional details
type ConsumedFoodItemWithDetails struct {
	ID                  string  `json:"id"`
	Barcode             string  `json:"barcode"`
	Name                string  `json:"name"`
	ConsumedQuantity    float64 `json:"consumed_quantity"`
	Date                string  `json:"date"`
	InsertDate          string  `json:"insert_date"`
	CaloriesPer100g     float64 `json:"calories_per_100g"`
	ProteinPer100g      float64 `json:"protein_per_100g"`
	CarbsPer100g        float64 `json:"carbs_per_100g"`
	FatPer100g          float64 `json:"fat_per_100g"`
	ServingQuantity     float64 `json:"serving_quantity"`
	ServingQuantityUnit string  `json:"serving_quantity_unit"`
}

// UserSettings represents user settings
type UserSettings struct {
	Weight             float64 `json:"weight"`
	Height             float64 `json:"height"`
	Calories           float64 `json:"calories"`
	Proteins           float64 `json:"proteins"`
	Carbs              float64 `json:"carbs"`
	Fat                float64 `json:"fat"`
	BirthDate          string  `json:"birth_date"`
	Gender             string  `json:"gender"`
	ActivityLevel      int     `json:"activity_level"`
	WeeklyWeightChange float64 `json:"weekly_weight_change"`
}

// Dish represents a dish
type Dish struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Barcode     *string   `json:"barcode,omitempty"` // Optional Barcode
	CreatedAt   time.Time `json:"created_at"`
	LastUpdated time.Time `json:"last_updated"`
}

// DishItem represents an ingredient in a dish
type DishItem struct {
	DishID   string  `json:"dish_id"`
	Barcode  string  `json:"barcode"`
	Quantity float64 `json:"quantity"`
}

// DetailedDishItem represents an ingredient with detailed information
type DetailedDishItem struct {
	Items    PersistentFoodItem `json:"food_item"`
	Quantity float64            `json:"quantity"`
}

// DishWithDetailedItems represents a dish with detailed ingredients
type DishWithDetailedItems struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Barcode     *string            `json:"barcode,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	LastUpdated time.Time          `json:"last_updated"`
	Items       []DetailedDishItem `json:"dish_items"`
}

// Profile represents a user profile
type Profile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}
