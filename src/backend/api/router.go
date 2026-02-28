// Package api provides the REST API for the Nutrack application
//
// @title Nutrack API
// @version 1.0
// @description Nutrition tracking application API
// @host localhost:8080
// @BasePath /api
// @schemes http
package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"nutrack/backend/data"
	"nutrack/backend/messaging"
	"nutrack/backend/service"
	"nutrack/backend/types"

	_ "nutrack/backend/api/docs"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/go-sqlite"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

var (
	allowedOrigins = []string{"http://nutrack-backend:8080", "http://localhost:82", "http://nutrack-frontend"}
	// Default API host for Swagger
	apiHost = "localhost:8080"
)

func init() {
	// Initialize API host from environment variable
	if envHost := os.Getenv("HOST_URL"); envHost != "" {
		// Add port 8080 if not specified
		if !strings.Contains(envHost, ":") {
			apiHost = envHost + ":8080"
		} else {
			apiHost = envHost
		}
	}
	fmt.Println("API Host for Swagger:", apiHost)

	// Initialize allowed origins
	additionalIPs := os.Getenv("ALLOWED_IPS")
	if additionalIPs != "" {
		ips := strings.Split(additionalIPs, ",")
		for _, ip := range ips {
			ip = strings.TrimSpace(ip)
			allowedOrigins = append(allowedOrigins,
				fmt.Sprintf("http://%s", ip))
		}
	} else {
		allowedOrigins = append(allowedOrigins, "http://localhost:3000")
	}

	fmt.Println("Allowed Origins:", allowedOrigins)
}

type (
	Router struct {
		engine      *gin.Engine
		foodService *service.FoodService
	}
)

func NewRouter() *Router {
	foodService, err := service.NewFoodService()
	if err != nil {
		panic(fmt.Sprintf("Failed to create food service: %v", err))
	}

	// Start the auto-sync monitor to automatically download database changes from Dropbox
	foodService.StartAutoSyncMonitor()

	router := &Router{
		engine:      gin.Default(),
		foodService: foodService,
	}
	return router
}

func (r *Router) SetupAndRunApiServer() {
	// Initialize swagger handler with the correct host
	swaggerHandler := ginSwagger.WrapHandler(swaggerFiles.Handler)
	// Use our custom swagger handler that handles host replacement
	r.engine.GET("/swagger/*any", CustomSwaggerHandler(swaggerHandler))
	config := cors.DefaultConfig()
	config.AllowOrigins = allowedOrigins
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	config.AllowCredentials = true
	r.engine.Use(cors.New(config))

	r.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	api := r.engine.Group("/api")
	{
		api.POST("/foodItems/check-and-insert", r.checkAndInsertFoodItem)
		api.POST("/foodItems/check-insert-and-consume", r.checkInsertAndConsume)
		api.POST("/foodItems/check-insert-and-consume-batch", r.checkInsertAndConsumeBatch)
		api.POST("/foodItems/manually-add", r.manuallyAddFoodItem)
		api.GET("/foodItems/all", r.getAllFoodItems)
		api.PUT("/foodItems/:barcode", r.updateFoodItem)
		api.DELETE("/foodItems/:barcode", r.deleteFoodItem)
		api.POST("/foodItems/reset/:barcode", r.resetFoodItem)
		api.GET("/foodItems/servingQuantity/:barcode", r.getServingQuantityByBarcode)
		api.GET("/foodItems/search", r.searchFoodItems)
		api.POST("/consumedFoodItems", r.postConsumedFoodItem)
		api.DELETE("/consumedFoodItems/:id", r.deleteConsumedFoodItem)
		api.GET("/consumedFoodItems/:date", r.getConsumedFoodItemsByDate)
		api.PUT("/consumedFoodItems/:id", r.updateConsumedFoodItem)

		api.POST("/settings", r.saveUserSettings)
		api.GET("/settings", r.getUserSettings)

		api.POST("/dishes", r.createDish)
		api.DELETE("/dishes/:id", r.deleteDish)
		api.PUT("/dishes/:id", r.updateDish)
		api.GET("/dishes/:id", r.getDish)
		api.GET("/dishes", r.getAllDishes)
		api.POST("/dishes/convert-to-food-item/:id", r.convertDishToFoodItem)

		api.GET("/profiles", r.getAllProfiles)
		api.POST("/profiles", r.createProfile)
		api.POST("/profiles/active", r.setActiveProfile)
		api.GET("/profiles/active", r.getActiveProfile)
		api.GET("/profiles/single/:id", r.getProfile)
		api.PUT("/profiles/single/:id", r.updateProfile)
		api.DELETE("/profiles/single/:id", r.deleteProfile)

		api.GET("/search", r.searchOpenFoodFacts)
		api.GET("/sse", setupSSE)

		api.POST("/dropbox/token", r.handleDropboxToken)
		api.GET("/dropbox/status", r.handleDropboxStatus)
		api.POST("/dropbox/logout", r.handleDropboxLogout)
		api.POST("/dropbox/upload-database", r.handleDropboxUploadDatabase)
		api.GET("/dropbox/download-database", r.handleDropboxDownloadDatabase)
		api.GET("/dropbox/autosync", r.handleGetDropboxAutosync)
		api.POST("/dropbox/autosync", r.handleSetDropboxAutosync)

		// Nutrition calculation endpoints
		api.POST("/nutrition/calculate", r.calculateNutrition)
		api.POST("/dropbox/sync", r.handleDropboxSync)
		api.GET("/settings/weighttracking", r.handleGetWeightTracking)
		api.POST("/settings/weighttracking", r.handleSetWeightTracking)
		api.GET("/settings/auto-recalculate-nutrition-values", r.handleGetAutoRecalculateNutritionValues)
		api.POST("/settings/auto-recalculate-nutrition-values", r.handleSetAutoRecalculateNutritionValues)
		api.POST("/settings/calculate-nutrients", r.calculateNutrition)
		api.POST("/settings/calculate-from-calories-and-weight", r.calculateNutritionFromCaloriesAndWeight)

		// Scanner endpoints
		api.GET("/scanners", r.listScanners)
		api.POST("/scanners/active", r.setActiveScanner)

	}

	println("Running API server on port 8080")
	r.engine.Run(":8080")
}

