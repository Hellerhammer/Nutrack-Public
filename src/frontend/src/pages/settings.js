import React, { useState, useEffect } from "react";
import {
  TextField,
  Button,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  InputAdornment,
  Snackbar,
  Grid2,
  Typography,
  Slider,
  Box,
  IconButton,
  Tooltip,
  FormControlLabel,
  Switch,
} from "@mui/material";
import { AdapterDateFns } from "@mui/x-date-pickers/AdapterDateFns";
import { LocalizationProvider } from "@mui/x-date-pickers/LocalizationProvider";
import { DatePicker } from "@mui/x-date-pickers/DatePicker";
import MuiAlert from "@mui/material/Alert";
import {
  calculateNutrition,
  calculateNutrientsFromCaloriesAndWeight,
} from "../utils/nutritioncalculator";
import Brightness4Icon from "@mui/icons-material/Brightness4";
import Brightness7Icon from "@mui/icons-material/Brightness7";
import SettingsBrightnessIcon from "@mui/icons-material/SettingsBrightness";
import { configPromise } from "../config";
import {
  formatNumberForDisplay,
  formatForBackend,
  formatNumericInput,
} from "../utils/formatter";
import { apiService } from "../services/apiService";
import dropboxService from '../services/dropboxService';
import Dropbox from '../components/Dropbox';
import ProfileSelector from '../components/ProfileSelector';
import BarcodeScanner from '../components/BarcodeScanner';

const Alert = React.forwardRef(function Alert(props, ref) {
  return <MuiAlert elevation={6} ref={ref} variant="filled" {...props} />;
});

