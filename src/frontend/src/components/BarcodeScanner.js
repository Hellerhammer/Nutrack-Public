import React, { useState, useEffect } from 'react';
import {
  Box,
  Typography,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  Alert,
} from '@mui/material';
import { apiService } from '../services/apiService';

const BarcodeScanner = () => {
  const [scanners, setScanners] = useState([]);
  const [selectedScanner, setSelectedScanner] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    loadScanners();
  }, []);

  const loadScanners = async () => {
    try {
      const data = await apiService.makeRequest('GET', '/scanners');
      console.log('Received scanners:', data);
      setScanners(data);
      
      // Find and set active scanner if one exists
      const activeScanner = data.find(scanner => scanner.is_active);
      if (activeScanner) {
        setSelectedScanner(activeScanner);
      }
      
      setError(null);
    } catch (err) {
      console.error('Error loading scanners:', err);
      setError('Error loading scanners: ' + err.message);
    }
  };

  const handleSetActiveScanner = async (scanner) => {
    try {
      console.log('Setting active scanner:', scanner);
      await apiService.makeRequest('POST', '/scanners/active', { path: scanner.path });
      setError(null);
    } catch (err) {
      console.error('Error setting active scanner:', err);
      setError('Error setting active scanner: ' + err.message);
      setSelectedScanner(null);
    }
  };
  
  const handleScannerChange = (event) => {
    const scannerId = event.target.value;
    console.log('Scanner selection changed to:', scannerId);
    if (scannerId === "") {
      setSelectedScanner(null);
      // Deactivate scanner
      apiService.makeRequest('PUT', '/scanners/active/null')
        .then(() => {
          setError(null);
        })
        .catch(err => {
          console.error('Error deactivating scanner:', err);
          setError('Error deactivating scanner: ' + err.message);
        });
    } else {
      const scanner = scanners.find(s => 
        s.vendor_id + ':' + s.product_id === scannerId
      );
      console.log('Found scanner:', scanner);
      setSelectedScanner(scanner);
      handleSetActiveScanner(scanner);
    }
  };

  return (
    <Box sx={{ width: '100%', maxWidth: 720, mx: 'auto', p: 2 }}>
      <Typography variant="h6" gutterBottom>
        Barcode Scanner
      </Typography>
      
      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      <FormControl fullWidth sx={{ mb: 2 }}>
        <InputLabel>Select Scanner</InputLabel>
        <Select
          value={selectedScanner ? selectedScanner.vendor_id + ':' + selectedScanner.product_id : ""}
          onChange={handleScannerChange}
          label="Select Scanner"
        >
          <MenuItem value="">
            <em>No Scanner active</em>
          </MenuItem>
          {scanners.map((scanner) => (
            <MenuItem 
              key={scanner.vendor_id + ':' + scanner.product_id} 
              value={scanner.vendor_id + ':' + scanner.product_id}
            >
              {scanner.name}
            </MenuItem>
          ))}
        </Select>
      </FormControl>

    </Box>
  );
};

export default BarcodeScanner;