func setupSSE(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Headers", "Content-Type")
	origin := c.GetHeader("Origin")
	allowedOrigin := ""
	for _, allowed := range allowedOrigins {
		if allowed == origin {
			allowedOrigin = allowed
			break
		}
	}

	println("SSE Origin:", allowedOrigin)

	if allowedOrigin != "" {
		c.Header("Access-Control-Allow-Origin", allowedOrigin)
	} else {
		c.Header("Access-Control-Allow-Origin", "null")
	}

	c.Header("Access-Control-Allow-Credentials", "true")
	c.Header("Access-Control-Allow-Methods", "GET")

	// Create a buffered channel to handle multiple messages
	clientChan := make(chan string, 10)
	messaging.AddSSEClient(clientChan)
	println("SSE Client added")

	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-clientChan:
			if !ok {
				println("Channel closed")
				return false
			}

			// Process this message
			c.SSEvent("message", msg)

			// Check for any additional pending messages without blocking
			// This ensures we process message bursts efficiently
			processingMessages := true
			for processingMessages {
				select {
				case additionalMsg, ok := <-clientChan:
					if !ok {
						println("Channel closed during batch processing")
						return false
					}
					c.SSEvent("message", additionalMsg)
				default:
					// No more messages waiting, continue normal flow
					processingMessages = false
				}
			}

			return true
		case <-c.Request.Context().Done():
			println("Client disconnected")
			messaging.RemoveSSEClient(clientChan)
			return false
		}
	})
}

// @Summary Get all food items
// @Description Get a list of all food items
// @Tags foodItems
// @Produce  json
// @Success 200 {array} types.PersistentFoodItem
// @Router /foodItems/all [get]
func (r *Router) getAllFoodItems(c *gin.Context) {
	foodItems, err := r.foodService.GetAllFoodItems()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve food items"})
		return
	}

	c.JSON(http.StatusOK, foodItems)
}

