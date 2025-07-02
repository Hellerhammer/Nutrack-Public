package data

import (
	"database/sql"
	"fmt"
	"log"
	"nutrack/backend/messaging"
	"nutrack/backend/settings"
	"os"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/google/uuid"
)

// FormatDateTimeISO8601 formats a time.Time to ISO 8601 format with UTC timezone
// Example output: YYYY-MM-DDTHH:MM:SS.MMMZ
func FormatDateTimeISO8601(t time.Time) string {
	// Convert to UTC and format with milliseconds and Z suffix
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

// TestISO8601DateFormat inserts a test food item and retrieves it to verify ISO 8601 datetime format
// This is a utility function for testing purposes only
func TestISO8601DateFormat() (string, error) {
	// Create a test food item
	testItem := PersistentFoodItem{
		Barcode:             "TEST-ISO8601",
		Name:                "ISO 8601 Test Item",
		CaloriesPer100g:     100,
		FatPer100g:          10,
		CarbsPer100g:        20,
		ProteinPer100g:      30,
		ServingQuantity:     1,
		ServingQuantityUnit: "piece",
	}

	// Insert the test item
	err := InsertFoodItem(testItem)
	if err != nil {
		return "", fmt.Errorf("failed to insert test item: %v", err)
	}

	// Query the database directly to check the datetime format
	db := OpenDataBase()
	defer CloseDataBase(db)

	var createdAt string
	query := "SELECT created_at FROM foodItems WHERE barcode = ?"
	err = db.QueryRow(query, testItem.Barcode).Scan(&createdAt)
	if err != nil {
		return "", fmt.Errorf("failed to query test item: %v", err)
	}

	// Delete the test item to clean up
	_, err = db.Exec("DELETE FROM foodItems WHERE barcode = ?", testItem.Barcode)
	if err != nil {
		fmt.Printf("Warning: Failed to delete test item: %v\n", err)
	}

	return createdAt, nil
}

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

type ConsumedFoodItem struct {
	ID               string    `json:"id"`
	Barcode          string    `json:"barcode"`
	ConsumedQuantity float64   `json:"consumed_quantity"`
	ServingQuantity  float64   `json:"serving_quantity"`
	Date             string    `json:"date"`
	InsertDate       time.Time `json:"insert_date"`
}

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

type Dish struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Barcode     *string   `json:"barcode,omitempty"` // Optional Barcode
	CreatedAt   time.Time `json:"created_at"`
	LastUpdated time.Time `json:"last_updated"`
}

type DishItem struct {
	DishID   string  `json:"dish_id"`
	Barcode  string  `json:"barcode"`
	Quantity float64 `json:"quantity"`
}

type DetailedDishItem struct {
	Items    PersistentFoodItem `json:"food_item"`
	Quantity float64            `json:"quantity"`
}

