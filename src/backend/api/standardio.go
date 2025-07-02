package api

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"nutrack/backend/data"
	"nutrack/backend/service"
	"nutrack/backend/types"
	"os"
	"strings"
)

type Request struct {
	Type      string      `json:"type"`
	Method    string      `json:"method"`
	Endpoint  string      `json:"endpoint"`
	Data      interface{} `json:"data"`
	RequestId string      `json:"requestId"`
}

type Response struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	RequestId string      `json:"requestId"`
}

type StandardIOHandler struct {
	foodService *service.FoodService
}

func NewStandardIOHandler() *StandardIOHandler {
	foodService, err := service.NewFoodService()
	if err != nil {
		panic(fmt.Sprintf("Failed to create food service: %v", err))
	}
	return &StandardIOHandler{
		foodService: foodService,
	}
}

func (h *StandardIOHandler) Start() {
	log.Println("StandardIO handler started - waiting for input")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()
		h.HandleStandardIOInput(input)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading standard input: %v\n", err)
	}
}

func (h *StandardIOHandler) HandleStandardIOInput(input string) {
	var request Request
	if err := json.Unmarshal([]byte(input), &request); err != nil {
		h.sendErrorResponse("Invalid JSON request format", "")
		return
	}

	log.Printf("Request received [%s]: %+v\n", request.RequestId, request)

	if request.Type != "request" {
		h.sendErrorResponse("Invalid request type", request.RequestId)
		return
	}

	requestDataMap, ok := request.Data.(map[string]interface{})
	if !ok {
		h.sendErrorResponse("Invalid request data format", request.RequestId)
		return
	}

	urlParams, ok := requestDataMap["urlParams"].([]interface{})
	if !ok {
		h.sendErrorResponse("Invalid urlParams format", request.RequestId)
		return
	}

	parts := strings.Split(request.Endpoint, "/")
	if len(parts) < 2 {
		h.sendErrorResponse("Invalid endpoint", request.RequestId)
		return
	}

	response, err := h.processRequest(parts[1], request.Method, request, requestDataMap, urlParams)
	if err != nil {
		h.sendErrorResponse(err.Error(), request.RequestId)
		return
	}

	h.sendResponse(response, request.RequestId)
}