// @Summary Get serving quantity by barcode
// @Description Get the serving quantity of a food item by barcode
// @Tags foodItems
// @Produce json
// @Param barcode path string true "Food item barcode"
// @Success 200 {object} gin.H
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /foodItems/servingQuantity/{barcode} [get]
func (r *Router) getServingQuantityByBarcode(c *gin.Context) {
	barcode := c.Param("barcode")

	servingQuantity, err := r.foodService.GetServingQuantityByBarcode(barcode)
	if err != nil {
		if err.Error() == fmt.Sprintf("no food item found with barcode %s", barcode) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve serving quantity"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"servingQuantity": servingQuantity,
	})
}

// @Summary Update a food item
// @Description Update a food item by barcode
// @Tags foodItems
// @Accept json
// @Produce json
// @Param barcode path string true "Food item barcode"
// @Param foodItem body types.PersistentFoodItem true "Updated food item data"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /foodItems/{barcode} [put]
func (r *Router) updateFoodItem(c *gin.Context) {
	barcode := c.Param("barcode")
	var updateData map[string]interface{}
	if err := c.BindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	err := r.foodService.UpdateFoodItem(barcode, updateData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update food item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Food item updated successfully"})
}

// @Summary Delete a food item
// @Description Delete a food item by barcode
// @Tags foodItems
// @Produce json
// @Param barcode path string true "Food item barcode"
// @Success 200 {object} gin.H
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /foodItems/{barcode} [delete]
func (r *Router) deleteFoodItem(c *gin.Context) {
	barcode := c.Param("barcode")

	err := r.foodService.DeleteFoodItem(barcode)
	if err != nil {
		if err.Error() == fmt.Sprintf("no food item found with barcode %s", barcode) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete food item"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Food item deleted successfully"})
}

// @Summary Reset a food item to default values
// @Description Reset a food item to its default values by barcode
// @Tags foodItems
// @Produce json
// @Param barcode path string true "Food item barcode"
// @Success 200 {object} gin.H
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /foodItems/reset/{barcode} [post]
func (r *Router) resetFoodItem(c *gin.Context) {
	barcode := c.Param("barcode")

	err := r.foodService.ResetFoodItem(barcode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Food item reset successfully"})
}

// @Summary Check if food item exists and insert if not
// @Description Check if a food item with the given barcode exists and insert it if not
// @Tags foodItems
// @Accept json
// @Produce json
// @Param foodItem body types.PersistentFoodItem true "Food item to check and insert"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /foodItems/check-and-insert [post]
func (r *Router) checkAndInsertFoodItem(c *gin.Context) {
	var request types.BarcodeRequest
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	err := r.foodService.CheckAndInsertFoodItem(request.Barcode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check and insert food item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Food item inserted successfully", "exists": false})
}

// @Summary Manually add a food item
// @Description Manually add a new food item
// @Tags foodItems
// @Accept json
// @Produce json
// @Param foodItem body types.PersistentFoodItem true "Food item to add"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /foodItems/manually-add [post]
func (r *Router) manuallyAddFoodItem(c *gin.Context) {
	var newItem data.PersistentFoodItem

	if err := c.BindJSON(&newItem); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	err := r.foodService.ManuallyAddFoodItem(newItem)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Food item added successfully"})
}

// @Summary Add consumed food item
// @Description Add a new consumed food item. If the consumed food item exists, the quantity will be added to the existing consumed food item. If no ProfileID is provided, the active profile will be used.
// @Tags consumedFoodItems
// @Accept json
// @Produce json
// @Param consumedItem body types.ConsumedFoodItemRequest true "Consumed food item to add"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /consumedFoodItems [post]
func (r *Router) postConsumedFoodItem(c *gin.Context) {
	var request types.ConsumedFoodItemRequest
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	err := r.foodService.PostConsumedFoodItem(request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to post consumed food item: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Food item consumed successfully"})
}

// @Summary Delete consumed food item
// @Description Delete a consumed food item by ID
// @Tags consumedFoodItems
// @Produce json
// @Param id path string true "Consumed food item ID"
// @Success 200 {object} gin.H
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /consumedFoodItems/{id} [delete]
func (r *Router) deleteConsumedFoodItem(c *gin.Context) {
	id := c.Param("id")

	err := r.foodService.DeleteConsumedFoodItem(id)
	if err != nil {
		if err.Error() == fmt.Sprintf("no consumed food item found with id %s", id) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete consumed food item"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Consumed food item deleted successfully"})
}

// @Summary Get consumed food items by date
// @Description Get all consumed food items for a specific date and profile. If no profile ID is provided, the active profile is used.
// @Tags consumedFoodItems
// @Produce json
// @Param date path string true "Date in YYYY-MM-DD format"
// @Param profile_id query string false "Profile ID"
// @Success 200 {array} []types.ConsumedFoodItemWithDetails
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /consumedFoodItems/{date} [get]
func (r *Router) getConsumedFoodItemsByDate(c *gin.Context) {
	dateStr := c.Param("date")
	profileID := c.Query("profile_id")

	consumedItems, err := r.foodService.GetConsumedFoodItemsByDate(dateStr, profileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to retrieve consumed food items: %v", err)})
		return
	}

	if len(consumedItems) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "No consumed food items found for the given date", "items": []data.ConsumedFoodItemWithDetails{}})
		return
	}

	c.JSON(http.StatusOK, consumedItems)
}

