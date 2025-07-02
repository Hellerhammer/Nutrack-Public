import React, { useState, useEffect, useCallback } from "react";
import {
  Button,
  Box,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
  List,
  ListItem,
  Grid2 as Grid,
  Paper,
  InputAdornment,
  IconButton,
  Tooltip,
} from "@mui/material";
import AddIcon from "@mui/icons-material/Add";
import DeleteIcon from "@mui/icons-material/Delete";
import EditIcon from "@mui/icons-material/Edit";
import FastfoodIcon from "@mui/icons-material/Fastfood";
import RestaurantIcon from "@mui/icons-material/Restaurant";
import PersistentFoodItemSearch from "./persistentfooditemssearch";
import useSnackbar from "../hooks/useSnackbar";
import { formatNumberForDisplay } from "../utils/formatter";
import { Snackbar, Alert } from "@mui/material";
import { apiService } from "../services/apiService";

const Dishes = ({ onDishesChange }) => {
  const [openDishListDialog, setOpenDishListDialog] = useState(false);
  const [openDishDialog, setOpenDishDialog] = useState(false);
  const [dishes, setDishes] = useState([]);
  const [newDish, setNewDish] = useState({ name: "" });
  const [editingDish, setEditingDish] = useState(null);
  const [dishItems, setDishItems] = useState([]);
  const [dishNutrients, setDishNutrients] = useState({
    "energy-kcal": 0,
    proteins: 0,
    carbohydrates: 0,
    fat: 0,
  });
  const { snackbar, showSnackbar, closeSnackbar } = useSnackbar();

  const fetchDishes = useCallback(async () => {
    try {
      const response = await apiService.makeRequest("GET", "/dishes");
      console.log(response);
      setDishes(response.dishes || []);
    } catch (error) {
      console.error("Error fetching dishes:", error);
      showSnackbar("Error fetching dishes. Please try again later.", "error");
    }
  }, [showSnackbar]);

  useEffect(() => {
    if (openDishListDialog) {
      fetchDishes();
    }
  }, [openDishListDialog, fetchDishes]);

  const handleShowDishList = () => {
    setOpenDishListDialog(true);
  };

  const handleCloseDishListDialog = () => {
    setOpenDishListDialog(false);
  };

  const handleAddDish = () => {
    setOpenDishDialog(true);
  };

  const handleCloseDishDialog = () => {
    setOpenDishDialog(false);
    setNewDish({ name: "" });
    setDishItems([]);
    setDishNutrients({
      "energy-kcal": 0,
      proteins: 0,
      carbohydrates: 0,
      fat: 0,
    });
    setEditingDish(null);
  };

  const handleDishInputChange = (event) => {
    const { name, value } = event.target;
    setNewDish({ ...newDish, [name]: value });
  };

  const handleIngredientSelect = (item) => {
    setDishItems((prevItems) => [
      ...prevItems,
      {
        ...item,
        quantity: item.serving_quantity || 100,
        unit: item.serving_quantity_unit || "g",
      },
    ]);
    updateDishNutrients([
      ...dishItems,
      {
        ...item,
        quantity: item.serving_quantity || 100,
        unit: item.serving_quantity_unit || "g",
      },
    ]);
  };

  const handleDishItemChange = (index, field, value) => {
    if (field === "quantity") {
      const numericValue = parseFloat(formatNumberForDisplay(value.toString()));
      value = isNaN(numericValue) ? 0 : numericValue;
    }

    const updatedItems = [...dishItems];
    updatedItems[index][field] = value;
    setDishItems(updatedItems);
    updateDishNutrients(updatedItems);
  };

  const handleRemoveDishItem = (index) => {
    const updatedItems = dishItems.filter((_, i) => i !== index);
    setDishItems(updatedItems);
    updateDishNutrients(updatedItems);
  };

  const calculateDishNutrients = (items) => {
    return items.reduce(
      (acc, item) => {
        const quantity = parseFloat(item.quantity) || 0;
        acc["energy-kcal"] +=
          ((parseFloat(item["energy-kcal_100g"]) || 0) / 100) * quantity;
        acc.proteins +=
          ((parseFloat(item.proteins_100g) || 0) / 100) * quantity;
        acc.carbohydrates +=
          ((parseFloat(item.carbohydrates_100g) || 0) / 100) * quantity;
        acc.fat += ((parseFloat(item.fat_100g) || 0) / 100) * quantity;

        acc.totalWeight += quantity;

        return acc;
      },
      {
        "energy-kcal": 0,
        proteins: 0,
        carbohydrates: 0,
        fat: 0,
        totalWeight: 0,
      }
    );
  };

  // Function to calculate nutrients per 100g
  const calculateNutrientsPer100g = (totalNutrients) => {
    const { totalWeight, ...nutrients } = totalNutrients;
    if (totalWeight === 0) return nutrients;

    return Object.entries(nutrients).reduce((acc, [key, value]) => {
      acc[key] = (value / totalWeight) * 100;
      return acc;
    }, {});
  };

  // Function to update dish nutrients
  const updateDishNutrients = (items) => {
    const totalNutrients = calculateDishNutrients(items);
    const nutrientsPer100g = calculateNutrientsPer100g(totalNutrients);
    setDishNutrients(nutrientsPer100g);
  };

  const handleCreateDish = async () => {
    try {
      const dishData = {
        dish: {
          name: newDish.name,
          barcode: newDish.barcode || null, // Optional barcode
        },
        items: dishItems.map((item) => ({
          barcode: item.barcode,
          quantity: item.quantity,
        })),
      };

      await apiService.makeRequest("POST", "/dishes", dishData);
      handleCloseDishDialog();
      fetchDishes();
      if (onDishesChange) onDishesChange();
      showSnackbar("Dish created successfully", "success");
    } catch (error) {
      console.error("Error creating dish:", error);
      const errorMessage = error.response?.data?.error || error.message || "Error creating dish";
      showSnackbar(errorMessage, "error");
    }
  };

  const handleDeleteDish = async (dishId) => {
    try {
      await apiService.makeRequest("DELETE", `/dishes`, [], [dishId]);
      fetchDishes();
      showSnackbar("Dish deleted successfully", "success");
    } catch (error) {
      console.error("Error deleting dish:", error);
      showSnackbar("Error deleting dish. Please try again.", "error");
    }
  };

  const handleEditDish = (dish) => {
    setEditingDish(dish);
    setNewDish({ name: dish.name, barcode: dish.barcode || null });
    setDishItems(
      dish.dish_items.map((item) => ({
        ...item.food_item,
        quantity: item.quantity,
        unit: item.food_item.serving_quantity_unit,
      }))
    );
    updateDishNutrients(
      dish.dish_items.map((item) => ({
        ...item.food_item,
        quantity: item.quantity,
        unit: item.food_item.serving_quantity_unit,
      }))
    );
    setOpenDishDialog(true);
  };

  const handleUpdateDish = async () => {
    try {
      const dishData = {
        dish: {
          name: newDish.name,
          barcode: newDish.barcode || null,
        },
        items: dishItems.map((item) => ({
          barcode: item.barcode,
          quantity: item.quantity,
        })),
      };

      await apiService.makeRequest("PUT", `/dishes`, dishData, [
        editingDish.id,
      ]);
      handleCloseDishDialog();
      fetchDishes();
      if (onDishesChange) onDishesChange();
      showSnackbar("Dish updated successfully", "success");
    } catch (error) {
      console.error("Error updating dish:", error);
      showSnackbar("Error updating dish. Please try again.", "error");
    }
  };

  const handleConvertToFoodItem = async (dish) => {
    try {
      const response = await apiService.makeRequest(
        "POST",
        `/dishes/convert-to-food-item`,
        [],
        [dish.id]
      );
      if (response.status === 200) {
        showSnackbar(
          "Dish successfully converted to/updated food item",
          "success"
        );
        fetchDishes();
        if (onDishesChange) onDishesChange();
      }
    } catch (error) {
      console.error("Error converting dish to food item:", error);
      if (error.response && error.response.status === 409) {
        showSnackbar(error.response.data.error, "warning");
      } else {
        showSnackbar(
          "Error converting dish to food item. Please try again.",
          "error"
        );
      }
    }
  };

  return (
    <>
      <Button
        onClick={handleShowDishList}
        variant="contained"
        startIcon={<RestaurantIcon />}
        style={{ marginRight: "10px", height: "40px" }}
      >
        Manage Dishes
      </Button>
      <Dialog
        open={openDishListDialog}
        onClose={handleCloseDishListDialog}
        maxWidth="md"
        fullWidth
      >
        <DialogTitle>Dish List</DialogTitle>
        <DialogContent>
          {dishes.length === 0 ? (
            <Typography
              variant="body1"
              align="left"
              style={{ marginTop: "20px" }}
            >
              No dishes available. Click 'Add Dish' to create a new dish.
            </Typography>
          ) : (
            <List>
              {dishes.map((dish, dishIndex) => (
                <ListItem
                  key={`dish-${dishIndex}`}
                  divider
                  alignItems="flex-start"
                  style={{ display: "flex", flexDirection: "column" }}
                >
                  <Box
                    sx={{
                      width: "100%",
                      display: "flex",
                      justifyContent: "space-between",
                      alignItems: "flex-start",
                    }}
                  >
                    <Box sx={{ flexGrow: 1 }}>
                      <Typography variant="subtitle1" component="div">
                        {dish.name}
                      </Typography>
                      <Typography
                        variant="body2"
                        color="text.secondary"
                        component="div"
                        sx={{ mt: 1 }}
                      >
                        Ingredients:{" "}
                        {dish.dish_items && dish.dish_items.length > 0
                          ? dish.dish_items
                              .map(
                                (item, itemIndex) =>
                                  `${
                                    item.food_item.name
                                  } (${formatNumberForDisplay(item.quantity)}${
                                    item.food_item.serving_quantity_unit
                                  })${
                                    itemIndex < dish.dish_items.length - 1
                                      ? ", "
                                      : ""
                                  }`
                              )
                              .join("")
                          : "No ingredients"}
                      </Typography>
                      {dish.dish_items && dish.dish_items.length > 0 && (
                        <Typography
                          variant="body2"
                          color="text.secondary"
                          component="div"
                          sx={{ mt: 0.5 }}
                        >
                          Nutrients per 100g:{" "}
                          {(() => {
                            const totalNutrients = calculateDishNutrients(
                              dish.dish_items.map((item) => ({
                                ...item.food_item,
                                quantity: item.quantity,
                                unit: item.food_item.serving_quantity_unit,
                              }))
                            );
                            const nutrientsPer100g =
                              calculateNutrientsPer100g(totalNutrients);
                            return `Calories: ${formatNumberForDisplay(
                              nutrientsPer100g["energy-kcal"]
                            )} kcal, 
                                    Protein: ${formatNumberForDisplay(
                                      nutrientsPer100g.proteins
                                    )}g, 
                                    Carbs: ${formatNumberForDisplay(
                                      nutrientsPer100g.carbohydrates
                                    )}g, 
                                    Fat: ${formatNumberForDisplay(
                                      nutrientsPer100g.fat
                                    )}g, 
                                    Total Weight: ${formatNumberForDisplay(
                                      totalNutrients.totalWeight
                                    )}g`;
                          })()}
                        </Typography>
                      )}
                    </Box>
                    <Box sx={{ display: "flex", ml: 2, mt: 4 }}>
                      <Tooltip title="Convert to/update Food Item">
                        <IconButton
                          aria-label="convert"
                          onClick={() => handleConvertToFoodItem(dish)}
                          size="small"
                        >
                          <FastfoodIcon />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="Edit Dish">
                        <IconButton
                          aria-label="edit"
                          onClick={() => handleEditDish(dish)}
                          size="small"
                        >
                          <EditIcon />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="Delete Dish">
                        <IconButton
                          aria-label="delete"
                          onClick={() => handleDeleteDish(dish.id)}
                          size="small"
                        >
                          <DeleteIcon />
                        </IconButton>
                      </Tooltip>
                    </Box>
                  </Box>
                </ListItem>
              ))}
            </List>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={handleAddDish} startIcon={<AddIcon />}>
            Add Dish
          </Button>
          <Button onClick={handleCloseDishListDialog}>Close</Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={openDishDialog}
        onClose={handleCloseDishDialog}
        maxWidth="md"
        fullWidth
        PaperProps={{
          style: {
            maxHeight: "90vh",
          },
        }}
      >
        <DialogTitle>Create New Dish</DialogTitle>
        <DialogContent style={{ overflowY: "auto", height: "calc(60vh)" }}>
          <TextField
            name="name"
            label="Dish Name"
            value={newDish.name}
            onChange={handleDishInputChange}
            fullWidth
            margin="normal"
          />
          <TextField
            name="barcode"
            label="Barcode (optional)"
            value={newDish.barcode || ""}
            onChange={handleDishInputChange}
            fullWidth
            margin="normal"
          />
          <Typography
            variant="h6"
            style={{ marginTop: "20px", marginBottom: "10px" }}
          >
            Dish Nutrients (per 100g)
          </Typography>
          <Grid container spacing={2} justifyContent="space-between">
            {Object.entries(dishNutrients).map(([nutrient, value]) => (
              <Grid item xs={12} sm={6} md={3} key={nutrient}>
                <Paper
                  elevation={2}
                  style={{
                    padding: "10px",
                    textAlign: "center",
                    height: "100%",
                    display: "flex",
                    flexDirection: "column",
                    justifyContent: "center",
                    minWidth: "200px",
                  }}
                >
                  <Typography variant="body2" color="textSecondary">
                    {nutrient === "energy-kcal"
                      ? "Calories"
                      : nutrient.charAt(0).toUpperCase() + nutrient.slice(1)}
                  </Typography>
                  <Typography variant="h6">
                    {formatNumberForDisplay(value)}{" "}
                    {nutrient === "energy-kcal" ? "kcal" : "g"}
                  </Typography>
                </Paper>
              </Grid>
            ))}
          </Grid>
          <Typography variant="h6" style={{ marginTop: "20px" }}>
            Ingredients
          </Typography>
          <div style={{ position: "relative", zIndex: 1 }}>
            <PersistentFoodItemSearch onItemSelect={handleIngredientSelect} />
          </div>
          {dishItems.map((item, index) => (
            <div
              key={index}
              style={{
                display: "flex",
                marginBottom: "10px",
                alignItems: "center",
              }}
            >
              <Typography style={{ flexGrow: 1 }}>{item.name}</Typography>
              <TextField
                label="Quantity"
                value={item.quantity}
                InputLabelProps={{
                  style: { zIndex: 0 },
                }}
                onChange={(e) =>
                  handleDishItemChange(
                    index,
                    "quantity",
                    parseFloat(e.target.value)
                  )
                }
                slotProps={{
                  input: {
                    endAdornment: (
                      <InputAdornment position="end">
                        {item.unit}
                      </InputAdornment>
                    ),
                  },
                }}
                style={{ width: "150px", marginRight: "10px" }}
              />
              <IconButton onClick={() => handleRemoveDishItem(index)}>
                <DeleteIcon />
              </IconButton>
            </div>
          ))}
        </DialogContent>
        <DialogActions>
          <Button onClick={handleCloseDishDialog}>Cancel</Button>
          <Button
            onClick={editingDish ? handleUpdateDish : handleCreateDish}
            variant="contained"
            color="primary"
          >
            {editingDish ? "Update Dish" : "Create Dish"}
          </Button>
        </DialogActions>
      </Dialog>
      <Snackbar
        open={snackbar.open}
        autoHideDuration={5000}
        onClose={closeSnackbar}
      >
        <Alert onClose={closeSnackbar} severity={snackbar.severity}>
          {snackbar.message}
        </Alert>
      </Snackbar>
    </>
  );
};

export default Dishes;