type DishWithDetailedItems struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Barcode     *string            `json:"barcode,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	LastUpdated time.Time          `json:"last_updated"`
	Items       []DetailedDishItem `json:"dish_items"`
}

type Profile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func InitDatabase() {
	db := OpenDataBase()
	defer CloseDataBase(db)

	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS profiles (
		id VARCHAR(36) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS foodItems (
			barcode TEXT PRIMARY KEY,
			name TEXT,
			kcalPer100g REAL,
			fatPer100g REAL,
			carbsPer100g REAL,
			proteinPer100g REAL,
			servingQuantity REAL,
			servingQuantityUnit TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_updated DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS consumedFoodItems (
        id TEXT PRIMARY KEY,
        barcode TEXT NOT NULL,
        consumed_quantity REAL NOT NULL,
		serving_quantity REAL,
        date TEXT NOT NULL,
        insertdate TEXT NOT NULL,
		profile_id VARCHAR(36),
		FOREIGN KEY (profile_id) REFERENCES profiles(id)
    )
    `)
	if err != nil {
		log.Fatal(err)
	}

	// Create index on date column
	_, err = db.Exec(`
    CREATE INDEX IF NOT EXISTS idx_consumed_food_items_date ON consumedFoodItems(date)
    `)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS userSettings (
		profile_id VARCHAR(36) PRIMARY KEY,
		weight REAL,
		height REAL,
		calories REAL,
		proteins REAL,
		carbs REAL,
		fat REAL,
		birthdate TEXT,
		gender TEXT,
		activity_level INTEGER CHECK (activity_level BETWEEN 0 AND 4),
		weekly_weight_change REAL,
		FOREIGN KEY (profile_id) REFERENCES profiles(id)
	)
    `)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS dishes (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        barcode TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        last_updated DATETIME DEFAULT CURRENT_TIMESTAMP
    )
    `)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS dish_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		dish_id TEXT,
		barcode TEXT,
		quantity REAL NOT NULL,
		FOREIGN KEY (dish_id) REFERENCES dishes(id),
		FOREIGN KEY (barcode) REFERENCES foodItems(barcode)
	)
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
	 CREATE TABLE IF NOT EXISTS weight_tracking (
            id TEXT PRIMARY KEY,
            profile_id VARCHAR(36) NOT NULL,
            weight REAL NOT NULL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
        )
    `)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
	CREATE INDEX IF NOT EXISTS idx_weight_tracking_created_at ON weight_tracking(created_at)`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
	CREATE INDEX IF NOT EXISTS idx_weight_tracking_profile_id ON weight_tracking(profile_id)`)
	if err != nil {
		log.Fatal(err)
	}

	err = MigrateDatabase()
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
}

func MigrateDatabase() error {
	db := OpenDataBase()
	defer CloseDataBase(db)

	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	var userSettingsProfile_idColumnExists bool
	err = tx.QueryRow("SELECT COUNT(*) FROM pragma_table_info('userSettings') WHERE name='profile_id'").Scan(&userSettingsProfile_idColumnExists)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to check if profile_id column exists: %v", err)
	}

	if !userSettingsProfile_idColumnExists {
		_, err = tx.Exec("ALTER TABLE userSettings ADD COLUMN profile_id VARCHAR(36)")
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to add profile_id column: %v", err)
		}
	}

	var consumedFoodItemsProfile_idColumnExists bool
	err = tx.QueryRow("SELECT COUNT(*) FROM pragma_table_info('consumedFoodItems') WHERE name='profile_id'").Scan(&consumedFoodItemsProfile_idColumnExists)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to check if profile_id column exists: %v", err)
	}

	if !consumedFoodItemsProfile_idColumnExists {
		_, err = tx.Exec("ALTER TABLE consumedFoodItems ADD COLUMN profile_id VARCHAR(36)")
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to add profile_id column: %v", err)
		}
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	// Run all pending migrations
	err = RunMigrations()
	if err != nil {
		return fmt.Errorf("failed to run migrations: %v", err)
	}

	return nil
}

func OpenDataBase() *sql.DB {
	dbPath := GetDBPath()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func GetDBPath() string {
	// Check if running in container
	if os.Getenv("CONTAINER") == "true" {
		return "/app/data/nutrack.db"
	} else {
		dataDir := os.Getenv("DATA_DIR")
		if dataDir == "" {
			dataDir = ".."
		}
		return dataDir + "/nutrack.db"
	}

}

func CloseDataBase(db *sql.DB) {
	db.Close()
}

// BroadcastMessage sends a message to all connected clients,
// using either SSE or StandardIO depending on the configuration

func InsertFoodItem(item PersistentFoodItem) error {
	// check if barcode is empty
	if item.Barcode == "" {
		println("Barcode is required")
		return fmt.Errorf("barcode is required")
	}

	db := OpenDataBase()
	defer CloseDataBase(db)

	query := `
    INSERT INTO foodItems (barcode, name, kcalPer100g, fatPer100g, carbsPer100g, proteinPer100g, servingQuantity, servingQuantityUnit, created_at, last_updated)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `

	now := time.Now()
	// Format datetime in ISO 8601 format
	createdAtISO := FormatDateTimeISO8601(now)
	lastUpdatedISO := FormatDateTimeISO8601(now)

	fmt.Println("Inserting food item: ", item, "at time: ", createdAtISO)
	_, err := db.Exec(query,
		item.Barcode,
		item.Name,
		item.CaloriesPer100g,
		item.FatPer100g,
		item.CarbsPer100g,
		item.ProteinPer100g,
		item.ServingQuantity,
		item.ServingQuantityUnit,
		createdAtISO,
		lastUpdatedISO,
	)

	if err != nil {
		return fmt.Errorf("failed to insert food item: %v", err)
	}

	println("Inserted food item: "+item.Name, item.Barcode, item.CaloriesPer100g, item.FatPer100g, item.CarbsPer100g, item.ProteinPer100g, item.ServingQuantity, item.ServingQuantityUnit)
	if err := markDatabaseAsUnsynced(); err != nil {
		log.Printf("Failed to mark database as unsynced: %v", err)
	}
	messaging.BroadcastMessage("food_items_updated")
	return nil
}

func CheckFoodItemExists(item PersistentFoodItem) (bool, error) {
	db := OpenDataBase()
	defer CloseDataBase(db)

	// Prüfen, ob der Barcode bereits existiert
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM foodItems WHERE barcode = ?)", item.Barcode).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error checking if barcode exists: %v", err)
	}

	return exists, nil
}

func GetFoodItemByBarcode(db *sql.DB, barcode string) (PersistentFoodItem, error) {
	query := `
    SELECT 
        barcode, 
        name, 
        COALESCE(kcalPer100g, 0) as kcalPer100g, 
        COALESCE(fatPer100g, 0) as fatPer100g, 
        COALESCE(carbsPer100g, 0) as carbsPer100g, 
        COALESCE(proteinPer100g, 0) as proteinPer100g, 
        COALESCE(servingQuantity, 0) as servingQuantity, 
        COALESCE(servingQuantityUnit, '') as servingQuantityUnit
    FROM foodItems
    WHERE barcode = ?
    `

	var item PersistentFoodItem
	err := db.QueryRow(query, barcode).Scan(
		&item.Barcode,
		&item.Name,
		&item.CaloriesPer100g,
		&item.FatPer100g,
		&item.CarbsPer100g,
		&item.ProteinPer100g,
		&item.ServingQuantity,
		&item.ServingQuantityUnit,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return PersistentFoodItem{}, fmt.Errorf("no food item found with barcode %s", barcode)
		}
		return PersistentFoodItem{}, fmt.Errorf("failed to query food item: %v", err)
	}

	return item, nil
}

func GetAllFoodItems() ([]PersistentFoodItem, error) {
	db := OpenDataBase()
	defer CloseDataBase(db)

	query := `
    SELECT 
        barcode, 
        name, 
        COALESCE(kcalPer100g, 0) as kcalPer100g, 
        COALESCE(fatPer100g, 0) as fatPer100g, 
        COALESCE(carbsPer100g, 0) as carbsPer100g, 
        COALESCE(proteinPer100g, 0) as proteinPer100g, 
        COALESCE(servingQuantity, 0) as servingQuantity, 
        COALESCE(servingQuantityUnit, '') as servingQuantityUnit,
        created_at,
        last_updated
    FROM foodItems
    ORDER BY created_at DESC
    `

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query food items: %v", err)
	}
	defer rows.Close()

	var foodItems []PersistentFoodItem

	for rows.Next() {
		var item PersistentFoodItem
		err := rows.Scan(
			&item.Barcode,
			&item.Name,
			&item.CaloriesPer100g,
			&item.FatPer100g,
			&item.CarbsPer100g,
			&item.ProteinPer100g,
			&item.ServingQuantity,
			&item.ServingQuantityUnit,
			&item.CreatedAt,
			&item.LastUpdated,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan food item: %v", err)
		}
		foodItems = append(foodItems, item)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating food items: %v", err)
	}

	return foodItems, nil
}

func UpdateFoodItem(barcode string, updateData map[string]interface{}) error {
	db := OpenDataBase()
	defer CloseDataBase(db)

	// Build the SQL query dynamically based on the fields to update

	query := "UPDATE foodItems SET "
	var args []interface{}
	for field, value := range updateData {
		switch field {
		case "name":
			query += "name = ?, "
		case "energy-kcal_100g":
			query += "kcalPer100g = ?, "
		case "proteins_100g":
			query += "proteinPer100g = ?, "
		case "carbohydrates_100g":
			query += "carbsPer100g = ?, "
		case "fat_100g":
			query += "fatPer100g = ?, "
		case "serving_quantity":
			query += "servingQuantity = ?, "
		case "serving_quantity_unit":
			query += "servingQuantityUnit = ?, "
		default:
			fmt.Println("Unknown field:", field)
			continue // Skip unknown fields
		}
		args = append(args, value)
	}

	if len(args) == 0 {
		return fmt.Errorf("no valid fields to update")
	}

	// Add last_updated field
	query += "last_updated = ? "
	args = append(args, time.Now())

	// Remove the trailing comma if it exists
	if query[len(query)-3:len(query)-1] == ", " {
		query = query[:len(query)-3] + query[len(query)-1:]
	}

	query += "WHERE barcode = ?"
	args = append(args, barcode)

	_, err := db.Exec(query, args...)
	if err != nil {
		fmt.Printf("Error updating food item: %v", err)
		return fmt.Errorf("failed to update food item: %v", err)
	}

	if err := markDatabaseAsUnsynced(); err != nil {
		log.Printf("Failed to mark database as unsynced: %v", err)
	}
	messaging.BroadcastMessage("food_items_updated")
	return nil
}

func DeleteFoodItem(barcode string) error {

	db := OpenDataBase()
	defer CloseDataBase(db)

	query := "DELETE FROM foodItems WHERE barcode = ?"

	result, err := db.Exec(query, barcode)
	if err != nil {
		return fmt.Errorf("failed to delete food item: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no food item found with barcode %s", barcode)
	}

	if err := markDatabaseAsUnsynced(); err != nil {
		log.Printf("Failed to mark database as unsynced: %v", err)
	}
	messaging.BroadcastMessage("food_items_updated")
	return nil
}

func SearchFoodItems(query string) ([]PersistentFoodItem, error) {
	if len(query) < 2 {
		return nil, fmt.Errorf("search query must be at least 2 characters long")
	}

	db := OpenDataBase()
	defer CloseDataBase(db)

	// Create a temporary trigger to create an index if it doesn't exist
	_, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_fooditems_name ON foodItems(name COLLATE NOCASE);
		CREATE INDEX IF NOT EXISTS idx_fooditems_barcode ON foodItems(barcode);
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create index: %v", err)
	}

	sqlQuery := `
    SELECT 
        barcode, 
        name, 
        COALESCE(kcalPer100g, 0) as kcalPer100g, 
        COALESCE(fatPer100g, 0) as fatPer100g, 
        COALESCE(carbsPer100g, 0) as carbsPer100g, 
        COALESCE(proteinPer100g, 0) as proteinPer100g, 
        COALESCE(servingQuantity, 0) as servingQuantity, 
        COALESCE(servingQuantityUnit, '') as servingQuantityUnit
    FROM foodItems
    WHERE 
        name LIKE ? COLLATE NOCASE 
        OR barcode LIKE ? 
    ORDER BY 
        CASE 
            WHEN name LIKE ? THEN 1  -- Exact match at start
            WHEN name LIKE ? THEN 2  -- Match at start
            WHEN name LIKE ? THEN 3  -- Match anywhere
            ELSE 4                   -- Barcode match
        END,
        length(name),               -- Prefer shorter names
        name COLLATE NOCASE
    LIMIT 15
    `

	// Prepare search patterns
	term := "%" + query + "%"
	exactStart := query + "%"

	rows, err := db.Query(sqlQuery, term, query+"%", exactStart, exactStart, term)
	if err != nil {
		return nil, fmt.Errorf("failed to query food items: %v", err)
	}
	defer rows.Close()

	var foodItems []PersistentFoodItem

	for rows.Next() {
		var item PersistentFoodItem
		err := rows.Scan(
			&item.Barcode,
			&item.Name,
			&item.CaloriesPer100g,
			&item.FatPer100g,
			&item.CarbsPer100g,
			&item.ProteinPer100g,
			&item.ServingQuantity,
			&item.ServingQuantityUnit,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan food item: %v", err)
		}
		foodItems = append(foodItems, item)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating food items: %v", err)
	}

	return foodItems, nil
}

func InsertConsumedFoodItem(item ConsumedFoodItem, profileID string) error {

	db := OpenDataBase()
	defer CloseDataBase(db)

	// Format insertDate in ISO 8601 format
	insertDateISO := FormatDateTimeISO8601(item.InsertDate)
	fmt.Println("Inserting consumed food item: ", item, "with ISO date: ", insertDateISO)

	query := `
    INSERT INTO consumedFoodItems (id, barcode, consumed_quantity, serving_quantity, date, insertdate, profile_id)
    VALUES (?, ?, ?, ?, ?, ?, ?)
    `

	_, err := db.Exec(query,
		item.ID,
		item.Barcode,
		item.ConsumedQuantity,
		item.ServingQuantity,
		item.Date,
		insertDateISO, // Use ISO 8601 formatted datetime
		profileID,
	)

	if err != nil {
		return fmt.Errorf("failed to insert consumed food item: %v", err)
	}
	if err := markDatabaseAsUnsynced(); err != nil {
		log.Printf("Failed to mark database as unsynced: %v", err)
	}
	messaging.BroadcastMessage("consumed_food_items_updated")
	return nil
}

// DeleteConsumedFoodItem löscht einen konsumierten Lebensmitteleintrag basierend auf der ID
func DeleteConsumedFoodItem(id string) error {
	db := OpenDataBase()
	defer CloseDataBase(db)

	query := "DELETE FROM consumedFoodItems WHERE id = ?"

	result, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete consumed food item: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no consumed food item found with id %s", id)
	}

	if err := markDatabaseAsUnsynced(); err != nil {
		log.Printf("Failed to mark database as unsynced: %v", err)
	}
	messaging.BroadcastMessage("consumed_food_items_updated")
	return nil
}

func GetConsumedFoodItemsByDate(date string, profileID string) ([]ConsumedFoodItemWithDetails, error) {
	db := OpenDataBase()
	defer CloseDataBase(db)

	query := `
    SELECT 
        c.id, 
        c.barcode, 
        f.name, 
        c.consumed_quantity, 
        c.serving_quantity, 
        c.date, 
        c.insertdate,
        COALESCE(f.kcalPer100g, 0) as kcalPer100g, 
        COALESCE(f.proteinPer100g, 0) as proteinPer100g, 
        COALESCE(f.carbsPer100g, 0) as carbsPer100g, 
        COALESCE(f.fatPer100g, 0) as fatPer100g, 
        COALESCE(f.servingQuantityUnit, '') as servingQuantityUnit
    FROM consumedFoodItems c
    JOIN foodItems f ON c.barcode = f.barcode
    WHERE c.date = ? AND c.profile_id = ?
    ORDER BY c.insertdate DESC
    `

	rows, err := db.Query(query, date, profileID)
	if err != nil {
		fmt.Println("Error querying consumed food items:", err)
		return nil, fmt.Errorf("failed to query consumed food items: %v", err)
	}
	defer rows.Close()

	var consumedFoodItems []ConsumedFoodItemWithDetails

	for rows.Next() {
		var item ConsumedFoodItemWithDetails
		err := rows.Scan(
			&item.ID,
			&item.Barcode,
			&item.Name,
			&item.ConsumedQuantity,
			&item.ServingQuantity,
			&item.Date,
			&item.InsertDate,
			&item.CaloriesPer100g,
			&item.ProteinPer100g,
			&item.CarbsPer100g,
			&item.FatPer100g,
			&item.ServingQuantityUnit,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan consumed food item: %v", err)
		}
		consumedFoodItems = append(consumedFoodItems, item)
	}

	return consumedFoodItems, nil
}

func GetConsumedFoodItemByBarcodeAndDate(barcode, date string, profileID string) (*ConsumedFoodItem, error) {
	db := OpenDataBase()
	defer CloseDataBase(db)

	query := `
    SELECT id, barcode, consumed_quantity, serving_quantity, date, insertdate
    FROM consumedFoodItems
    WHERE barcode = ? AND date = ? AND profile_id = ?
    LIMIT 1
    `

	var item ConsumedFoodItem
	err := db.QueryRow(query, barcode, date, profileID).Scan(
		&item.ID,
		&item.Barcode,
		&item.ConsumedQuantity,
		&item.ServingQuantity,
		&item.Date,
		&item.InsertDate,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get consumed food item: %v", err)
	}

	return &item, nil
}

func UpdateConsumedFoodItem(id string, updateData map[string]interface{}) error {

	db := OpenDataBase()
	defer CloseDataBase(db)

	println("updateData", updateData)

	var existingProfileID string
	err := db.QueryRow("SELECT profile_id FROM consumedFoodItems WHERE id = ?", id).Scan(&existingProfileID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no consumed food item found with id %s", id)
		}
		return fmt.Errorf("failed to verify item ownership: %v", err)
	}
	// Build the SQL query dynamically based on the fields to update
	query := "UPDATE consumedFoodItems SET "
	var args []interface{}
	for field, value := range updateData {
		switch field {
		case "consumed_quantity":
			query += "consumed_quantity = ?, "
			args = append(args, value)
		case "serving_quantity":
			query += "serving_quantity = ?, "
			args = append(args, value)
		case "date":
			query += "date = ?, "
			args = append(args, value)
		case "profile_id":
			continue // Skip profile_id as we don't want to update it
		default:
			fmt.Println("Unknown field:", field)
			continue // Skip unknown fields
		}
	}

	if len(args) == 0 {
		return fmt.Errorf("no valid fields to update")
	}

	// Remove the trailing comma and space
	query = query[:len(query)-2]

	// Add the WHERE clause with both id and profile_id
	query += " WHERE id = ?"
	args = append(args, id)

	_, err = db.Exec(query, args...)
	if err != nil {
		fmt.Printf("Error updating consumed food item: %v", err)
		return fmt.Errorf("failed to update consumed food item: %v", err)
	}

	if err := markDatabaseAsUnsynced(); err != nil {
		log.Printf("Failed to mark database as unsynced: %v", err)
	}
	messaging.BroadcastMessage("consumed_food_items_updated")
	return nil
}

func GetConsumedFoodItemById(id string) (*ConsumedFoodItem, error) {
	db := OpenDataBase()
	defer CloseDataBase(db)

	query := `
    SELECT id, barcode, consumed_quantity, date, time
    FROM consumedFoodItems
    WHERE id = ?
    `

	var item ConsumedFoodItem
	err := db.QueryRow(query, id).Scan(
		&item.ID,
		&item.Barcode,
		&item.ConsumedQuantity,
		&item.Date,
		&item.InsertDate,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no consumed food item found with id %s", id)
	} else if err != nil {
		return nil, fmt.Errorf("failed to get consumed food item: %v", err)
	}

	return &item, nil
}

// GetServingQuantityByBarcode holt die Serving Quantity eines Lebensmittels basierend auf dem Barcode
func GetServingQuantityByBarcode(barcode string) (float64, error) {
	db := OpenDataBase()
	defer CloseDataBase(db)

	query := `
    SELECT servingQuantity
    FROM foodItems
    WHERE barcode = ?
    `

	var servingQuantity float64
	err := db.QueryRow(query, barcode).Scan(&servingQuantity)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("no food item found with barcode %s", barcode)
		}
		return 0, fmt.Errorf("failed to query serving quantity: %v", err)
	}

	return servingQuantity, nil
}

func SaveUserSettings(settings UserSettings, profileID string) error {
	db := OpenDataBase()
	defer CloseDataBase(db)

	// Überprüfe, ob das Profil existiert
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM profiles WHERE id = ?)", profileID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("error checking if profile exists: %v", err)
	}
	if !exists {
		return fmt.Errorf("profile with ID %s does not exist", profileID)
	}

	fmt.Println("settings", profileID, settings)

	query := `
    INSERT OR REPLACE INTO userSettings (profile_id, weight, height, calories, proteins, carbs, fat, birthdate, gender, activity_level, weekly_weight_change)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `

	_, err = db.Exec(query,
		profileID,
		settings.Weight,
		settings.Height,
		settings.Calories,
		settings.Proteins,
		settings.Carbs,
		settings.Fat,
		settings.BirthDate,
		settings.Gender,
		settings.ActivityLevel,
		settings.WeeklyWeightChange,
	)
	if err != nil {
		fmt.Println("error", err.Error())
		return fmt.Errorf("failed to save user settings: %v", err)
	}

	if err := markDatabaseAsUnsynced(); err != nil {
		log.Printf("Failed to mark database as unsynced: %v", err)
	}

	// Broadcast message for UI updates
	messaging.BroadcastMessage("user_settings_updated")
	return nil
}

func GetUserSettings(profileID string) (UserSettings, error) {
	db := OpenDataBase()
	defer CloseDataBase(db)

	query := `
    SELECT weight, height, calories, proteins, carbs, fat, birthdate, gender, activity_level, weekly_weight_change
    FROM userSettings
    WHERE profile_id = ?
    `

	var settings UserSettings
	err := db.QueryRow(query, profileID).Scan(
		&settings.Weight,
		&settings.Height,
		&settings.Calories,
		&settings.Proteins,
		&settings.Carbs,
		&settings.Fat,
		&settings.BirthDate,
		&settings.Gender,
		&settings.ActivityLevel,
		&settings.WeeklyWeightChange,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return UserSettings{}, nil // Return empty settings if not found
		}
		return UserSettings{}, fmt.Errorf("failed to get user settings: %v", err)
	}

	return settings, nil
}

func CreateDish(dish Dish, items []DishItem) error {

	db := OpenDataBase()
	defer CloseDataBase(db)

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	// Insert the dish
	_, err = tx.Exec(`
		INSERT INTO dishes (id, name, barcode, created_at, last_updated)
		VALUES (?, ?, ?, ?, ?)
	`, dish.ID, dish.Name, dish.Barcode, time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to insert dish: %v", err)
	}

	// Insert the dish items
	for _, item := range items {
		_, err = tx.Exec(`
            INSERT INTO dish_items (dish_id, barcode, quantity)
            VALUES (?, ?, ?)
        `, dish.ID, item.Barcode, item.Quantity)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to insert dish item: %v", err)
		}
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	if err := markDatabaseAsUnsynced(); err != nil {
		log.Printf("Failed to mark database as unsynced: %v", err)
	}
	return nil
}

func DeleteDish(dishID string) error {

	db := OpenDataBase()
	defer CloseDataBase(db)

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	// Delete dish items first
	_, err = tx.Exec("DELETE FROM dish_items WHERE dish_id = ?", dishID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete dish items: %v", err)
	}

	// Then delete the dish itself
	_, err = tx.Exec("DELETE FROM dishes WHERE id = ?", dishID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete dish: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	if err := markDatabaseAsUnsynced(); err != nil {
		log.Printf("Failed to mark database as unsynced: %v", err)
	}
	return nil
}

func UpdateDish(dishID string, name string, barcode *string, items []DishItem) error {

	db := OpenDataBase()
	defer CloseDataBase(db)

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	// Update the dish name and barcode
	_, err = tx.Exec(`
		UPDATE dishes SET name = ?, barcode = ?, last_updated = ?
		WHERE id = ?
	`, name, barcode, time.Now(), dishID)
	if err != nil {
		return fmt.Errorf("failed to update dish: %v", err)
	}

	// Delete existing dish items
	_, err = tx.Exec("DELETE FROM dish_items WHERE dish_id = ?", dishID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete existing dish items: %v", err)
	}

	// Insert the new dish items
	for _, item := range items {
		_, err = tx.Exec(`
            INSERT INTO dish_items (dish_id, barcode, quantity)
            VALUES (?, ?, ?)
        `, dishID, item.Barcode, item.Quantity)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to insert dish item: %v", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	if err := markDatabaseAsUnsynced(); err != nil {
		log.Printf("Failed to mark database as unsynced: %v", err)
	}
	return nil
}

func InsertDishAsFoodItem(dishID string) error {
	db := OpenDataBase()
	defer CloseDataBase(db)

	// Calculate dish nutrition
	dishNutrition, err := CalculateDishNutrition(dishID)
	if err != nil {
		return fmt.Errorf("failed to calculate dish nutrition: %v", err)
	}
	// Get the dish name and barcode
	var dishName string
	var dishBarcode *string
	err = db.QueryRow("SELECT name, barcode FROM dishes WHERE id = ?", dishID).Scan(&dishName, &dishBarcode)
	if err != nil {
		return fmt.Errorf("failed to get dish details: %v", err)
	}

	// Use the dish's barcode if it exists, otherwise use the dish ID
	barcode := dishID
	if dishBarcode != nil {
		barcode = *dishBarcode
	}

	fmt.Println("Barcode:", barcode)
	// Check if the barcode already exists in the foodItems table
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM foodItems WHERE barcode = ?)", barcode).Scan(&exists)
	if err != nil {
		return fmt.Errorf("error checking if barcode exists: %v", err)
	}

	if exists {
		// Update the existing food item
		_, err = db.Exec(`
		UPDATE foodItems 
		SET name = ?, kcalPer100g = ?, fatPer100g = ?, carbsPer100g = ?, proteinPer100g = ?, 
		    servingQuantity = ?, servingQuantityUnit = ?, last_updated = ?
		WHERE barcode = ?
		`, dishName, dishNutrition.CaloriesPer100g, dishNutrition.FatPer100g,
			dishNutrition.CarbsPer100g, dishNutrition.ProteinPer100g,
			dishNutrition.ServingQuantity, dishNutrition.ServingQuantityUnit,
			time.Now(), barcode)
		if err != nil {
			return fmt.Errorf("failed to update existing food item: %v", err)
		}
	} else {
		// Insert the dish as a new food item
		_, err = db.Exec(`
		INSERT INTO foodItems (barcode, name, kcalPer100g, fatPer100g, carbsPer100g, proteinPer100g, 
		                       servingQuantity, servingQuantityUnit, created_at, last_updated)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, barcode, dishName, dishNutrition.CaloriesPer100g, dishNutrition.FatPer100g,
			dishNutrition.CarbsPer100g, dishNutrition.ProteinPer100g,
			dishNutrition.ServingQuantity, dishNutrition.ServingQuantityUnit,
			time.Now(), time.Now())
		if err != nil {
			return fmt.Errorf("failed to insert dish as food item: %v", err)
		}
	}

	if err := markDatabaseAsUnsynced(); err != nil {
		log.Printf("Failed to mark database as unsynced: %v", err)
	}
	messaging.BroadcastMessage("food_items_updated")
	return nil
}

