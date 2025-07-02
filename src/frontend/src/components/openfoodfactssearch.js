import React, { useState, useCallback, useMemo } from "react";
import {
  TextField,
  List,
  ListItem,
  ListItemText,
  CircularProgress,
  Typography,
  ClickAwayListener,
  Paper,
  InputAdornment,
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import KeyboardReturnIcon from '@mui/icons-material/KeyboardReturn';
import axios from "axios";
import debounce from "lodash/debounce";
import { apiService } from "../services/apiService";

const OpenFoodFactsSearch = ({ onItemSelect }) => {
  const [searchTerm, setSearchTerm] = useState("");
  const [searchResults, setSearchResults] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [isFocused, setIsFocused] = useState(false);
  const theme = useTheme();

  const handleSearch = useCallback(async (term) => {
    if (term.length < 3) {
      setSearchResults([]);
      setError(term.length > 0 ? "Please enter at least 3 characters" : null);
      return;
    }

    setLoading(true);
    setError(null);
    try {
      const response = await apiService.makeRequest(
        "GET",
        `/search`,
        [],
        [`q=${encodeURIComponent(term)}`]
      );

      if (!response) {
        setSearchResults([]);
        return;
      }

      let products = [];
      if (response.products) {
        products = response.products;
      } else if (response.product) {
        products = [response.product];
      }

      // Filter out products without essential information
      products = products.filter(product => 
        product.product_name && 
        product.code &&
        product.nutriments
      );

      setSearchResults(products);
      setError(null);
    } catch (err) {
      console.error("Search error:", err);
      setSearchResults([]);
      setError(err.response?.data?.error || "An error occurred while searching");
    } finally {
      setLoading(false);
    }
  }, []);

  const handleKeyPress = (event) => {
    if (event.key === 'Enter') {
      handleSearch(searchTerm);
    }
  };

  const handleInputChange = (event) => {
    setSearchTerm(event.target.value);
    setError(null);
    if (event.target.value.length === 0) {
      setSearchResults([]);
    }
  };

  return (
    <ClickAwayListener onClickAway={() => setIsFocused(false)}>
      <div style={{ position: "relative", width: "100%", marginBottom: "36px" }}>
        <TextField
          label="Search for a product or barcode (press Enter to search)"
          variant="outlined"
          value={searchTerm}
          onChange={handleInputChange}
          onKeyPress={handleKeyPress}
          onFocus={() => setIsFocused(true)}
          fullWidth
          margin="normal"
          disabled={loading}
          InputProps={{
            endAdornment: (
              <InputAdornment position="end">
                <KeyboardReturnIcon color="action" />
              </InputAdornment>
            ),
          }}
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
            {loading && (
              <CircularProgress
                sx={{ marginLeft: "20px", marginTop: "20px" }}
              />
            )}
            {error && (
              <Typography color="error" sx={{ padding: "10px" }}>
                {error}
              </Typography>
            )}
            {!loading && !error && searchResults.length === 0 && searchTerm.length >= 3 && (
              <Typography sx={{ padding: "10px" }}>
                No products found
              </Typography>
            )}
            <List>
              {searchResults.map((product) => (
                <ListItem
                  key={product.code}
                  component="div"
                  onClick={() => {
                    console.log(product);
                    onItemSelect(product);
                    setIsFocused(false);
                    setSearchTerm("");
                  }}
                  sx={{
                    cursor: "pointer",
                    "&:hover": {
                      backgroundColor: "rgba(0, 0, 0, 0.04)", //slightly grayed out when hovering
                    },
                  }}
                >
                  <ListItemText
                    primary={product.product_name || "No product name"}
                    secondary={`Barcode: ${product.code}`}
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

export default OpenFoodFactsSearch;
