package service

import (
	"fmt"
	"time"

	"nutrack/backend/data"

	"github.com/google/uuid"
)

func ValidateBarcode(barcode string) error {
	if barcode == "" {
		return fmt.Errorf("barcode is required")
	}
	return nil
}

func ValidateServingQuantity(servingQuantity float64) error {
	if servingQuantity < 0 {
		return fmt.Errorf("serving quantity cannot be negative")
	}
	return nil
}

func ValidateServingQuantityUnit(servingQuantityUnit string) error {
	validUnits := map[string]bool{"g": true, "ml": true}
	if !validUnits[servingQuantityUnit] {
		return fmt.Errorf("serving quantity unit must be either 'g' or 'ml'")
	}
	return nil
}

func ValidateConsumedQuantity(consumedQuantity float64) error {
	if consumedQuantity <= 0 {
		return fmt.Errorf("consumed quantity must be positive")
	}
	return nil
}

func ValidateDate(date string) error {
	_, err := time.Parse("2006-01-02", date)
	if err != nil {
		return fmt.Errorf("invalid date format: use YYYY-MM-DD")
	}
	return nil
}

func ValidateActivityLevel(level int) error {
	if level < 0 || level > 4 {
		return fmt.Errorf("activity level must be between 0 and 4")
	}
	return nil
}

func ValidateGender(gender string) error {
	validGenders := map[string]bool{"male": true, "female": true, "undefined": true}
	if !validGenders[gender] {
		return fmt.Errorf("gender must be either 'male', 'female', or 'undefined'")
	}
	return nil
}

func ValidateWeeklyWeightChange(weightChange float64) error {
	if weightChange < -1 || weightChange > 1 {
		return fmt.Errorf("weekly weight change must be between -1 and 1 kg")
	}
	return nil
}

// placeholder, currently no validation
func ValidateName(name string) error {
	return nil
}

// ValidateNutrientValue checks if the nutrient value is non-negative
func ValidateNutrientValue(value float64, fieldName string) error {
	if value < 0 {
		return fmt.Errorf("%s cannot be negative", fieldName)
	}
	return nil
}

// ValidateTime checks if the time string is in the format HH:MM:SS
func ValidateTime(timeStr string) error {
	_, err := time.Parse("15:04:05", timeStr)
	if err != nil {
		return fmt.Errorf("invalid time format: use HH:MM:SS")
	}
	return nil
}

// ValidateDateTime checks if the time is valid
func ValidateDateTime(time time.Time) error {
	if time.IsZero() {
		return fmt.Errorf("invalid date time")
	}
	return nil
}

func ValidateUUID(id string) error {
	_, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid UUID format")
	}
	return nil
}

// ValidateSearchTerm checks if the search term is long enough
func ValidateSearchTerm(term string) error {
	if len(term) < 3 {
		return fmt.Errorf("search term must be at least 3 characters long")
	}
	return nil
}

func ValidateWeight(weight float64) error {
	if weight <= 0 {
		return fmt.Errorf("number must be positive")
	}
	return nil
}

func ValidateHeight(height float64) error {
	if height <= 0 {
		return fmt.Errorf("number must be positive")
	}
	return nil
}

func ValidateAge(age int) error {
	if age <= 0 {
		return fmt.Errorf("number must be positive")
	}
	return nil
}

func ValidatePersistentFoodItem(item data.PersistentFoodItem) error {
	if err := ValidateBarcode(item.Barcode); err != nil {
		return err
	}
	if err := ValidateName(item.Name); err != nil {
		return err
	}
	if err := ValidateNutrientValue(item.CaloriesPer100g, "calories"); err != nil {
		return err
	}
	if err := ValidateNutrientValue(item.ProteinPer100g, "protein"); err != nil {
		return err
	}
	if err := ValidateNutrientValue(item.CarbsPer100g, "carbs"); err != nil {
		return err
	}
	if err := ValidateNutrientValue(item.FatPer100g, "fat"); err != nil {
		return err
	}
	if err := ValidateServingQuantity(item.ServingQuantity); err != nil {
		return err
	}
	return nil
}

func ValidateConsumedFoodItem(item data.ConsumedFoodItem) error {
	if err := ValidateBarcode(item.Barcode); err != nil {
		return err
	}
	if err := ValidateConsumedQuantity(item.ConsumedQuantity); err != nil {
		return err
	}
	if err := ValidateServingQuantity(item.ServingQuantity); err != nil {
		return err
	}
	if err := ValidateDate(item.Date); err != nil {
		return err
	}
	if err := ValidateDateTime(item.InsertDate); err != nil {
		return err
	}
	return nil
}

func ValidateUserSettings(settings data.UserSettings) error {

	if err := ValidateNutrientValue(settings.Calories, "calories"); err != nil {
		return err
	}
	if err := ValidateNutrientValue(settings.Proteins, "proteins"); err != nil {
		return err
	}
	if err := ValidateNutrientValue(settings.Carbs, "carbs"); err != nil {
		return err
	}
	if err := ValidateNutrientValue(settings.Fat, "fat"); err != nil {
		return err
	}
	if err := ValidateActivityLevel(settings.ActivityLevel); err != nil {
		return err
	}
	if err := ValidateGender(settings.Gender); err != nil {
		return err
	}
	if err := ValidateWeeklyWeightChange(settings.WeeklyWeightChange); err != nil {
		return err
	}
	if err := ValidateWeight(settings.Weight); err != nil {
		return err
	}
	if err := ValidateHeight(settings.Height); err != nil {
		return err
	}
	if err := ValidateDate(settings.BirthDate); err != nil {
		return err
	}
	return nil
}

func ValidateField(fieldName string, value interface{}) error {
	switch fieldName {
	case "name":
		if strValue, ok := value.(string); ok {
			return ValidateName(strValue)
		}
		return fmt.Errorf("invalid type for name: expected string")
	case "energy-kcal_100g", "proteins_100g", "carbohydrates_100g", "fat_100g":
		if floatValue, ok := value.(float64); ok {
			return ValidateNutrientValue(floatValue, fieldName)
		}
		return fmt.Errorf("invalid type for %s: expected float64", fieldName)
	case "serving_quantity":
		if floatValue, ok := value.(float64); ok {
			return ValidateServingQuantity(floatValue)
		}
		return fmt.Errorf("invalid type for serving_quantity: expected float64")
	case "serving_quantity_unit":
		if strValue, ok := value.(string); ok {
			return ValidateServingQuantityUnit(strValue)
		}
		return fmt.Errorf("invalid type for serving_quantity_unit: expected string")
	case "consumed_quantity":
		if floatValue, ok := value.(float64); ok {
			return ValidateConsumedQuantity(floatValue)
		}
		return fmt.Errorf("invalid type for consumed_quantity: expected float64")
	case "date":
		if strValue, ok := value.(string); ok {
			return ValidateDate(strValue)
		}
		return fmt.Errorf("invalid type for date: expected string")
	case "time":
		if strValue, ok := value.(string); ok {
			return ValidateTime(strValue)
		}
		return fmt.Errorf("invalid type for time: expected string")
	case "profile_id":
		if strValue, ok := value.(string); ok {
			return ValidateUUID(strValue)
		}
		return fmt.Errorf("invalid type for profile_id: expected string")
	default:
		return fmt.Errorf("unknown field: %s", fieldName)
	}
}