// @Summary Update consumed food item
// @Description Update a consumed food item by ID
// @Tags consumedFoodItems
// @Accept json
// @Produce json
// @Param id path string true "Consumed food item ID"
// @Param consumedItem body types.ConsumedFoodItemRequest true "Updated consumed food item data"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /consumedFoodItems/{id} [put]
func (r *Router) updateConsumedFoodItem(c *gin.Context) {
	id := c.Param("id")
	var updateData map[string]interface{}
	if err := c.BindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	err := r.foodService.UpdateConsumedFoodItem(id, updateData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update consumed food item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Consumed food item updated successfully"})
}

// @Summary Search food items
// @Description Search for food items by name
// @Tags foodItems
// @Produce json
// @Param query query string true "Search query"
// @Success 200 {array} []types.PersistentFoodItem
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /foodItems/search [get]
func (r *Router) searchFoodItems(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query is required"})
		return
	}

	items, err := r.foodService.SearchFoodItems(query)
	if err != nil {
		if strings.Contains(err.Error(), "must be at least") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search food items"})
		return
	}

	c.JSON(http.StatusOK, items)
}

// @Summary Search food items on OpenFoodFacts
// @Description Search for food items on OpenFoodFacts by name
// @Tags foodItems
// @Produce json
// @Param query query string true "Search query"
// @Success 200 {array} []types.PersistentFoodItem
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /foodItems/searchOpenFoodFacts [get]
func (r *Router) searchOpenFoodFacts(c *gin.Context) {
	searchTerm := c.Query("q")
	if searchTerm == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query is required"})
		return
	}

	items, err := r.foodService.SearchOpenFoodFacts(searchTerm)
	if err != nil {
		if strings.Contains(err.Error(), "must be at least") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if strings.Contains(err.Error(), "failed to fetch") {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "OpenFoodFacts service is currently unavailable"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search OpenFoodFacts"})
		return
	}

	c.JSON(http.StatusOK, items)
}