func GetDishWithItems(dishID string) (Dish, []DishItem, error) {
	db := OpenDataBase()
	defer CloseDataBase(db)

	// Get dish details
	var dish Dish
	err := db.QueryRow(`
        SELECT id, name, barcode, created_at, last_updated
        FROM dishes
        WHERE id = ?
    `, dishID).Scan(&dish.ID, &dish.Name, &dish.Barcode, &dish.CreatedAt, &dish.LastUpdated)
	if err != nil {
		if err == sql.ErrNoRows {
			return Dish{}, nil, fmt.Errorf("dish not found with ID: %s", dishID)
		}
		return Dish{}, nil, fmt.Errorf("failed to get dish: %v", err)
	}

	// Get dish items
	rows, err := db.Query(`
        SELECT di.barcode, di.quantity, 
               f.name
        FROM dish_items di
        JOIN foodItems f ON di.barcode = f.barcode
        WHERE di.dish_id = ?
    `, dishID)
	if err != nil {
		return Dish{}, nil, fmt.Errorf("failed to get dish items: %v", err)
	}
	defer rows.Close()

	var items []DishItem
	for rows.Next() {
		var item DishItem
		var foodName string
		err := rows.Scan(&item.Barcode, &item.Quantity, &foodName)
		if err != nil {
			return Dish{}, nil, fmt.Errorf("failed to scan dish item: %v", err)
		}
		item.DishID = dishID
		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return Dish{}, nil, fmt.Errorf("error iterating dish items: %v", err)
	}

	return dish, items, nil
}

