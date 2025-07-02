import React, { useState, useCallback } from "react";
import {
  TextField,
  List,
  ListItem,
  ListItemText,
  ClickAwayListener,
  Paper,
  Typography,
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import { configPromise } from "../config";
import { apiService } from "../services/apiService";
import debounce from 'lodash/debounce';

const PersistentFoodItemSearch = ({ onItemSelect }) => {
  const [searchTerm, setSearchTerm] = useState("");
  const [searchResults, setSearchResults] = useState([]);
  const [allFoodItems, setAllFoodItems] = useState([]);
  const [isFocused, setIsFocused] = useState(false);
  const [error, setError] = useState(null);
  const [hasSearched, setHasSearched] = useState(false);
  const theme = useTheme();

  const handleFocus = () => {
    setIsFocused(true);
    if (!searchTerm) {
      fetchAllFoodItems();
    }
  };

  const handleBlur = () => {
    setTimeout(() => {
      setIsFocused(false);
    }, 200);
  };

  const fetchAllFoodItems = async () => {
    try {
      await configPromise;
      const response = await apiService.makeRequest("GET", "/foodItems/all");

      const itemsArray = Array.isArray(response)
        ? response
        : Object.values(response);
      const validItems = itemsArray.filter((item) => item.barcode);

      setAllFoodItems(validItems);
      setSearchResults(validItems);
    } catch (error) {
      console.error("Error fetching all food items:", error);
    }
  };

  const debouncedSearch = useCallback(
    debounce(async (term) => {
      if (term.length === 0) {
        setSearchResults(allFoodItems);
        setError(null);
        setHasSearched(false);
        return;
      }

      if (term.length < 2) {
        setSearchResults([]);
        setError("Please enter at least 2 characters");
        setHasSearched(false);
        return;
      }

      setHasSearched(true);
      try {
        const response = await apiService.makeRequest(
          "GET",
          `/foodItems/search`,
          [],
          [`q=${encodeURIComponent(term)}`]
        );

        if (!response) {
          setSearchResults([]);
          return;
        }

        const data = Array.isArray(response)
          ? response
          : typeof response === "object" && response !== null
          ? Object.values(response)
          : [];

        setSearchResults(data);
        setError(null);
      } catch (error) {
        console.error("Error searching food items:", error);
        setSearchResults([]);
        setError(error.response?.data?.error || "Error searching food items");
      }
    }, 300),
    [allFoodItems]
  );

  const handleInputChange = (e) => {
    const newTerm = e.target.value;
    setSearchTerm(newTerm);
    setError(null);
    if (newTerm.length === 0) {
      setSearchResults(allFoodItems);
    } else {
      debouncedSearch(newTerm);
    }
  };

  const handleItemClick = (item) => {
    onItemSelect(item);
    setSearchTerm("");
    setIsFocused(false);
  };

  return (
    <ClickAwayListener onClickAway={() => setIsFocused(false)}>
      <div
        style={{ position: "relative", width: "100%", marginBottom: "4px" }}
      >
        <TextField
          label="Search for a food item"
          variant="outlined"
          value={searchTerm}
          onChange={handleInputChange}
          onFocus={handleFocus}
          onBlur={handleBlur}
          fullWidth
          margin="normal"
          disabled={false}
        />
        {isFocused && (
          <Paper
            elevation={3}
            sx={{
              position: "absolute",
              top: "100%",
              left: 0,
              right: 0,
              zIndex: 1,
              maxHeight: "300px",
              overflowY: "auto",
              backgroundColor: theme.palette.background.paper,
            }}
          >
            {error && (
              <Typography color="error" sx={{ padding: "10px" }}>
                {error}
              </Typography>
            )}
            {!error && hasSearched && searchResults.length === 0 && (
              <Typography sx={{ padding: "10px" }}>
                No products found
              </Typography>
            )}
            <List>
              {searchResults.map((item) => (
                <ListItem
                  key={item.barcode}
                  onClick={() => handleItemClick(item)}
                  sx={{
                    cursor: "pointer",
                    "&:hover": {
                      backgroundColor: "rgba(0, 0, 0, 0.04)",
                    },
                  }}
                >
                  <ListItemText
                    primary={item.name}
                    secondary={`Barcode: ${item.barcode}`}
                  />
                </ListItem>
              ))}
            </List>
          </Paper>
        )}
      </div>
    </ClickAwayListener>
  );
};

export default PersistentFoodItemSearch;
