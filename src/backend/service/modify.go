package service

import (
	"nutrack/backend/data"
)

// Set default values for PersistentFoodItem if fields are invalid or empty
func ModifyPersistentFoodItem(item *data.PersistentFoodItem) {
	if item.Name == "" {
		item.Name = "No product name"
	}
	if item.CaloriesPer100g == 0 {
		item.CaloriesPer100g = 0
	}
	if item.ProteinPer100g == 0 {
		item.ProteinPer100g = 0
	}
	if item.CarbsPer100g == 0 {
		item.CarbsPer100g = 0
	}
	if item.FatPer100g == 0 {
		item.FatPer100g = 0
	}
	if item.ServingQuantity == 0 {
		item.ServingQuantity = 100
	}
	if item.ServingQuantityUnit == "" {
		item.ServingQuantityUnit = "g"
	}
}