func GetDishItemsAsPersistentFoodItems(dishID string) ([]PersistentFoodItem, error) {
	db := OpenDataBase()
	defer CloseDataBase(db)

	query := `
    SELECT f.barcode, f.name, f.kcalPer100g, f.fatPer100g, f.carbsPer100g, f.proteinPer100g, 
           f.servingQuantity, f.servingQuantityUnit, SUM(di.quantity) as total_quantity
    FROM dish_items di
    JOIN foodItems f ON di.barcode = f.barcode
    WHERE di.dish_id = ?
    GROUP BY f.barcode
    `

	rows, err := db.Query(query, dishID)
	if err != nil {
		return nil, fmt.Errorf("failed to query dish items: %v", err)
	}
	defer rows.Close()

	var items []PersistentFoodItem
	for rows.Next() {
		var item PersistentFoodItem
		var quantity float64
		err := rows.Scan(
			&item.Barcode, &item.Name, &item.CaloriesPer100g, &item.FatPer100g,
			&item.CarbsPer100g, &item.ProteinPer100g, &item.ServingQuantity,
			&item.ServingQuantityUnit, &quantity)
		if err != nil {
			return nil, fmt.Errorf("failed to scan dish item: %v", err)
		}
		item.ServingQuantity = quantity
		items = append(items, item)
	}

	return items, nil
}