func (h *StandardIOHandler) processRequest(endpoint string, method string, request Request, requestDataMap map[string]interface{}, urlParams []interface{}) (interface{}, error) {
	switch endpoint {
	case "/foodItems/check-and-insert":
		barcode := requestDataMap["barcode"].(string)

		err := h.foodService.CheckAndInsertFoodItem(barcode)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"message": "Food item inserted successfully", "exists": false}, nil
	case "/foodItems/check-insert-and-consume":
		barcode := requestDataMap["barcode"].(string)

		var consumedRequest types.ConsumedFoodItemRequest
		consumedRequest.Barcode = barcode

		err := h.foodService.CheckInsertAndConsume(consumedRequest)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"message": "Food item checked/inserted and consumed successfully"}, nil

	case "/foodItems/check-insert-and-consume-batch":
		// Extrahiere die Items aus der Anfrage
		items, ok := requestDataMap["items"].([]interface{})
		if !ok {
			return nil, errors.New("invalid items format")
		}

		// Erstelle die Batch-Anfrage
		var batchRequest types.BatchConsumedFoodItemRequest

		// Konvertiere jedes Item in der Liste
		for _, item := range items {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				return nil, errors.New("invalid item format in batch")
			}

			var consumedItem types.ConsumedFoodItemRequest

			// Barcode ist erforderlich
			barcode, ok := itemMap["barcode"].(string)
			if !ok || barcode == "" {
				return nil, errors.New("barcode is required for each item")
			}
			consumedItem.Barcode = barcode

			// Weitere Felder, falls vorhanden
			if quantity, ok := itemMap["consumed_quantity"].(float64); ok {
				consumedItem.ConsumedQuantity = quantity
			}

			if date, ok := itemMap["date"].(string); ok {
				consumedItem.Date = date
			}

			if profileID, ok := itemMap["profile_id"].(string); ok {
				consumedItem.ProfileID = profileID
			}

			batchRequest.Items = append(batchRequest.Items, consumedItem)
		}

		// ForceSync-Parameter, falls vorhanden
		if forceSync, ok := requestDataMap["force_sync"].(bool); ok {
			batchRequest.ForceSync = forceSync
		}

		// Verarbeite die Batch-Anfrage
		err := h.foodService.CheckInsertAndConsumeBatch(batchRequest)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{"message": "Food items batch checked/inserted and consumed successfully"}, nil

	case "/foodItems/all":
		foodItems, err := h.foodService.GetAllFoodItems()
		if err != nil {
			return nil, err
		} else {
			return foodItems, nil
		}

	case "/foodItems/manually-add":
		requestData, err := json.Marshal(requestDataMap)
		if err != nil {
			return nil, err
		}

		var foodItemData data.PersistentFoodItem
		if err := json.Unmarshal(requestData, &foodItemData); err != nil {
			return nil, err
		}

		err = h.foodService.ManuallyAddFoodItem(foodItemData)
		if err != nil {
			return nil, err
		} else {
			return map[string]interface{}{"message": "Food item added successfully"}, nil
		}

	case "/foodItems":
		switch method {
		case "PUT":
			barcode, ok := urlParams[0].(string)
			if !ok {
				return nil, errors.New("invalid barcode")
			}
			updateData, ok := requestDataMap["data"].(map[string]interface{})
			if !ok {
				return nil, errors.New("invalid update data")
			}

			err := h.foodService.UpdateFoodItem(barcode, updateData)
			if err != nil {
				return nil, err
			} else {
				return map[string]interface{}{"message": "Food item updated successfully"}, nil
			}
		case "DELETE":
			barcode, ok := urlParams[0].(string)
			if !ok {
				return nil, errors.New("invalid barcode")
			}

			err := h.foodService.DeleteFoodItem(barcode)
			if err != nil {
				return nil, err
			} else {
				return map[string]interface{}{"message": "Food item deleted successfully"}, nil
			}
		default:
			return nil, fmt.Errorf("unknown method: %s", method)
		}

	case "/foodItems/reset":
		barcode, ok := urlParams[0].(string)
		if !ok {
			return nil, errors.New("invalid barcode")
		}

		err := h.foodService.ResetFoodItem(barcode)
		if err != nil {
			return nil, err
		} else {
			return map[string]interface{}{"message": "Food item reset successfully"}, nil
		}

	case "/foodItems/servingQuantity":
		barcode, ok := requestDataMap["barcode"].(string)
		if !ok {
			return nil, errors.New("invalid barcode")
		}

		servingQuantity, err := h.foodService.GetServingQuantityByBarcode(barcode)
		if err != nil {
			return nil, err
		} else {
			return map[string]interface{}{"servingQuantity": servingQuantity}, nil
		}

	case "/foodItems/search":
		query, ok := urlParams[0].(string)
		if !ok {
			return nil, errors.New("invalid query")
		}

		query = query[3:]
		log.Printf("Searching for food items: %s\n", query)
		results, err := h.foodService.SearchFoodItems(query)
		if err != nil {
			return nil, err
		} else {
			return results, nil
		}

	case "/consumedFoodItems":
		switch method {
		case "POST":
			requestData, err := json.Marshal(requestDataMap)
			if err != nil {
				return nil, err
			}

			var consumedRequest types.ConsumedFoodItemRequest
			if err := json.Unmarshal(requestData, &consumedRequest); err != nil {
				return nil, err
			}

			if consumedRequest.ProfileID == "" {
				return nil, errors.New("ProfileID is required")
			}

			err = h.foodService.PostConsumedFoodItem(consumedRequest)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"message": "Food item consumed successfully"}, nil
		case "DELETE":
			id, ok := urlParams[0].(string)
			if !ok {
				return nil, errors.New("invalid id")
			}
			err := h.foodService.DeleteConsumedFoodItem(id)
			if err != nil {
				return nil, err
			} else {
				return map[string]interface{}{"message": "Food item deleted successfully"}, nil
			}
		case "GET":
			dateStr := urlParams[0].(string)
			profileID, ok := requestDataMap["ProfileID"].(string)
			if !ok || profileID == "" {
				return nil, errors.New("ProfileID is required")
			}

			consumedItems, err := h.foodService.GetConsumedFoodItemsByDate(dateStr, profileID)
			if err != nil {
				return nil, err
			}
			return consumedItems, nil
		case "PUT":
			var consumedRequest types.ConsumedFoodItemRequest

			id, ok := urlParams[0].(string)
			if !ok {
				return nil, errors.New("invalid id")
			}
			requestData, err := json.Marshal(request.Data)
			if err != nil {
				return nil, err
			}

			if err := json.Unmarshal(requestData, &consumedRequest); err != nil {
				return nil, err
			}

			err = h.foodService.UpdateConsumedFoodItem(id, requestDataMap)
			if err != nil {
				return nil, err
			} else {
				return map[string]interface{}{"message": "Food item updated successfully"}, nil
			}
		default:
			return nil, fmt.Errorf("unknown method: %s", method)
		}
	case "/settings":
		switch method {
		case "GET":
			profileID, ok := requestDataMap["ProfileID"].(string)
			if !ok || profileID == "" {
				return nil, errors.New("ProfileID is required")
			}

			settings, err := h.foodService.GetUserSettings(profileID)
			if err != nil {
				return nil, err
			}
			return settings, nil
		case "POST":
			requestData, err := json.Marshal(requestDataMap)
			if err != nil {
				return nil, err
			}

			var settingsRequest data.UserSettings
			if err := json.Unmarshal(requestData, &settingsRequest); err != nil {
				return nil, err
			}

			profileID, ok := requestDataMap["ProfileID"].(string)
			if !ok || profileID == "" {
				return nil, errors.New("ProfileID is required")
			}

			err = h.foodService.SaveUserSettings(settingsRequest, profileID)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"message": "Settings saved successfully"}, nil
		default:
			return nil, fmt.Errorf("unknown method: %s", method)
		}
	case "/dishes":
		switch method {
		case "POST":
			var dishRequest types.DishRequest
			requestData, err := json.Marshal(request.Data)
			if err != nil {
				return nil, err
			}

			if err := json.Unmarshal(requestData, &dishRequest); err != nil {
				return nil, err
			}

			err = h.foodService.CreateDish(dishRequest)
			if err != nil {
				return nil, err
			} else {
				return map[string]interface{}{"message": "Dish created successfully"}, nil
			}
		case "DELETE":
			dishID, ok := urlParams[0].(string)
			if !ok {
				return nil, errors.New("invalid dish ID")
			}

			err := h.foodService.DeleteDish(dishID)
			if err != nil {
				return nil, err
			} else {
				return map[string]interface{}{"message": "Dish deleted successfully"}, nil
			}
		case "PUT":
			dishID, ok := urlParams[0].(string)
			if !ok {
				return nil, errors.New("invalid dish ID")
			}
			var dishRequest types.DishRequest
			requestData, err := json.Marshal(request.Data)
			if err != nil {
				return nil, err
			}

			if err := json.Unmarshal(requestData, &dishRequest); err != nil {
				return nil, err
			}

			err = h.foodService.UpdateDish(dishID, dishRequest)
			if err != nil {
				return nil, err
			} else {
				return map[string]interface{}{"message": "Dish updated successfully"}, nil
			}
		case "GET":
			if len(urlParams) == 0 {
				dishes, err := h.foodService.GetAllDishes()
				if err != nil {
					return nil, err
				} else {
					if len(dishes) == 0 {
						return map[string]interface{}{
							"message": "No dishes found",
							"dishes":  []data.DishWithDetailedItems{},
						}, nil
					} else {
						return map[string]interface{}{"dishes": dishes}, nil
					}
				}
			} else {
				dishID, ok := urlParams[0].(string)
				if !ok {
					return nil, errors.New("invalid dish ID")
				}

				dish, items, err := h.foodService.GetDish(dishID)
				if err != nil {
					return nil, err
				} else {
					return map[string]interface{}{"dish": dish, "items": items}, nil
				}
			}
		default:
			return nil, fmt.Errorf("unknown method: %s", method)
		}

	case "/dishes/convert-to-food-item":
		dishID, ok := urlParams[0].(string)
		if !ok {
			return nil, errors.New("invalid dish ID")
		}

		err := h.foodService.ConvertDishToFoodItem(dishID)
		if err != nil {
			return nil, err
		} else {
			return map[string]interface{}{"message": "Dish successfully converted to food item"}, nil
		}

	case "/profiles":
		switch method {
		case "GET":
			if len(urlParams) == 0 {
				profiles, err := h.foodService.GetAllProfiles()
				if err != nil {
					return nil, err
				}
				return map[string]interface{}{"profiles": profiles}, nil
			} else {
				profileID, ok := urlParams[0].(string)
				if !ok {
					return nil, errors.New("invalid profile ID")
				}
				profile, err := h.foodService.GetProfile(profileID)
				if err != nil {
					return nil, err
				}
				return profile, nil
			}

		case "POST":
			name, ok := requestDataMap["name"].(string)
			if !ok || name == "" {
				return nil, errors.New("profile name is required")
			}

			profileID, err := h.foodService.CreateProfile(name)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"message": "Profile created successfully", "profileID": profileID}, nil

		case "PUT":
			profileID, ok := urlParams[0].(string)
			if !ok {
				return nil, errors.New("invalid profile ID")
			}

			name, ok := requestDataMap["name"].(string)
			if !ok || name == "" {
				return nil, errors.New("profile name is required")
			}

			err := h.foodService.UpdateProfile(profileID, name)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"message": "Profile updated successfully"}, nil

		case "DELETE":
			profileID, ok := urlParams[0].(string)
			if !ok {
				return nil, errors.New("invalid profile ID")
			}

			err := h.foodService.DeleteProfile(profileID)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"message": "Profile deleted successfully"}, nil
		default:
			return nil, fmt.Errorf("unknown method: %s", method)
		}
	case "/profiles/active":
		switch method {
		case "GET":
			profileID := h.foodService.GetActiveProfile()

			if profileID == "" {
				return map[string]interface{}{
					"profile_id": "",
					"active":     false,
				}, nil
			}

			// Hole die Profilinformationen, wenn ein aktives Profil vorhanden ist
			profile, err := h.foodService.GetProfile(profileID)
			if err != nil {
				return nil, fmt.Errorf("failed to get active profile details: %v", err)
			}

			return map[string]interface{}{
				"profile_id": profileID,
				"name":       profile.Name,
				"active":     true,
			}, nil

		case "POST":
			profileID, ok := requestDataMap["ProfileID"].(string)
			if !ok || profileID == "" {
				return nil, errors.New("ProfileID is required")
			}

			err := h.foodService.SetActiveProfile(profileID)
			if err != nil {
				return nil, err
			}

			return map[string]interface{}{"message": "Active profile set successfully"}, nil

		default:
			return nil, fmt.Errorf("unknown method: %s", method)
		}
	case "/dropbox/status":
		if method != "GET" {
			return nil, fmt.Errorf("method %s not allowed for %s", method, endpoint)
		}
		isAuthenticated, err := h.foodService.GetAuthenticationStatus()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"isAuthenticated": isAuthenticated}, nil

	case "/dropbox/token":
		if method != "POST" {
			return nil, fmt.Errorf("method %s not allowed for %s", method, endpoint)
		}
		code, ok := requestDataMap["code"].(string)
		if !ok || code == "" {
			return nil, errors.New("code is required")
		}
		codeVerifier, ok := requestDataMap["codeVerifier"].(string)
		if !ok || codeVerifier == "" {
			return nil, errors.New("codeVerifier is required")
		}

		_, err := h.foodService.ExchangeToken(code, codeVerifier)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"message": "Token exchange successful"}, nil

	case "/dropbox/logout":
		if method != "POST" {
			return nil, fmt.Errorf("method %s not allowed for %s", method, endpoint)
		}
		err := h.foodService.Logout()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"message": "Logged out successfully"}, nil

	case "/dropbox/upload-database":
		if method != "POST" {
			return nil, fmt.Errorf("method %s not allowed for %s", method, endpoint)
		}
		result, err := h.foodService.UploadDatabase()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"success": result.Success,
			"status":  result.Status,
		}, nil

	case "/dropbox/download-database":
		if method != "GET" {
			return nil, fmt.Errorf("method %s not allowed for %s", method, endpoint)
		}
		result, err := h.foodService.DownloadDatabase()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"success": result.Success,
			"status":  result.Status,
		}, nil

	case "/dropbox/sync":
		if method != "POST" {
			return nil, fmt.Errorf("method %s not allowed for %s", method, endpoint)
		}
		force, ok := requestDataMap["force"].(bool)
		if !ok {
			force = false
		}

		err := h.foodService.SyncToDropbox(force)
		if err != nil {
			return nil, err
		}
		return nil, nil

	case "/dropbox/autosync":
		switch method {
		case "GET":
			return map[string]interface{}{"enabled": h.foodService.GetAutoSync()}, nil
		case "POST":
			enabled, ok := requestDataMap["enabled"].(bool)
			if !ok {
				return nil, errors.New("invalid request: enabled must be a boolean")
			}
			err := h.foodService.SetAutoSync(enabled)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"message": "Auto-sync setting updated"}, nil
		default:
			return nil, fmt.Errorf("method %s not allowed for endpoint %s", method, endpoint)
		}

	case "/settings/weighttracking":
		switch method {
		case "GET":
			return map[string]interface{}{"enabled": h.foodService.GetWeightTracking()}, nil
		case "POST":
			enabled, ok := requestDataMap["enabled"].(bool)
			if !ok {
				return nil, errors.New("invalid request: enabled must be a boolean")
			}
			err := h.foodService.SetWeightTracking(enabled)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"message": "Weight tracking setting updated"}, nil
		default:
			return nil, fmt.Errorf("method %s not allowed for endpoint %s", method, endpoint)
		}

	case "/scanners":
		if method != "GET" {
			return nil, fmt.Errorf("method %s not allowed for endpoint %s", method, endpoint)
		}
		scanners, err := h.foodService.ListScanners()
		if err != nil {
			return nil, err
		}
		return scanners, nil

	case "/scanners/active":
		if method != "POST" {
			return nil, fmt.Errorf("method %s not allowed for endpoint %s", method, endpoint)
		}
		devicePath, ok := requestDataMap["path"].(string)
		if !ok {
			return nil, errors.New("invalid device path")
		}
		err := h.foodService.SetActiveScanner(devicePath)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"message": "Active scanner set"}, nil

	default:
		return nil, fmt.Errorf("unknown endpoint: %s", endpoint)
	}
}

func (h *StandardIOHandler) sendResponse(data interface{}, requestId string) {
	response := Response{
		Type:      "response",
		Data:      data,
		RequestId: requestId,
	}
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		log.Printf("Error encoding response for request %s: %v\n", requestId, err)
	}
}

func (h *StandardIOHandler) sendErrorResponse(message string, requestId string) {
	response := Response{
		Type: "response",
		Data: map[string]interface{}{
			"error": map[string]string{
				"message": message,
				"code":    "BACKEND_ERROR",
			},
		},
		RequestId: requestId,
	}

	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		log.Printf("Error encoding error response for request %s: %v\n", requestId, err)
		fallbackResponse := fmt.Sprintf(`{"type":"response","data":{"error":{"message":"Internal server error","code":"INTERNAL_ERROR"}},"requestId":"%s"}`, requestId)
		fmt.Println(fallbackResponse)
	}
}
