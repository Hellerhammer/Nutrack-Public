import React, { useState, useEffect, useCallback, useRef } from "react";
import { DataGrid } from "@mui/x-data-grid";
import {
  Button,
  Tooltip,
  IconButton,
  Typography,
  Box,
  Snackbar,
  Dialog,
  DialogTitle,
  DialogContent,
} from "@mui/material";
import MuiAlert from "@mui/material/Alert";
import { AdapterDateFns } from "@mui/x-date-pickers/AdapterDateFns";
import { LocalizationProvider } from "@mui/x-date-pickers/LocalizationProvider";
import { DatePicker } from "@mui/x-date-pickers/DatePicker";
import PersistentFoodItemSearch from "../components/persistentfooditemssearch";
import {
  formatNumberForDisplay,
  formatForBackend,
  formatNumericInput,
} from "../utils/formatter";
import DeleteIcon from "@mui/icons-material/Delete";
import AddIcon from "@mui/icons-material/Add";
import RemoveIcon from "@mui/icons-material/Remove";
import { configPromise } from "../config";
import RestoreIcon from "@mui/icons-material/Restore";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip as RechartsTooltip,
  Legend,
  ResponsiveContainer,
  ReferenceLine,
} from "recharts";
import { useTheme } from "@mui/material/styles";
import { format, subDays } from "date-fns";
import { debounce } from "lodash";
import { useSSE } from "../hooks/useSSE";
import { apiService } from "../services/apiService";
import dropboxService from "../services/dropboxService"; 

const Alert = React.forwardRef(function Alert(props, ref) {
  return <MuiAlert elevation={6} ref={ref} variant="filled" {...props} />;
});

