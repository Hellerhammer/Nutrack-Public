import React, { useState, useEffect } from "react";
import { ThemeProvider } from "@mui/material/styles";
import CssBaseline from "@mui/material/CssBaseline";
import { lightTheme, darkTheme } from "./theme";
import Layout from "./components/layout";
import useMediaQuery from "@mui/material/useMediaQuery";
import { BrowserRouter as Router, Routes, Route } from "react-router-dom";
import DropboxCallback from "./pages/DropboxCallback";
import { DialogProvider } from './contexts/DialogContext';
import { DropboxSyncHandler } from './components/DropboxSyncHandler';

function App() {
  const prefersDarkMode = useMediaQuery("(prefers-color-scheme: dark)");
  const [themeMode, setThemeMode] = useState(() => {
    const savedMode = localStorage.getItem("themeMode");
    return savedMode ? JSON.parse(savedMode) : "system";
  });

  const effectiveTheme =
    themeMode === "system" ? (prefersDarkMode ? "dark" : "light") : themeMode;

  useEffect(() => {
    localStorage.setItem("themeMode", JSON.stringify(themeMode));
  }, [themeMode]);

  const toggleTheme = () => {
    setThemeMode((prevMode) => {
      if (prevMode === "light") return "dark";
      if (prevMode === "dark") return "system";
      return "light";
    });
  };

  return (
    <ThemeProvider theme={effectiveTheme === "dark" ? darkTheme : lightTheme}>
      <CssBaseline />
      <DialogProvider>
        <DropboxSyncHandler />
        <Router>
          <Routes>
            <Route path="/auth/callback" element={<DropboxCallback />} />
            <Route path="*" element={<Layout toggleTheme={toggleTheme} themeMode={themeMode} />} />
          </Routes>
        </Router>
      </DialogProvider>
    </ThemeProvider>
  );
}

export default App;
