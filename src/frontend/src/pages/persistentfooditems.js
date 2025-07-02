import React, { useState, useEffect, useCallback, useRef } from "react";
import { DataGrid, GridActionsCellItem } from "@mui/x-data-grid";
import {
  Button,
  Tooltip,
  Snackbar,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
} from "@mui/material";
import MuiAlert from "@mui/material/Alert";
import DeleteIcon from "@mui/icons-material/Delete";
import RestoreIcon from "@mui/icons-material/Restore";
import OpenFoodFactsSearch from "../components/openfoodfactssearch";
import Dishes from "../components/dishes";
import { configPromise } from "../config";
import {
  formatNumberForDisplay,
  formatForBackend,
  formatNumericInput,
} from "../utils/formatter";
import { useSnackbar } from "../hooks/useSnackbar";
import { useSSE } from "../hooks/useSSE";
import AddIcon from "@mui/icons-material/Add";
import OpenInNewIcon from "@mui/icons-material/OpenInNew";
import { debounce } from "lodash";
import { apiService } from "../services/apiService";
import dropboxService from "../services/dropboxService";

const Alert = React.forwardRef(function Alert(props, ref) {
  return <MuiAlert elevation={6} ref={ref} variant="filled" {...props} />;
});

const PersistentFoodItems = () => {
  const [foodItems, setFoodItems] = useState([]);
  const [loading, setLoading] = useState(false);

  const lastRefreshRef = useRef(Date.now());
  const REFRESH_COOLDOWN = 1000;
  const { snackbar, showSnackbar, closeSnackbar } = useSnackbar();
  const [openDialog, setOpenDialog] = useState(false);
  const [newItem, setNewItem] = useState({
    barcode: "",
    name: "",
    "energy-kcal_100g": "",
    proteins_100g: "",
    carbohydrates_100g: "",
    fat_100g: "",
    serving_quantity: "",
    serving_quantity_unit: "",
  });

  const [columnVisibilityModel, setColumnVisibilityModel] = useState(() => {
    const savedModel = localStorage.getItem("columnVisibilityModel");
    return savedModel
      ? JSON.parse(savedModel)
      : {
          serving_quantity_unit: false,
        };
  });

  useEffect(() => {
    localStorage.setItem(
      "columnVisibilityModel",
      JSON.stringify(columnVisibilityModel)
    );
  }, [columnVisibilityModel]);

  const fetchFoodItems = useCallback(async (syncing = false, forceSync = false) => {
    setLoading(true);
    try {
      await configPromise;
      if (syncing) {
        await dropboxService.sync(forceSync);
      }
      const response = await apiService.makeRequest("GET", "/foodItems/all");
      console.log("Response:", response);

      const foodItemsArray = Object.values(response || {});

      setFoodItems(foodItemsArray);
    } catch (error) {
      console.error("Error fetching food items:", error);
      showSnackbar("Failed to fetch food items", "error");
      setFoodItems([]); // Ensure we set an empty array on error
    } finally {
      setLoading(false);
    }
  }, [showSnackbar]);

  useEffect(() => {
    fetchFoodItems();
  }, [fetchFoodItems]);

  const handleSSEMessage = useCallback(
    (message) => {
      if (message === "food_items_updated") {
        console.log("Updating food items due to SSE message:", message);
        fetchFoodItems();
        lastRefreshRef.current = Date.now();
      }
    },
    [fetchFoodItems]
  );

  // Use the SSE hook
  useSSE(handleSSEMessage);

  const resetRefreshCooldown = useCallback(() => {
    console.log("Resetting refresh cooldown");
    lastRefreshRef.current = 0;
  }, []);

  const refreshFoodItems = useCallback(
    debounce(() => {
      const now = Date.now();
      if (now - lastRefreshRef.current > REFRESH_COOLDOWN) {
        console.log("Refreshing food items");
        fetchFoodItems();
        lastRefreshRef.current = now;
      } else {
        console.log("Skipping refresh due to recent update");
      }
    }, 300),
    [fetchFoodItems, REFRESH_COOLDOWN]
  );

  const processRowUpdate = useCallback(
    async (newRow, oldRow) => {
      const changedFields = Object.keys(newRow).filter(
        (field) => newRow[field] !== oldRow[field]
      );

      if (changedFields.length === 0) {
        return oldRow; // No changes were made
      }

      const updatedFields = {};
      changedFields.forEach((field) => {
        if (
          field === "barcode" ||
          field === "name" ||
          field === "serving_quantity_unit"
        ) {
          updatedFields[field] = newRow[field];
        } else {
          const formattedValue = formatNumericInput(newRow[field]);
          updatedFields[field] = formatForBackend(formattedValue);
          if (isNaN(updatedFields[field])) {
            showSnackbar(
              `Error formatting field ${field}. Please enter a valid number.`,
              "error"
            );
            throw new Error(`Invalid value for ${field}`);
          }
        }
      });

      try {
        resetRefreshCooldown();
        if (changedFields.includes("barcode")) {
        await apiService.makeRequest(
            "DELETE",
            `/foodItems`,
            [],
            [oldRow.barcode]
          );
          await apiService.makeRequest(
            "POST",
            `/foodItems/manually-add`,
            newRow
          );
        } else {
          await apiService.makeRequest("PUT", `/foodItems`, updatedFields, [
            newRow.barcode,
          ]);
        }

        showSnackbar("Item updated successfully", "success");
        refreshFoodItems();
        return { ...newRow, ...updatedFields };
      } catch (error) {
        console.error("Error updating food item:", error);
        showSnackbar(
          error.response?.data?.error || "Failed to update food item",
          "error"
        );
        throw error; // This will cause the grid to revert to the old value
      }
    },
    [refreshFoodItems, showSnackbar, resetRefreshCooldown]
  );
  const handleDeleteClick = useCallback(
    (id) => async () => {
      setLoading(true);
      console.log("Deleting item with id:", id);
      try {
        resetRefreshCooldown();
        await apiService.makeRequest("DELETE", `/foodItems`, [], [id]);
        setFoodItems((prevItems) =>
          prevItems.filter((item) => item.barcode !== id)
        );
        refreshFoodItems();
      } catch (error) {
        console.error("Error deleting food item:", error);
      } finally {
        setLoading(false);
      }
    },
    [refreshFoodItems, resetRefreshCooldown]
  );

  const handleResetClick = useCallback(
    (id) => async () => {
      setLoading(true);
      try {
        resetRefreshCooldown();
        await apiService.makeRequest("POST", `/foodItems/reset`, [], [id]);
        showSnackbar("Food item reset successfully", "success");
        refreshFoodItems();
      } catch (error) {
        console.error("Error resetting food item:", error);
        showSnackbar(
          error.response?.data?.error || "Error resetting food item",
          "error"
        );
      } finally {
        setLoading(false);
      }
    },
    [refreshFoodItems, showSnackbar, resetRefreshCooldown]
  );

  const addFoundItemToList = async (product) => {
    setLoading(true);
    if (!product.code) {
      showSnackbar("Invalid product: missing barcode", "error");
      setLoading(false);
      return;
    }
    const newItem = {
      barcode: product.code,
      name: product.product_name,
      "energy-kcal_100g": formatForBackend(
        product.nutriments["energy-kcal_100g"]
      ),
      proteins_100g: formatForBackend(product.nutriments.proteins_100g),
      carbohydrates_100g: formatForBackend(
        product.nutriments.carbohydrates_100g
      ),
      fat_100g: formatForBackend(product.nutriments.fat_100g),
      serving_quantity: formatForBackend(product.serving_quantity),
      serving_quantity_unit: product.serving_quantity_unit,
    };

    try {
      // API call to store the new item in the backend
      resetRefreshCooldown();
      const response = await apiService.makeRequest(
        "POST",
        "/foodItems/check-and-insert",
        {
          barcode: newItem.barcode,
        }
      );

      if (response.exists) {
        showSnackbar("Item already exists in the database", "info");
      } else {
        // Update the local list only if the item was successfully added
        setFoodItems((prevItems) => {
          console.log("Previous items:", prevItems);
          const currentItems = Array.isArray(prevItems) ? prevItems : [];
          return [...currentItems, newItem];
        });
        await fetchFoodItems();
        showSnackbar("Item added successfully", "success");
      }
      refreshFoodItems();
    } catch (error) {
      console.error("Error adding item:", error);
      showSnackbar("Failed to add item", "error");
    } finally {
      setLoading(false);
    }
  };

  const handleAddManually = () => {
    setOpenDialog(true);
  };
  const handleCloseDialog = () => {
    setOpenDialog(false);
    setNewItem({
      barcode: "",
      name: "",
      "energy-kcal_100g": "",
      proteins_100g: "",
      carbohydrates_100g: "",
      fat_100g: "",
      serving_quantity: "",
      serving_quantity_unit: "",
    });
  };

  const handleInputChange = (event) => {
    const { name, value } = event.target;
    let parsedValue = value;

    if (
      [
        "energy-kcal_100g",
        "proteins_100g",
        "carbohydrates_100g",
        "fat_100g",
        "serving_quantity",
      ].includes(name)
    ) {
      parsedValue = formatNumericInput(value);
    }

    setNewItem({ ...newItem, [name]: parsedValue });
  };

  const handleSubmitNewItem = async () => {
    if (!newItem.barcode || !newItem.serving_quantity) {
      showSnackbar("Barcode and Serving Quantity are required", "error");
      return;
    }

    const formattedNewItem = {
      ...newItem,
      "energy-kcal_100g": formatForBackend(newItem["energy-kcal_100g"]),
      proteins_100g: formatForBackend(newItem.proteins_100g),
      carbohydrates_100g: formatForBackend(newItem.carbohydrates_100g),
      fat_100g: formatForBackend(newItem.fat_100g),
      serving_quantity: formatForBackend(newItem.serving_quantity),
    };

    try {
      const response = await apiService.makeRequest(
        "POST",
        "/foodItems/manually-add",
        formattedNewItem
      );
      if (response) {
        showSnackbar("Item added successfully", "success");
        fetchFoodItems();
        handleCloseDialog();
      }
    } catch (error) {
      console.error("Error adding new item:", error);
      showSnackbar("Failed to add item: " + error, "error");
    }
  };

  const handleColumnVisibilityModelChange = useCallback((newModel) => {
    setColumnVisibilityModel(newModel);
  }, []);

  const columns = [
    {
      field: "barcode",
      headerName: "Barcode",
      width: 150,
      editable: true,
      headerAlign: "left",
      align: "left",
    },
    {
      field: "name",
      headerName: "Name",
      width: 200,
      editable: true,
      headerAlign: "left",
      align: "left",
    },
    {
      field: "serving_quantity",
      headerName: "Serving Quantity",
      width: 150,
      editable: true,
      headerAlign: "left",
      align: "left",
      renderCell: (params) =>
        `${formatNumberForDisplay(params.value)} ${
          params.row.serving_quantity_unit
        }` || "g",
    },
    {
      field: "serving_quantity_unit",
      headerName: "Serving Quantity Unit",
      width: 160,
      editable: true,
      headerAlign: "left",
      align: "left",
    },

    {
      field: "energy-kcal_100g",
      headerName: "Calories/100g",
      width: 150,
      editable: true,
      headerAlign: "left",
      align: "left",
      renderCell: (params) => `${formatNumberForDisplay(params.value)} kcal`,
    },
    {
      field: "fat_100g",
      headerName: "Fat/100g",
      width: 150,
      editable: true,
      headerAlign: "left",
      align: "left",
      renderCell: (params) => `${formatNumberForDisplay(params.value)} g`,
    },
    {
      field: "carbohydrates_100g",
      headerName: "Carbs/100g",
      width: 150,
      editable: true,
      headerAlign: "left",
      align: "left",
      renderCell: (params) => `${formatNumberForDisplay(params.value)} g`,
    },
    {
      field: "proteins_100g",
      headerName: "Protein/100g",
      width: 150,
      editable: true,
      headerAlign: "left",
      align: "left",
      renderCell: (params) => `${formatNumberForDisplay(params.value)} g`,
    },

    {
      field: "last_updated",
      headerName: "Last Updated",
      width: 180,
      headerAlign: "left",
      align: "left",
      renderCell: (cellValues) => {
        if (cellValues.row && cellValues.row.last_updated) {
          const date = new Date(cellValues.row.last_updated);
          return date.toLocaleString("en-GB", {
            year: "numeric",
            month: "2-digit",
            day: "2-digit",
            hour: "2-digit",
            minute: "2-digit",
            second: "2-digit",
          });
        }
        return "N/A";
      },
    },
    {
      field: "actions",
      type: "actions",
      headerName: "Actions",
      cellClassName: "actions",
      flex: 1,
      align: "right",
      headerAlign: "right",
      getActions: ({ id }) => [
        <Tooltip title="View on OpenFoodFacts, if available">
          <GridActionsCellItem
            icon={<OpenInNewIcon />}
            label="View"
            onClick={() =>
              window.open(
                `https://world.openfoodfacts.org/product/${id}`,
                "_blank"
              )
            }
            color="inherit"
          />
        </Tooltip>,
        <Tooltip title="Reset to data from OpenFoodFacts, if available">
          <GridActionsCellItem
            icon={<RestoreIcon />}
            label="Reset"
            onClick={handleResetClick(id)}
            color="inherit"
          />
        </Tooltip>,
        <Tooltip title="Delete this item">
          <GridActionsCellItem
            icon={<DeleteIcon />}
            label="Delete"
            onClick={handleDeleteClick(id)}
            color="inherit"
          />
        </Tooltip>,
      ],
    },
  ];

  return (
    <div
      style={{
        height: "calc(100vh - 100px)",
        width: "100%",
        display: "flex",
        flexDirection: "column",
      }}
    >
      <OpenFoodFactsSearch onItemSelect={addFoundItemToList} />
      <div
        style={{
          display: "flex",
          justifyContent: "flex-end",
          alignItems: "center",
          marginBottom: "36px",
        }}
      >
        <Dishes onDishesChange={refreshFoodItems} />
        <Button
          onClick={handleAddManually}
          variant="contained"
          startIcon={<AddIcon />}
          style={{ marginRight: "10px", height: "40px" }}
        >
          Add Manually
        </Button>
        <Button
          onClick={() => fetchFoodItems(true, true)}
          variant="contained"
          style={{ height: "40px" }}
        >
          Refresh
        </Button>
      </div>
      <div
        style={{
          flexGrow: 1,
          width: "100%",
          minHeight: "163px",
        }}
      >
        <DataGrid
          rows={foodItems}
          columns={columns}
          getRowId={(row) => row.barcode}
          autoPageSize={true}
          loading={loading}
          editMode="cell"
          processRowUpdate={processRowUpdate}
          onProcessRowUpdateError={(error) => {
            console.error("Error updating row:", error);
          }}
          initialState={{
            columns: {
              columnVisibilityModel: columnVisibilityModel,
            },
          }}
          onColumnVisibilityModelChange={handleColumnVisibilityModelChange}
        />
        <Snackbar
          open={snackbar.open}
          autoHideDuration={5000}
          onClose={closeSnackbar}
        >
          <Alert onClose={closeSnackbar} severity={snackbar.severity}>
            {snackbar.message}
          </Alert>
        </Snackbar>
      </div>
      <Dialog open={openDialog} onClose={handleCloseDialog}>
        <DialogTitle>Add New Food Item</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            margin="dense"
            name="barcode"
            label="Barcode (required)"
            type="text"
            fullWidth
            value={newItem.barcode}
            onChange={handleInputChange}
            required
          />
          <TextField
            margin="dense"
            name="name"
            label="Name"
            type="text"
            fullWidth
            value={newItem.name}
            onChange={handleInputChange}
          />
          <TextField
            margin="dense"
            name="energy-kcal_100g"
            label="Calories/100g"
            fullWidth
            value={newItem["energy-kcal_100g"]}
            onChange={handleInputChange}
          />
          <TextField
            margin="dense"
            name="fat_100g"
            label="Fat/100g"
            fullWidth
            value={newItem.fat_100g}
            onChange={handleInputChange}
          />
          <TextField
            margin="dense"
            name="carbohydrates_100g"
            label="Carbs/100g"
            fullWidth
            value={newItem.carbohydrates_100g}
            onChange={handleInputChange}
          />
          <TextField
            margin="dense"
            name="proteins_100g"
            label="Protein/100g"
            fullWidth
            value={newItem.proteins_100g}
            onChange={handleInputChange}
          />
          <TextField
            margin="dense"
            name="serving_quantity"
            label="Serving Quantity (required)"
            fullWidth
            value={newItem.serving_quantity}
            onChange={handleInputChange}
            required
          />
          <TextField
            margin="dense"
            name="serving_quantity_unit"
            label="Serving Quantity Unit"
            type="text"
            fullWidth
            value={newItem.serving_quantity_unit}
            onChange={handleInputChange}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={handleCloseDialog}>Cancel</Button>
          <Button onClick={handleSubmitNewItem}>Add</Button>
        </DialogActions>
      </Dialog>
    </div>
  );
};
export default PersistentFoodItems;