const Home = () => {
  const theme = useTheme();
  const [consumedItems, setConsumedItems] = useState([]);
  const [loading, setLoading] = useState(true);
  const [selectedDate, setSelectedDate] = useState(new Date());
  const [weeklyData, setWeeklyData] = useState([]);
  const [openChart, setOpenChart] = useState(false);

  const lastRefreshRef = useRef(Date.now());
  const REFRESH_COOLDOWN = 1000;

  const [snackbar, setSnackbar] = useState({
    open: false,
    message: "",
    severity: "info",
  });
  const [totalNutrition, setTotalNutrition] = useState({
    calories: 0,
    protein: 0,
    carbs: 0,
    fat: 0,
  });
  const [userSettings, setUserSettings] = useState({
    calories: 0,
    proteins: 0,
    carbs: 0,
    fat: 0,
  });
  const calculateNutrition = useCallback((item, servingQuantity) => {
    const factor =
      (parseFloat(servingQuantity) / 100) * parseFloat(item.consumed_quantity);
    return {
      calories: formatForBackend(parseFloat(item.calories_per_100g) * factor),
      protein: formatForBackend(parseFloat(item.protein_per_100g) * factor),
      carbs: formatForBackend(parseFloat(item.carbs_per_100g) * factor),
      fat: formatForBackend(parseFloat(item.fat_per_100g) * factor),
    };
  }, []);

  const calculateTotalNutrition = useCallback((items) => {
    const totals = items.reduce(
      (acc, item) => ({
        calories: acc.calories + parseFloat(item.calories),
        protein: acc.protein + parseFloat(item.protein),
        carbs: acc.carbs + parseFloat(item.carbs),
        fat: acc.fat + parseFloat(item.fat),
      }),
      { calories: 0, protein: 0, carbs: 0, fat: 0 }
    );

    setTotalNutrition({
      calories: formatNumberForDisplay(totals.calories),
      protein: formatNumberForDisplay(totals.protein),
      carbs: formatNumberForDisplay(totals.carbs),
      fat: formatNumberForDisplay(totals.fat),
    });
  }, []);

  const fetchConsumedItems = useCallback(
    async (date, syncing = false, forceSync = false) => {
      setLoading(true);
      try {
        await configPromise;
        if (syncing) {
          await dropboxService.sync(forceSync);
        }
        const adjustedDate = new Date(
          date.getTime() - date.getTimezoneOffset() * 60000
        );
        const formattedDate = adjustedDate.toISOString().split("T")[0];
        const response = await apiService.makeRequest(
          "GET",
          `/consumedFoodItems`,
          [],
          [formattedDate]
        );

        const responseArray = Array.isArray(response)
          ? response
          : Object.values(response || {});

        if (!responseArray || responseArray.length === 0) {
          setConsumedItems([]);
          calculateTotalNutrition([]);
          return;
        }

        const items = responseArray
          .filter((item) => item.id)
          .map((item) => ({
            id: item.id,
            ...item,
            calories:
              ((parseFloat(item.calories_per_100g) *
                parseFloat(item.serving_quantity)) /
                100) *
              parseFloat(item.consumed_quantity),
            protein:
              ((parseFloat(item.protein_per_100g) *
                parseFloat(item.serving_quantity)) /
                100) *
              parseFloat(item.consumed_quantity),
            carbs:
              ((parseFloat(item.carbs_per_100g) *
                parseFloat(item.serving_quantity)) /
                100) *
              parseFloat(item.consumed_quantity),
            fat:
              ((parseFloat(item.fat_per_100g) *
                parseFloat(item.serving_quantity)) /
                100) *
              parseFloat(item.consumed_quantity),
          }));
        setConsumedItems(items);
        calculateTotalNutrition(items);
      } catch (error) {
        console.error("Error fetching consumed items:", error);
      } finally {
        setLoading(false);
      }
    },
    [calculateTotalNutrition]
  );

  const resetRefreshCooldown = useCallback(() => {
    console.log("Resetting refresh cooldown");
    lastRefreshRef.current = 0;
  }, []);

  // Create a ref to hold our debounced function
  const refreshConsumedItemsRef = useRef(null);
  
  // Update the debounced function when dependencies change
  useEffect(() => {
    refreshConsumedItemsRef.current = debounce(() => {
      const now = Date.now();
      if (now - lastRefreshRef.current > REFRESH_COOLDOWN) {
        console.log("Refreshing consumed items");
        fetchConsumedItems(selectedDate, false, false);
        lastRefreshRef.current = now;
      } else {
        console.log("Skipping refresh due to recent update");
      }
    }, 300);
    
    // Clean up the debounced function on unmount or when dependencies change
    return () => {
      if (refreshConsumedItemsRef.current) {
        refreshConsumedItemsRef.current.cancel();
      }
    };
  }, [fetchConsumedItems, selectedDate]);
  
  // Create a stable function to call the debounced function
  const refreshConsumedItems = useCallback(() => {
    if (refreshConsumedItemsRef.current) {
      refreshConsumedItemsRef.current();
    }
  }, []);

  const fetchUserSettings = async () => {
    try {
      await configPromise;
      const data = await apiService.makeRequest("GET", "/settings");
      setUserSettings(data);
    } catch (error) {
      console.error("Error fetching user settings:", error);
    }
  };

  const handleItemSelect = async (item) => {
    try {
      const adjustedDate = new Date(
        selectedDate.getTime() - selectedDate.getTimezoneOffset() * 60000
      );
      const formattedDate = adjustedDate.toISOString().split("T")[0];
      const newConsumedItem = {
        barcode: item.barcode,
        consumed_quantity: 1,
        date: formattedDate,
      };
      resetRefreshCooldown();
      await apiService.makeRequest(
        "POST",
        "/consumedFoodItems",
        newConsumedItem
      );
      refreshConsumedItems();
    } catch (error) {
      console.error("Error adding consumed item:", error);
    }
  };

  const handleDateChange = (newDate) => {
    setSelectedDate(newDate);
  };

  const handleDeleteClick = useCallback(
    async (id) => {
      setLoading(true);
      try {
        resetRefreshCooldown();
        await apiService.makeRequest("DELETE", `/consumedFoodItems`, [], [id]);
        refreshConsumedItems();
      } catch (error) {
        console.error("Error deleting consumed food item:", error);
      } finally {
        setLoading(false);
      }
    },
    [refreshConsumedItems, resetRefreshCooldown]
  );

  const handleQuantityChange = useCallback(
    async (id, newQuantity) => {
      try {
        resetRefreshCooldown();
        await apiService.makeRequest(
          "PUT",
          `/consumedFoodItems`,
          {
            consumed_quantity: newQuantity,
          },
          [id]
        );
        refreshConsumedItems();
      } catch (error) {
        console.error("Error updating consumed quantity:", error);
      }
    },
    [refreshConsumedItems, resetRefreshCooldown]
  );
  const handleResetServingQuantity = useCallback(
    async (id, barcode) => {
      try {
        const response = await apiService.makeRequest(
          "GET",
          `/foodItems/servingQuantity`,
          [],
          [barcode]
        );
        const originalServingQuantity = response.servingQuantity;
        resetRefreshCooldown();
        await apiService.makeRequest(
          "PUT",
          `/consumedFoodItems`,
          { serving_quantity: originalServingQuantity },
          [id]
        );

        setConsumedItems((prevItems) =>
          prevItems.map((item) =>
            item.id === id
              ? {
                  ...item,
                  serving_quantity: originalServingQuantity,
                  ...calculateNutrition(item, originalServingQuantity),
                }
              : item
          )
        );
        setSnackbar({
          open: true,
          message: "Serving quantity reset successfully",
          severity: "success",
        });
        refreshConsumedItems();
      } catch (error) {
        console.error("Error resetting serving quantity:", error);
      }
    },
    [refreshConsumedItems, calculateNutrition, resetRefreshCooldown]
  );

  const processRowUpdate = useCallback(
    async (newRow, oldRow) => {
      if (newRow.serving_quantity === oldRow.serving_quantity) {
        return oldRow;
      }

      const formattedServingQuantity = formatNumericInput(
        newRow.serving_quantity
      );
      const roundedServingQuantity = formatForBackend(formattedServingQuantity);

      if (formattedServingQuantity === "") {
        setSnackbar({
          open: true,
          message: "Invalid serving quantity. Please enter a valid number.",
          severity: "error",
        });
        throw new Error("Invalid serving quantity");
      }

      try {
        resetRefreshCooldown();
        await apiService.makeRequest(
          "PUT",
          `/consumedFoodItems`,
          {
            serving_quantity: roundedServingQuantity,
          },
          [newRow.id]
        );

        const updatedRow = {
          ...newRow,
          serving_quantity: roundedServingQuantity,
          ...calculateNutrition(newRow, roundedServingQuantity),
        };

        setConsumedItems((prevItems) =>
          prevItems.map((item) =>
            item.id === updatedRow.id ? updatedRow : item
          )
        );
        refreshConsumedItems();
        return updatedRow;
      } catch (error) {
        console.error("Error updating serving quantity:", error);
        setSnackbar({
          children: "Error updating serving quantity.",
          severity: "error",
        });
        throw error;
      }
    },
    [refreshConsumedItems, calculateNutrition, resetRefreshCooldown]
  );

  const handleCloseSnackbar = (event, reason) => {
    if (reason === "clickaway") {
      return;
    }
    setSnackbar({ ...snackbar, open: false });
  };

  const fetchWeeklyData = useCallback(async () => {
    try {
      const endDate = new Date();
      const startDate = subDays(endDate, 6);
      const dates = [];
      for (
        let d = new Date(startDate);
        d <= endDate;
        d.setDate(d.getDate() + 1)
      ) {
        const adjustedDate = new Date(
          d.getTime() - d.getTimezoneOffset() * 60000
        );
        dates.push(adjustedDate.toISOString().split("T")[0]);
      }

      const weekData = await Promise.all(
        dates.map(async (date) => {
          try {
            const response = await apiService.makeRequest(
              "GET",
              `/consumedFoodItems`,
              [],
              [date]
            );

            // Process the response similar to fetchConsumedItems
            const responseArray = Array.isArray(response)
              ? response
              : Object.values(response || {});

            const totals = responseArray.reduce(
              (acc, item) => ({
                calories:
                  acc.calories +
                  parseFloat(
                    (item.calories_per_100g / 100) *
                      item.consumed_quantity *
                      item.serving_quantity || 0
                  ),
                protein:
                  acc.protein +
                  parseFloat(
                    (item.protein_per_100g / 100) *
                      item.consumed_quantity *
                      item.serving_quantity || 0
                  ),
                carbs:
                  acc.carbs +
                  parseFloat(
                    (item.carbs_per_100g / 100) *
                      item.consumed_quantity *
                      item.serving_quantity || 0
                  ),
                fat:
                  acc.fat +
                  parseFloat(
                    (item.fat_per_100g / 100) *
                      item.consumed_quantity *
                      item.serving_quantity || 0
                  ),
              }),
              { calories: 0, protein: 0, carbs: 0, fat: 0 }
            );

            const settings = {
              calories: userSettings.calories || 1,
              proteins: userSettings.proteins || 1,
              carbs: userSettings.carbs || 1,
              fat: userSettings.fat || 1,
            };

            return {
              date: format(new Date(date), "EEE, MMM d"),
              caloriesPercentage: (totals.calories / settings.calories) * 100,
              proteinPercentage: (totals.protein / settings.proteins) * 100,
              carbsPercentage: (totals.carbs / settings.carbs) * 100,
              fatPercentage: (totals.fat / settings.fat) * 100,
            };
          } catch (error) {
            console.error(`Error processing data for ${date}:`, error);
            return {
              date: format(new Date(date), "EEE, MMM d"),
              caloriesPercentage: 0,
              proteinPercentage: 0,
              carbsPercentage: 0,
              fatPercentage: 0,
            };
          }
        })
      );
      setWeeklyData(weekData);
    } catch (error) {
      console.error("Error in fetchWeeklyData:", error);
      setWeeklyData([]);
    }
  }, [userSettings]);
  const handleOpenChart = useCallback(() => {
    setOpenChart(true);
    fetchWeeklyData();
  }, [fetchWeeklyData]);

  const handleCloseChart = () => {
    setOpenChart(false);
  };

  const WeeklyChart = () => {
    const tooltipStyle = {
      backgroundColor: theme.palette.background.paper,
      border: `1px solid ${theme.palette.divider}`,
      borderRadius: theme.shape.borderRadius,
      boxShadow: theme.shadows[3],
      color: theme.palette.text.primary,
    };

    console.log("Weekly Chart Data:", weeklyData);

    if (!weeklyData || weeklyData.length === 0) {
      return <Typography>No data available for the selected week</Typography>;
    }

    return (
      <Box sx={{ width: "100%", height: 500, p: 2 }}>
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={weeklyData}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="date" />
            <YAxis
              domain={[0, 200]}
              ticks={[0, 25, 50, 75, 100, 125, 150, 175, 200]}
              label={{
                value: "% of Daily Goal",
                angle: -90,
                position: "insideLeft",
              }}
            />
            <RechartsTooltip
              formatter={(value, name) => [
                `${value.toFixed(1)}%`,
                name.replace("Percentage", ""),
              ]}
              labelFormatter={(label) => `Date: ${label}`}
              contentStyle={tooltipStyle}
              itemStyle={{ color: theme.palette.text.primary }}
              labelStyle={{ color: theme.palette.text.secondary }}
            />
            <Legend />
            <Bar dataKey="caloriesPercentage" fill="#8884d8" name="Calories" />
            <Bar dataKey="proteinPercentage" fill="#82ca9d" name="Protein" />
            <Bar dataKey="carbsPercentage" fill="#ffc658" name="Carbs" />
            <Bar dataKey="fatPercentage" fill="#ff7300" name="Fat" />
            <ReferenceLine y={100} stroke="red" strokeDasharray="3 3" />
          </BarChart>
        </ResponsiveContainer>
      </Box>
    );
  };
  useEffect(() => {
    fetchConsumedItems(selectedDate, false, false);
    fetchUserSettings();
  }, [fetchConsumedItems, selectedDate]);

  useEffect(() => {
    calculateTotalNutrition(consumedItems);
  }, [consumedItems, calculateTotalNutrition]);

  const handleSSEMessage = useCallback(
    (message) => {
      if (message === "consumed_food_items_updated") {
        console.log(
          `Updating consumed food items due to SSE message: ${message}`
        );
        fetchConsumedItems(selectedDate, false, false);
        lastRefreshRef.current = Date.now();
      }
    },
    [fetchConsumedItems, selectedDate]
  );

  useSSE(handleSSEMessage);

  const columns = [
    {
      field: "name",
      headerName: "Name",
      width: 200,
      headerAlign: "left",
      align: "left",
    },
    {
      field: "consumed_quantity",
      headerName: "Consumed Quantity",
      width: 150,
      headerAlign: "left",
      align: "center",
      renderCell: (params) => (
        <div>
          <IconButton
            onClick={() =>
              handleQuantityChange(params.row.id, Number(params.value) - 1)
            }
            disabled={Number(params.value) <= 1}
          >
            <RemoveIcon />
          </IconButton>
          {params.value}
          <IconButton
            onClick={() =>
              handleQuantityChange(params.row.id, Number(params.value) + 1)
            }
          >
            <AddIcon />
          </IconButton>
        </div>
      ),
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
          params.row.serving_quantity_unit || "g"
        }`,
    },
    {
      field: "calories",
      headerName: "Calories",
      width: 130,
      headerAlign: "left",
      align: "left",
      renderCell: (params) => `${formatNumberForDisplay(params.value)} kcal`,
    },
    {
      field: "fat",
      headerName: "Fat",
      width: 130,
      headerAlign: "left",
      align: "left",
      renderCell: (params) => `${formatNumberForDisplay(params.value)} g`,
    },
    {
      field: "carbs",
      headerName: "Carbs",
      width: 130,
      headerAlign: "left",
      align: "left",
      renderCell: (params) => `${formatNumberForDisplay(params.value)} g`,
    },
    {
      field: "protein",
      headerName: "Protein",
      width: 130,
      headerAlign: "left",
      align: "left",
      renderCell: (params) => `${formatNumberForDisplay(params.value)} g`,
    },
    {
      field: "barcode",
      headerName: "Barcode",
      width: 130,
      headerAlign: "left",
      align: "left",
    },
    {
      field: "insert_date",
      headerName: "Last Updated",
      width: 180,
      headerAlign: "left",
      align: "left",
      renderCell: (cellValues) => {
        if (cellValues.row && cellValues.row.insert_date) {
          const date = new Date(cellValues.row.insert_date);
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
      headerName: "Actions",
      width: 100,
      flex: 1,
      cellClassName: "actions",
      align: "right",
      headerAlign: "right",

      renderCell: (params) => (
        <>
          <Tooltip title="Reset serving quantity">
            <IconButton
              onClick={() =>
                handleResetServingQuantity(params.row.id, params.row.barcode)
              }
              color="inherit"
            >
              <RestoreIcon />
            </IconButton>
          </Tooltip>
          <Tooltip title="Delete this item">
            <IconButton
              onClick={() => handleDeleteClick(params.row.id)}
              color="inherit"
            >
              <DeleteIcon />
            </IconButton>
          </Tooltip>
        </>
      ),
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
      <Box
        display="flex"
        justifyContent="space-between"
        alignItems="center"
        mb={2}
      >
        <PersistentFoodItemSearch onItemSelect={handleItemSelect} />
      </Box>
      <Box
        display="flex"
        justifyContent="space-between"
        alignItems="center"
        mb={2}
        marginBottom="20px"
      >
        <Box
          display="flex"
          justifyContent="space-between"
          alignItems="center"
          mb={2}
        >
          <LocalizationProvider dateAdapter={AdapterDateFns}>
            <DatePicker
              label="Date"
              value={selectedDate}
              onChange={handleDateChange}
              sx={{ marginRight: "20px" }}
            />
          </LocalizationProvider>
        </Box>
        <Box>
          <Typography variant="body1">
            Calories: {totalNutrition.calories} / {userSettings.calories} kcal
          </Typography>
          <Typography variant="body2">
            Protein: {totalNutrition.protein} / {userSettings.proteins}g |
            Carbs: {totalNutrition.carbs} / {userSettings.carbs}g | Fat:{" "}
            {totalNutrition.fat} / {userSettings.fat}g
          </Typography>
        </Box>
        <Box>
          <Button
            onClick={handleOpenChart}
            variant="contained"
            style={{ height: "40px", marginRight: "10px" }}
          >
            Show Weekly Chart
          </Button>
          <Button
            onClick={() => fetchConsumedItems(selectedDate, true, true)}
            variant="contained"
            style={{ height: "40px" }}
          >
            Refresh
          </Button>
        </Box>
      </Box>
      <div
        style={{
          flexGrow: 1,
          width: "100%",
          minHeight: "163px",
        }}
      >
        <DataGrid
          rows={consumedItems}
          columns={columns}
          getRowId={(row) => row.id}
          autoPageSize={true}
          loading={loading}
          disableSelectionOnClick
          processRowUpdate={processRowUpdate}
          onProcessRowUpdateError={(error) => {
            console.error("Error updating row:", error);
          }}
        />
        <Dialog
          open={openChart}
          onClose={handleCloseChart}
          fullWidth
          maxWidth="lg"
        >
          <DialogTitle sx={{ textAlign: "center" }}>
            Weekly Nutrition Overview
          </DialogTitle>
          <DialogContent>
            <WeeklyChart />
          </DialogContent>
        </Dialog>
        <Snackbar
          open={snackbar.open}
          autoHideDuration={5000}
          onClose={handleCloseSnackbar}
        >
          <Alert onClose={handleCloseSnackbar} severity={snackbar.severity}>
            {snackbar.message}
          </Alert>
        </Snackbar>
      </div>
    </div>
  );
};

export default Home;