func GetAllDishes() ([]DishWithDetailedItems, error) {
	db := OpenDataBase()
	defer CloseDataBase(db)

	query := `
    SELECT d.id, d.name, d.barcode, d.created_at, d.last_updated
    FROM dishes d
    ORDER BY d.created_at DESC
    `

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query dishes: %v", err)
	}
	defer rows.Close()

	var dishes []DishWithDetailedItems

	for rows.Next() {
		var dish DishWithDetailedItems
		err := rows.Scan(
			&dish.ID,
			&dish.Name,
			&dish.Barcode,
			&dish.CreatedAt,
			&dish.LastUpdated,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan dish: %v", err)
		}

		// Fetch dish items with full food item details for each dish
		itemsQuery := `
        SELECT di.barcode, di.quantity, 
               f.name, f.kcalPer100g, f.fatPer100g, f.carbsPer100g, f.proteinPer100g, 
               f.servingQuantity, f.servingQuantityUnit
        FROM dish_items di
        JOIN foodItems f ON di.barcode = f.barcode
        WHERE di.dish_id = ?
        `
		itemRows, err := db.Query(itemsQuery, dish.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to query dish items: %v", err)
		}
		defer itemRows.Close()

		for itemRows.Next() {
			var item DetailedDishItem
			var foodItem PersistentFoodItem
			err := itemRows.Scan(
				&foodItem.Barcode, &item.Quantity,
				&foodItem.Name, &foodItem.CaloriesPer100g, &foodItem.FatPer100g,
				&foodItem.CarbsPer100g, &foodItem.ProteinPer100g,
				&foodItem.ServingQuantity, &foodItem.ServingQuantityUnit,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to scan dish item: %v", err)
			}
			item.Items = foodItem
			dish.Items = append(dish.Items, item)
		}

		dishes = append(dishes, dish)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating dishes: %v", err)
	}

	return dishes, nil
}