const Settings = ({ toggleTheme, themeMode }) => {
  const [weight, setWeight] = useState(0);
  const [height, setHeight] = useState(0);
  const [birthDate, setBirthDate] = useState(null);
  const [gender, setGender] = useState("");
  const [activityLevel, setActivityLevel] = useState(0);
  const [weeklyWeightChange, setWeeklyWeightChange] = useState(0);
  const [calories, setCalories] = useState(0);
  const [proteins, setProteins] = useState(0);
  const [carbs, setCarbs] = useState(0);
  const [fat, setFat] = useState(0);
  const [snackbar, setSnackbar] = useState({
    open: false,
    message: "",
    severity: "info",
  });
  const [isDropboxAuthenticated, setIsDropboxAuthenticated] = useState(false);
  const [weightTracking, setWeightTracking] = useState(false);
  const [autoRecalculateNutritionValues, setAutoRecalculateNutritionValues] = useState(false);

  const handleInputChange = (setter) => (event) => {
    const { value } = event.target;
    if (value === "") {
      setter("");
    } else {
      const formattedValue = formatNumericInput(value);
      if (formattedValue !== "") {
        setter(formattedValue);
        setSnackbar({
          open: false,
        });
      } else {
        setSnackbar({
          open: true,
          message: "Please enter a valid number.",
          severity: "error",
        });
      }
    }
  };

  useEffect(() => {
    fetchSettings();
    fetchWeightTrackingState();

    // Add event listener for profile changes
    const handleProfileChange = () => {
      fetchSettings();
      fetchWeightTrackingState();
    };

    window.addEventListener('profileChanged', handleProfileChange);

    // Cleanup
    return () => {
      window.removeEventListener('profileChanged', handleProfileChange);
    };
  }, []);

  useEffect(() => {
    const checkDropboxAuth = async () => {
      const isAuthenticated = await dropboxService.isAuthenticated();
      setIsDropboxAuthenticated(isAuthenticated);
    };
    checkDropboxAuth();
  }, [setIsDropboxAuthenticated]);

  const handleThemeToggle = () => {
    console.log("Toggle theme button clicked");
    toggleTheme();
  };

  const getThemeIcon = () => {
    switch (themeMode) {
      case "light":
        return (
          <Tooltip title="Switch to Dark Mode">
            <Brightness7Icon />
          </Tooltip>
        );
      case "dark":
        return (
          <Tooltip title="Switch to System Mode">
            <Brightness4Icon />
          </Tooltip>
        );
      case "system":
      default:
        return (
          <Tooltip title="Switch to Light Mode">
            <SettingsBrightnessIcon />
          </Tooltip>
        );
    }
  };

  const fetchWeightTrackingState = async () => {
    try {
      const response = await apiService.makeRequest("GET", "/settings/weighttracking");
      setWeightTracking(response.enabled);
    } catch (error) {
      console.error("Failed to fetch weight tracking state:", error);
      setSnackbar({
        open: true,
        message: "Failed to fetch weight tracking settings",
        severity: "error",
      });
    }
  };

  const handleWeightTrackingChange = async (event) => {
    const newValue = event.target.checked;
    setWeightTracking(newValue);
    try {
      await apiService.makeRequest("POST", "/settings/weighttracking", { enabled: newValue });
      setSnackbar({
        open: true,
        message: "Weight tracking setting updated",
        severity: "success",
      });
    } catch (error) {
      console.error("Failed to update weight tracking state:", error);
      setWeightTracking(!newValue); // Revert on error
      setSnackbar({
        open: true,
        message: "Failed to update weight tracking setting",
        severity: "error",
      });
    }
  };

  const fetchSettings = async () => {
    try {
      await configPromise;
      const response = await apiService.makeRequest("GET", "/settings");
      const settings = response;
      setWeight(formatNumberForDisplay(settings.weight) || "");
      setHeight(formatNumberForDisplay(settings.height) || "");
      setCalories(formatNumberForDisplay(settings.calories) || "");
      setProteins(formatNumberForDisplay(settings.proteins) || "");
      setCarbs(formatNumberForDisplay(settings.carbs) || "");
      setFat(formatNumberForDisplay(settings.fat) || "");
      setBirthDate(settings.birth_date ? new Date(settings.birth_date) : null);
      setGender(settings.gender || "");
      setActivityLevel(settings.activity_level || 0);
      setWeeklyWeightChange(
        settings.weekly_weight_change !== null && settings.weekly_weight_change !== undefined
          ? parseFloat(settings.weekly_weight_change)
          : 0
      );
    } catch (error) {
      console.error("Error fetching settings:", error);
    }
  };

  const handleCalculateFromAll = async () => {
    try {
      if (!weight || !height || !birthDate || !gender) {
        setSnackbar({
          open: true,
          message: "Please fill in all required fields.",
          severity: "error",
        });
        return;
      }
      
      // Calculate age from birth date
      const today = new Date();
      const birthDateObj = new Date(birthDate);
      let calculatedAge = today.getFullYear() - birthDateObj.getFullYear();
      const monthDiff = today.getMonth() - birthDateObj.getMonth();
      if (monthDiff < 0 || (monthDiff === 0 && today.getDate() < birthDateObj.getDate())) {
        calculatedAge--;
      }

      const result = await calculateNutrition(
        parseFloat(weight),
        parseFloat(height),
        calculatedAge,
        gender,
        parseInt(activityLevel),
        parseFloat(weeklyWeightChange)
      );

      setCalories(formatNumberForDisplay(result.calories));
      setProteins(formatNumberForDisplay(result.proteins));
      setCarbs(formatNumberForDisplay(result.carbs));
      setFat(formatNumberForDisplay(result.fat));
      setAutoRecalculateNutritionValues(true);
    } catch (error) {
      console.error("Error calculating nutrition:", error);
    }
  };

  const handleCalculateFromCalories = async () => {
    if (calories && weight) {
      const result = await calculateNutrientsFromCaloriesAndWeight(
        parseFloat(calories),
        parseFloat(weight)
      );

      console.log("Result: ", result);

      setProteins(formatNumberForDisplay(result.proteins));
      setCarbs(formatNumberForDisplay(result.carbs));
      setFat(formatNumberForDisplay(result.fat));
    } else {
      setSnackbar({
        open: true,
        message: "Please enter both calories and weight before calculating.",
        severity: "warning",
      });
    }
  };

  const handleSubmit = (event) => {
    event.preventDefault();
    if (!weight || !height || !birthDate || !gender) {
      setSnackbar({
        open: true,
        message: "Please fill in all required fields.",
        severity: "error",
      });
      return;
    }
    
    const submitSettings = async () => {
      try {
        await apiService.makeRequest("POST", "/settings/auto-recalculate-nutrition-values", {
          enabled: autoRecalculateNutritionValues,
        });
        await apiService.makeRequest("POST", "/settings", {
          weight: formatForBackend(weight),
          height: formatForBackend(height),
          calories: formatForBackend(calories),
          proteins: formatForBackend(proteins),
          carbs: formatForBackend(carbs),
          fat: formatForBackend(fat),
          gender: gender,
          activity_level: Number(activityLevel),
          weekly_weight_change: formatForBackend(weeklyWeightChange),
          birth_date: birthDate ? birthDate.toISOString().split('T')[0] : null,
        });
        setSnackbar({
          open: true,
          message: "Settings saved successfully!",
          severity: "success",
        });
      } catch (error) {
        console.error("Error saving settings:", error);
        setSnackbar({
          open: true,
          message: "Error saving settings. Please try again.",
          severity: "error",
        });
      }
    };
    
    submitSettings();
  };

  const handleCloseSnackbar = (event, reason) => {
    if (reason === "clickaway") {
      return;
    }
    setSnackbar({ ...snackbar, open: false });
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', width: '100%', maxWidth: '800px', margin: '0 auto' }}>
      <Grid2 container spacing={2} direction="column" alignItems="center" style={{ width: '100%', marginBottom: '20px' }}>
        <Grid2 xs={12} style={{ width: '80%' }}>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', width: '100%', marginBottom: '20px' }}>
            <ProfileSelector />
            <IconButton onClick={handleThemeToggle} color="inherit">
              {getThemeIcon()}
            </IconButton>
          </Box>
        </Grid2>
      </Grid2>

      <form onSubmit={handleSubmit} style={{ width: '100%' }}>
        <Grid2 container spacing={2} direction="column" alignItems="center">
          <Grid2 xs={12} style={{ width: '100%' }}>
            <Box
              sx={{
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
                width: "100%",
              }}
            >
              <Typography gutterBottom align="center">
                Weekly Weight Change (kg)
              </Typography>
              <Slider
                value={weeklyWeightChange}
                onChange={(e, newValue) => {
                  setWeeklyWeightChange(newValue);
                }}
                aria-labelledby="weekly-weight-change-slider"
                valueLabelDisplay="auto"
                step={0.01}
                marks={[
                  { value: -1, label: "-1 kg" },
                  { value: 0, label: "0 kg" },
                  { value: 1, label: "+1 kg" },
                ]}
                min={-1}
                max={1}
                sx={{ width: "80%" }}
              />
            </Box>
            <Typography variant="body2" color="textSecondary" align="center">
              {weeklyWeightChange < 0
                ? `Weight loss: ${Math.abs(weeklyWeightChange)} kg/week`
                : weeklyWeightChange > 0
                ? `Weight gain: ${weeklyWeightChange} kg/week`
                : "Maintain weight"}
            </Typography>
          </Grid2>

          <Grid2 xs={12} style={{ width: '80%' }}>
            <TextField
              label="Weight"
              value={weight}
              onChange={(e) => {
                setWeight(e.target.value);
              }}
              fullWidth
              margin="normal"
              InputProps={{
                endAdornment: <InputAdornment position="end">kg</InputAdornment>,
              }}
            />
          </Grid2>

          <Grid2 xs={12} style={{ width: '80%' }}>
            <TextField
              label="Height"
              value={height}
              onChange={(e) => {
                setHeight(e.target.value);
              }}
              fullWidth
              margin="normal"
              InputProps={{
                endAdornment: <InputAdornment position="end">cm</InputAdornment>,
              }}
            />
          </Grid2>

          <Grid2 xs={12} style={{ width: '80%' }}>
            <LocalizationProvider dateAdapter={AdapterDateFns}>
              <DatePicker
                label="Birth Date"
                value={birthDate}
                onChange={(newDate) => setBirthDate(newDate)}
                slotProps={{ textField: { fullWidth: true, margin: "normal" } }}
              />
            </LocalizationProvider>
          </Grid2>

          <Grid2 xs={12} style={{ width: '80%' }}>
            <FormControl fullWidth margin="normal">
              <InputLabel id="gender-label">Gender</InputLabel>
              <Select
                labelId="gender-label"
                value={gender}
                onChange={(e) => {
                  setGender(e.target.value);
                }}
                label="Gender"
              >
                <MenuItem value="male">Male</MenuItem>
                <MenuItem value="female">Female</MenuItem>
                <MenuItem value="undefined">Undefined</MenuItem>
              </Select>
            </FormControl>
          </Grid2>

          <Grid2 xs={12} style={{ width: '80%' }}>
            <FormControl fullWidth margin="normal">
              <InputLabel id="activity-level-label">Activity Level</InputLabel>
              <Select
                labelId="activity-level-label"
                value={activityLevel}
                onChange={(e) => {
                  setActivityLevel(e.target.value);
                }}
                label="Activity Level"
              >
                <MenuItem value={0}>Little or no exercise</MenuItem>
                <MenuItem value={1}>Light activity</MenuItem>
                <MenuItem value={2}>Moderate activity</MenuItem>
                <MenuItem value={3}>High activity</MenuItem>
                <MenuItem value={4}>Very high activity</MenuItem>
              </Select>
            </FormControl>
          </Grid2>

          <Grid2 xs={12} style={{ width: '80%' }}>
            <TextField
              label="Calories"
              value={calories}
              onChange={(e) => {
                setCalories(e.target.value);
                setAutoRecalculateNutritionValues(false);
              }}
              fullWidth
              margin="normal"
              InputProps={{
                endAdornment: <InputAdornment position="end">kcal</InputAdornment>,
              }}
            />
          </Grid2>

          <Grid2 xs={12} style={{ width: '80%' }}>
            <TextField
              label="Proteins"
              value={proteins}
              onChange={(e) => {
                setProteins(e.target.value);
                setAutoRecalculateNutritionValues(false);
              }}
              fullWidth
              margin="normal"
              InputProps={{
                endAdornment: <InputAdornment position="end">g</InputAdornment>,
              }}
            />
          </Grid2>

          <Grid2 xs={12} style={{ width: '80%' }}>
            <TextField
              label="Carbs"
              value={carbs}
              onChange={(e) => {
                setCarbs(e.target.value);
                setAutoRecalculateNutritionValues(false);
              }}
              fullWidth
              margin="normal"
              InputProps={{
                endAdornment: <InputAdornment position="end">g</InputAdornment>,
              }}
            />
          </Grid2>

          <Grid2 xs={12} style={{ width: '80%' }}>
            <TextField
              label="Fat"
              value={fat}
              onChange={(e) => {
                setFat(e.target.value);
                setAutoRecalculateNutritionValues(false);
              }}
              fullWidth
              margin="normal"
              InputProps={{
                endAdornment: <InputAdornment position="end">g</InputAdornment>,
              }}
            />
          </Grid2>
        </Grid2>
      </form>

      <Box sx={{ width: '80%', mt: 4, mb: 2, display: 'flex', flexDirection: 'column', gap: 2 }}>
        <Button
          variant="contained"
          color="primary"
          onClick={handleCalculateFromAll}
          fullWidth
        >
          Calculate Nutrients from All Inputs
        </Button>
        <Button
          variant="contained"
          color="primary"
          onClick={handleCalculateFromCalories}
          fullWidth
        >
          Calculate from Calories & Weight
        </Button>
        <Button
          type="submit"
          variant="contained"
          color="primary"
          onClick={handleSubmit}
          fullWidth
        >
          Save Settings
        </Button>
        
        <FormControlLabel
          control={
            <Switch
              checked={weightTracking}
              onChange={handleWeightTrackingChange}
            />
          }
          label="Enable Weight Tracking"
          sx={{ mt: 2, display: 'block', width: '100%', justifyContent: 'center' }}
        />
      </Box>

      <Box sx={{ width: '80%', mt: 4, mb: 2 }}>
        <Typography variant="h6" gutterBottom align="center">
          Dropbox Integration
        </Typography>
        <Dropbox />
      </Box>

      <BarcodeScanner />

      <Box sx={{ width: '80%', mt: 4, mb: 2 }}>
        <Typography variant="h6" gutterBottom align="center">
          Other
        </Typography>
        <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1 }}>
          <Typography variant="body1">
            <Typography component="a" href="https://nutrack-app.github.io/" target="_blank" rel="noopener noreferrer" sx={{ color: 'primary.main' }}>
              Website
            </Typography>
          </Typography>
          <Typography variant="body1">
            Report bugs to: <Typography component="a" href="mailto:kachonkdev@gmail.com" sx={{ color: 'primary.main' }} display="inline">
              kachonkdev@gmail.com
            </Typography>
          </Typography>
        </Box>
      </Box>
        

      <Snackbar
        open={snackbar.open}
        autoHideDuration={5000}
        onClose={handleCloseSnackbar}
      >
        <Alert
          onClose={handleCloseSnackbar}
          severity={snackbar.severity}
          sx={{ width: "100%" }}
        >
          {snackbar.message}
        </Alert>
      </Snackbar>
    </div>
  );
};

export default Settings;