// @Summary Save user settings
// @Description Save user settings. If no profile ID is provided, the active profile is used.
// @Tags settings
// @Accept json
// @Produce json
// @Param settings body types.UserSettings true "User settings"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /settings [post]
func (r *Router) saveUserSettings(c *gin.Context) {
	var request struct {
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
		ProfileID          string  `json:"profile_id"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// convert the request to UserSettings
	settings := data.UserSettings{
		Weight:             request.Weight,
		Height:             request.Height,
		Calories:           request.Calories,
		Proteins:           request.Proteins,
		Carbs:              request.Carbs,
		Fat:                request.Fat,
		BirthDate:          request.BirthDate,
		Gender:             request.Gender,
		ActivityLevel:      request.ActivityLevel,
		WeeklyWeightChange: request.WeeklyWeightChange,
	}

	err := r.foodService.SaveUserSettings(settings, request.ProfileID)
	if err != nil {
		if strings.Contains(err.Error(), "no profile found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save user settings"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User settings saved successfully"})
}

// @Summary Get user settings
// @Description Get user settings by profile ID. If no profile ID is provided, the active profile is used.
// @Tags settings
// @Produce json
// @Param profile_id query string true "Profile ID"
// @Success 200 {object} types.UserSettings
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /settings [get]
func (r *Router) getUserSettings(c *gin.Context) {
	profileID := c.Query("profile_id")

	settings, err := r.foodService.GetUserSettings(profileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get user settings: %v", err)})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// @Summary Check insert and consume food item
// @Description Check if a food item exists, insert it if it doesn't, and consume it. If no profile ID is provided, the active profile is used.
// @Tags consumedFoodItems
// @Accept json
// @Produce json
// @Param consumedFoodItem body types.ConsumedFoodItemRequest true "Consumed food item"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /consumedFoodItems/checkInsertAndConsume [post]
func (r *Router) checkInsertAndConsume(c *gin.Context) {
	var request types.ConsumedFoodItemRequest
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	err := r.foodService.CheckInsertAndConsume(request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check/insert and consume food item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Food item checked/inserted and consumed successfully"})
}

func (r *Router) checkInsertAndConsumeBatch(c *gin.Context) {
	var request types.BatchConsumedFoodItemRequest
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	err := r.foodService.CheckInsertAndConsumeBatch(request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to batch check/insert and consume food items"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Food items batch checked/inserted and consumed successfully"})
}

// @Summary Create a new dish
// @Description Create a new dish with ingredients
// @Tags dishes
// @Accept json
// @Produce json
// @Param dish body types.DishRequest true "Dish to create"
// @Success 200 {object} types.Dish
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /dishes [post]
func (r *Router) createDish(c *gin.Context) {
	var request types.DishRequest
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	err := r.foodService.CreateDish(request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create dish"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Dish created successfully"})
}

// @Summary Delete a dish
// @Description Delete a dish by ID
// @Tags dishes
// @Produce json
// @Param id path string true "Dish ID"
// @Success 200 {object} gin.H
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /dishes/{id} [delete]
func (r *Router) deleteDish(c *gin.Context) {
	dishID := c.Param("id")

	err := r.foodService.DeleteDish(dishID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete dish"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Dish deleted successfully"})
}

// @Summary Update a dish
// @Description Update a dish by ID
// @Tags dishes
// @Accept json
// @Produce json
// @Param id path string true "Dish ID"
// @Param dish body types.DishRequest true "Updated dish data"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /dishes/{id} [put]
func (r *Router) updateDish(c *gin.Context) {
	dishID := c.Param("id")
	var request types.DishRequest
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	err := r.foodService.UpdateDish(dishID, request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update dish"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Dish updated successfully"})
}

// @Summary Get a dish
// @Description Get a dish by ID
// @Tags dishes
// @Produce json
// @Param id path string true "Dish ID"
// @Success 200 {object} types.DishWithDetailedItems
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /dishes/{id} [get]
func (r *Router) getDish(c *gin.Context) {
	id := c.Param("id")

	dish, items, err := r.foodService.GetDish(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve dish"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"dish": dish, "items": items})
}

// @Summary Get all dishes
// @Description Get a list of all dishes
// @Tags dishes
// @Produce json
// @Success 200 {array} []types.Dish
// @Failure 500 {object} gin.H
// @Router /dishes [get]
func (r *Router) getAllDishes(c *gin.Context) {
	dishes, err := r.foodService.GetAllDishes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve dishes"})
		return
	}

	if len(dishes) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "No dishes found", "dishes": []data.DishWithDetailedItems{}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"dishes": dishes})
}

// @Summary Convert dish to food item
// @Description Convert a dish to a food item
// @Tags dishes
// @Produce json
// @Param id path string true "Dish ID"
// @Success 200 {object} gin.H
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /dishes/{id}/convertToFoodItem [post]
func (r *Router) convertDishToFoodItem(c *gin.Context) {
	dishID := c.Param("id")

	err := r.foodService.ConvertDishToFoodItem(dishID)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to convert dish to food item"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Dish successfully converted to food item"})
}

// @Summary Get all profiles
// @Description Get a list of all profiles
// @Tags profiles
// @Produce json
// @Success 200 {array} []types.Profile
// @Failure 500 {object} gin.H
// @Router /profiles [get]
func (r *Router) getAllProfiles(c *gin.Context) {
	profiles, err := r.foodService.GetAllProfiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve profiles"})
		return
	}

	if len(profiles) == 0 {
		c.JSON(http.StatusOK, gin.H{"profiles": []data.Profile{}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"profiles": profiles})
}

// @Summary Get a profile
// @Description Get a profile by ID. If no profile ID is provided, the active profile is used.
// @Tags profiles
// @Produce json
// @Param id path string true "Profile ID"
// @Success 200 {object} types.Profile
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /profiles/single/{id} [get]
func (r *Router) getProfile(c *gin.Context) {
	profileID := c.Param("id")

	profile, err := r.foodService.GetProfile(profileID)
	if err != nil {
		if strings.Contains(err.Error(), "no profile found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve profile"})
		}
		return
	}

	c.JSON(http.StatusOK, profile)
}

// @Summary Create a profile
// @Description Create a new profile
// @Tags profiles
// @Accept json
// @Produce json
// @Param profile body types.Profile true "Profile to create"
// @Success 201 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /profiles [post]
func (r *Router) createProfile(c *gin.Context) {
	var profile data.Profile
	if err := c.BindJSON(&profile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	profileID, err := r.foodService.CreateProfile(profile.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create profile"})
		return
	}

	// Create a basic profile response without calling GetProfile
	createdProfile := data.Profile{
		ID:   profileID,
		Name: profile.Name,
	}

	c.JSON(http.StatusCreated, createdProfile)
}

// @Summary Update a profile
// @Description Update a profile by ID. If no profile ID is provided, the active profile is used.
// @Tags profiles
// @Accept json
// @Produce json
// @Param id path string true "Profile ID"
// @Param profile body types.Profile true "Updated profile data"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /profiles/single/{id} [put]
func (r *Router) updateProfile(c *gin.Context) {
	profileID := c.Param("id")
	var updateData struct {
		Name string `json:"name"`
	}

	if err := c.BindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	err := r.foodService.UpdateProfile(profileID, updateData.Name)
	if err != nil {
		if strings.Contains(err.Error(), "no profile found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

// @Summary Delete a profile
// @Description Delete a profile by ID
// @Tags profiles
// @Produce json
// @Param id path string true "Profile ID"
// @Success 200 {object} gin.H
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /profiles/single/{id} [delete]
func (r *Router) deleteProfile(c *gin.Context) {
	profileID := c.Param("id")

	err := r.foodService.DeleteProfile(profileID)
	if err != nil {
		if strings.Contains(err.Error(), "no profile found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete profile"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile deleted successfully"})
}

// @Summary Set active profile
// @Description Set the active profile
// @Tags profiles
// @Accept json
// @Produce json
// @Param profile body types.ActiveProfileRequest true "Profile to set as active"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /profiles/active [post]
func (r *Router) setActiveProfile(c *gin.Context) {

	var request types.ActiveProfileRequest
	// Log request body
	body, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	fmt.Printf("Request body for setActiveProfile: %s\n", string(body))

	if err := c.BindJSON(&request); err != nil {
		fmt.Printf("Error binding JSON: %v\n", err)
		fmt.Printf("Request body: %s\n", string(body))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	fmt.Printf("Setting active profile to: %s\n", request.ProfileID)

	err := r.foodService.SetActiveProfile(request.ProfileID)
	if err != nil {
		fmt.Printf("Error setting active profile: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set active profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Active profile set successfully"})
}

// @Summary Get active profile
// @Description Get the currently active profile
// @Tags profiles
// @Produce json
// @Success 200 {object} types.Profile
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /profiles/active [get]
func (r *Router) getActiveProfile(c *gin.Context) {
	profile, err := r.foodService.GetProfile("")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get active profile details"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"profile_id": profile.ID,
		"name":       profile.Name,
		"active":     true,
	})
}

// @Summary Exchange Dropbox token
// @Description Exchange a Dropbox authorization code for an access token
// @Tags authentication
// @Accept json
// @Produce json
// @Param token body object true "Dropbox authorization code and code verifier"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /dropbox/token [post]
func (r *Router) handleDropboxToken(c *gin.Context) {
	var request types.TokenRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := r.foodService.ExchangeToken(request.Code, request.CodeVerifier)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Token exchange successful"})
}

// @Summary Get Dropbox authentication status
// @Description Get the authentication status of the Dropbox account
// @Tags authentication
// @Produce json
// @Success 200 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /dropbox/status [get]
func (r *Router) handleDropboxStatus(c *gin.Context) {
	isAuthenticated, err := r.foodService.GetAuthenticationStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"isAuthenticated": isAuthenticated})
}

// @Summary Logout from Dropbox
// @Description Logout from Dropbox
// @Tags authentication
// @Produce json
// @Success 200 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /dropbox/logout [post]
func (r *Router) handleDropboxLogout(c *gin.Context) {
	err := r.foodService.Logout()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// @Summary Upload database to Dropbox
// @Description Upload the database to Dropbox
// @Tags database
// @Produce json
// @Success 200 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /dropbox/uploadDatabase [post]
func (r *Router) handleDropboxUploadDatabase(c *gin.Context) {
	result, err := r.foodService.UploadDatabase()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": result.Success,
		"status":  result.Status,
	})
}

// @Summary Download database from Dropbox
// @Description Download the database from Dropbox
// @Tags database
// @Produce json
// @Success 200 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /dropbox/downloadDatabase [post]
func (r *Router) handleDropboxDownloadDatabase(c *gin.Context) {
	result, err := r.foodService.DownloadDatabase()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": result.Success,
		"status":  result.Status,
	})
}

// @Summary Get Dropbox autosync status
// @Description Get the autosync status of the Dropbox account
// @Tags database
// @Produce json
// @Success 200 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /dropbox/autosync [get]
func (r *Router) handleGetDropboxAutosync(c *gin.Context) {
	enabled := r.foodService.GetAutoSync()
	c.JSON(http.StatusOK, gin.H{"enabled": enabled})
}

// @Summary Set Dropbox autosync status
// @Description Set the autosync status of the Dropbox account
// @Tags database
// @Accept json
// @Produce json
// @Param autosync body types.DropboxAutosyncRequest true "Autosync status"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /dropbox/autosync [post]
func (r *Router) handleSetDropboxAutosync(c *gin.Context) {
	var request types.DropboxAutosyncRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := r.foodService.SetAutoSync(request.Enabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Auto-sync setting updated"})
}

// @Summary Get weight tracking status
// @Description Get the weight tracking status
// @Tags weightTracking
// @Produce json
// @Success 200 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /weightTracking [get]
func (r *Router) handleGetWeightTracking(c *gin.Context) {
	enabled := r.foodService.GetWeightTracking()
	c.JSON(http.StatusOK, gin.H{"enabled": enabled})
}

// @Summary Set weight tracking status
// @Description Set the weight tracking status
// @Tags weightTracking
// @Accept json
// @Produce json
// @Param weightTracking body types.WeightTrackingRequest true "Weight tracking status"
// @Success 200 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /weightTracking [post]
func (r *Router) handleSetWeightTracking(c *gin.Context) {
	var request types.WeightTrackingRequest
	fmt.Printf("Received request: %s\n", c.Request.Body)
	body, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	fmt.Printf("Request body: %s\n", string(body))
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := r.foodService.SetWeightTracking(request.Enabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Weight tracking setting updated"})
}

// @Summary Get auto recalculate nutrition values status
// @Description Get the auto recalculate nutrition values status
// @Tags autoRecalculateNutritionValues
// @Produce json
// @Success 200 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /autoRecalculateNutritionValues [get]
func (r *Router) handleGetAutoRecalculateNutritionValues(c *gin.Context) {
	enabled := r.foodService.GetAutoRecalculateNutritionValues()
	c.JSON(http.StatusOK, gin.H{"enabled": enabled})
}

// @Summary Set auto recalculate nutrition values status
// @Description Set the auto recalculate nutrition values status
// @Tags autoRecalculateNutritionValues
// @Accept json
// @Produce json
// @Param autoRecalculateNutritionValues body types.AutoRecalculateNutritionValuesRequest true "Auto recalculate nutrition values status"
// @Success 200 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /autoRecalculateNutritionValues [post]
func (r *Router) handleSetAutoRecalculateNutritionValues(c *gin.Context) {
	var request types.AutoRecalculateNutritionValuesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := r.foodService.SetAutoRecalculateNutritionValues(request.Enabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Auto recalculate nutrition values setting updated"})
}

// CalculateNutrition handles the nutrition calculation request
// @Summary Calculate nutrition targets
// @Description Calculate daily nutrition targets based on user metrics
// @Tags nutrition
// @Accept json
// @Produce json
// @Param request body types.NutritionCalculationRequest true "User metrics for calculation"
// @Success 200 {object} types.NutritionCalculationResponse
// @Failure 400 {object} types.ApiResponse
// @Router /nutrition/calculate [post]
func (r *Router) calculateNutrition(c *gin.Context) {
	var req types.NutritionCalculationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ApiResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// Validate activity level
	if req.ActivityLevel < 0 || req.ActivityLevel > 4 {
		c.JSON(http.StatusBadRequest, types.ApiResponse{
			Success: false,
			Error:   "Activity level must be between 0 and 4",
		})
		return
	}

	// Validate gender
	if req.Gender != "male" && req.Gender != "female" && req.Gender != "other" {
		c.JSON(http.StatusBadRequest, types.ApiResponse{
			Success: false,
			Error:   "Gender must be 'male', 'female', or 'other'",
		})
		return
	}

	// Calculate nutrition
	result := r.foodService.CalculateNutrition(
		req.Weight,
		req.Height,
		req.Age,
		req.Gender,
		req.ActivityLevel,
		req.WeeklyWeightChange,
	)

	c.JSON(http.StatusOK, result)
}

// CalculateNutritionFromCaloriesAndWeight handles the nutrition calculation request
// @Summary Calculate nutrition targets
// @Description Calculate daily nutrition targets based on user metrics
// @Tags nutrition
// @Accept json
// @Produce json
// @Param request body types.NutritionCalculationRequest true "User metrics for calculation"
// @Success 200 {object} types.NutritionCalculationResponse
// @Failure 400 {object} types.ApiResponse
// @Router /nutrition/calculate-from-calories-and-weight [post]
func (r *Router) calculateNutritionFromCaloriesAndWeight(c *gin.Context) {
	var req types.NutritionCalculationFromCaloriesAndWeightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ApiResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// Calculate nutrition
	result := r.foodService.CalculateNutritionFromCaloriesAndWeight(
		req.Calories,
		req.Weight,
	)

	c.JSON(http.StatusOK, result)
}

// @Summary Sync with Dropbox
// @Description Sync the database with Dropbox
// @Tags database
// @Accept json
// @Produce json
// @Param force body types.ForceSyncRequest true "Force sync"
// @Success 200 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /dropbox/sync [post]
func (r *Router) handleDropboxSync(c *gin.Context) {
	var request types.ForceSyncRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		request.Force = false
	}

	err := r.foodService.SyncToDropbox(request.Force)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

// @Summary List scanners
// @Description List all available scanners
// @Tags scanners
// @Produce json
// @Success 200 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /scanners [get]
func (r *Router) listScanners(c *gin.Context) {
	scanners, err := r.foodService.ListScanners()
	if err != nil {
		fmt.Printf("Error listing scanners: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	fmt.Printf("Found scanners: %+v\n", scanners)
	c.JSON(http.StatusOK, scanners)
}

// @Summary Set active scanner
// @Description Set the active scanner
// @Tags scanners
// @Accept json
// @Produce json
// @Param scanner body types.ActiveScannerRequest true "Scanner to set as active"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /scanners/active [post]
func (r *Router) setActiveScanner(c *gin.Context) {
	var request types.ActiveScannerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var pathValue string
	if request.Path == nil {
		pathValue = ""
	} else {
		pathValue = *request.Path
	}

	fmt.Printf("Setting active scanner with path: %s\n", pathValue)
	if err := r.foodService.SetActiveScanner(pathValue); err != nil {
		fmt.Printf("Error setting active scanner: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	fmt.Printf("Active scanner set successfully\n")
	c.JSON(http.StatusOK, gin.H{"message": "Active scanner set"})
}