func CalculateDishNutrition(dishID string) (PersistentFoodItem, error) {
	db := OpenDataBase()
	defer CloseDataBase(db)

	var dish PersistentFoodItem
	dish.Barcode = dishID // Use the dish ID as the "barcode"

	var totalWeight float64
	var totalCalories, totalProtein, totalCarbs, totalFat float64

	rows, err := db.Query(`
	SELECT 
		di.quantity, 
		COALESCE(f.kcalPer100g, 0) as kcalPer100g, 
		COALESCE(f.proteinPer100g, 0) as proteinPer100g, 
		COALESCE(f.carbsPer100g, 0) as carbsPer100g, 
		COALESCE(f.fatPer100g, 0) as fatPer100g, 
		COALESCE(f.servingQuantity, 0) as servingQuantity, 
		COALESCE(f.servingQuantityUnit, '') as servingQuantityUnit
	FROM dish_items di
	JOIN foodItems f ON di.barcode = f.barcode
	WHERE di.dish_id = ?
	`, dishID)

	if err != nil {
		return dish, fmt.Errorf("failed to query dish items: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var quantity, calories, protein, carbs, fat, servingQuantity float64
		var servingQuantityUnit string

		err := rows.Scan(&quantity, &calories, &protein, &carbs, &fat, &servingQuantity, &servingQuantityUnit)
		if err != nil {
			return dish, fmt.Errorf("failed to scan dish item: %v", err)
		}

		weight := quantity
		if servingQuantityUnit != "g" && servingQuantity > 0 {
			weight = quantity * servingQuantity
		}

		totalWeight += weight
		totalCalories += (calories * weight) / 100
		totalProtein += (protein * weight) / 100
		totalCarbs += (carbs * weight) / 100
		totalFat += (fat * weight) / 100
	}

	// Get dish name
	var dishName string
	err = db.QueryRow("SELECT name FROM dishes WHERE id = ?", dishID).Scan(&dishName)
	if err != nil {
		return dish, fmt.Errorf("failed to get dish name: %v", err)
	}

	if totalWeight > 0 {
		dish.Name = dishName
		dish.CaloriesPer100g = (totalCalories / totalWeight) * 100
		dish.ProteinPer100g = (totalProtein / totalWeight) * 100
		dish.CarbsPer100g = (totalCarbs / totalWeight) * 100
		dish.FatPer100g = (totalFat / totalWeight) * 100
		dish.ServingQuantity = totalWeight
		dish.ServingQuantityUnit = "g"
	}

	return dish, nil
}

func AddProfile(profile Profile) error {

	db := OpenDataBase()
	defer CloseDataBase(db)

	// Format created_at in ISO 8601 format
	createdAt := FormatDateTimeISO8601(profile.CreatedAt)

	query := `
    INSERT INTO profiles (id, name, created_at)
    VALUES (?, ?, ?)
    `

	_, err := db.Exec(query,
		profile.ID,
		profile.Name,
		createdAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert profile: %v", err)
	}

	if err := markDatabaseAsUnsynced(); err != nil {
		log.Printf("Failed to mark database as unsynced: %v", err)
	}
	messaging.BroadcastMessage("profiles_updated")
	return nil
}

func DeleteProfile(profileID string) error {

	db := OpenDataBase()
	defer CloseDataBase(db)

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	// Lösche abhängige Daten
	_, err = tx.Exec("DELETE FROM consumedFoodItems WHERE profile_id = ?", profileID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete consumed food items: %v", err)
	}

	_, err = tx.Exec("DELETE FROM userSettings WHERE profile_id = ?", profileID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete user settings: %v", err)
	}

	// Lösche das Profil selbst
	result, err := tx.Exec("DELETE FROM profiles WHERE id = ?", profileID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete profile: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error checking rows affected: %v", err)
	}

	if rowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("no profile found with id %s", profileID)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	if err := markDatabaseAsUnsynced(); err != nil {
		log.Printf("Failed to mark database as unsynced: %v", err)
	}
	messaging.BroadcastMessage("profiles_updated")
	return nil
}

func UpdateProfile(profileID string, name string) error {
	db := OpenDataBase()
	defer CloseDataBase(db)

	query := `
    UPDATE profiles 
    SET name = ?
    WHERE id = ?
    `

	result, err := db.Exec(query, name, profileID)
	if err != nil {
		return fmt.Errorf("failed to update profile: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no profile found with id %s", profileID)
	}

	if err := markDatabaseAsUnsynced(); err != nil {
		log.Printf("Failed to mark database as unsynced: %v", err)
	}
	messaging.BroadcastMessage("profiles_updated")
	return nil
}

func GetProfile(profileID string) (Profile, error) {
	db := OpenDataBase()
	defer CloseDataBase(db)

	var profile Profile
	query := `
    SELECT id, name, created_at
    FROM profiles
    WHERE id = ?
    `

	err := db.QueryRow(query, profileID).Scan(
		&profile.ID,
		&profile.Name,
		&profile.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return Profile{}, fmt.Errorf("no profile found with id %s", profileID)
	}
	if err != nil {
		return Profile{}, fmt.Errorf("failed to get profile: %v", err)
	}

	return profile, nil
}

func GetAllProfiles() ([]Profile, error) {
	db := OpenDataBase()
	defer CloseDataBase(db)

	query := `
    SELECT id, name, created_at
    FROM profiles
    ORDER BY created_at DESC
    `

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query profiles: %v", err)
	}
	defer rows.Close()

	var profiles []Profile

	for rows.Next() {
		var profile Profile
		err := rows.Scan(
			&profile.ID,
			&profile.Name,
			&profile.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan profile: %v", err)
		}
		profiles = append(profiles, profile)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating profiles: %v", err)
	}

	return profiles, nil
}

func markDatabaseAsUnsynced() error {
	store, err := settings.GetStore()
	if err != nil {
		return fmt.Errorf("failed to get settings store: %v", err)
	}
	store.MarkAsUnsynced()
	return nil
}

// InsertWeightTracking adds a new entry to the weight_tracking table
func InsertWeightTracking(profileID string, weight float64) error {
	db := OpenDataBase()
	defer CloseDataBase(db)

	// Generate a new UUID for the tracking entry
	id := uuid.New().String()

	query := `
	INSERT INTO weight_tracking (id, profile_id, weight)
	VALUES (?, ?, ?)
	`

	_, err := db.Exec(query, id, profileID, weight)
	if err != nil {
		return fmt.Errorf("failed to insert weight tracking record: %v", err)
	}

	return nil
}

// DeleteConsumedFoodItemsOlderThan deletes all consumed food items older than the specified date from all profiles
func DeleteConsumedFoodItemsOlderThan(date string) (int64, error) {
	db := OpenDataBase()
	defer CloseDataBase(db)

	query := "DELETE FROM consumedFoodItems WHERE date < ?"

	result, err := db.Exec(query, date)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old consumed food items: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error checking rows affected: %v", err)
	}

	if rowsAffected > 0 {
		if err := markDatabaseAsUnsynced(); err != nil {
			log.Printf("Failed to mark database as unsynced: %v", err)
		}
		messaging.BroadcastMessage("consumed_food_items_updated")
	}

	return rowsAffected, nil
}
