import React, { useState } from "react";
import { Outlet } from "react-router-dom";
import Box from "@mui/material/Box";
import Tab from "@mui/material/Tab";
import TabContext from "@mui/lab/TabContext";
import TabList from "@mui/lab/TabList";
import TabPanel from "@mui/lab/TabPanel";
import Home from "../pages/home";
import SavedFoodItems from "../pages/persistentfooditems";
import Settings from "../pages/settings";

const Layout = ({ toggleTheme, themeMode }) => {
  const [value, setValue] = useState("home");

  const handleChange = (event, newValue) => {
    setValue(newValue);
  };

  return (
    <>
      <nav>
        <TabContext value={value}>
          <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
            <Box sx={{ display: 'flex', alignItems: 'center' }}>
              <TabList
                onChange={handleChange}
                aria-label="lab API tabs example"
                variant="fullWidth"
                sx={{ flex: 1, '& .MuiTabs-flexContainer': { justifyContent: 'space-evenly' } }}
              >
                <Tab label="Home" value="home" sx={{ flex: 1 }} />
                <Tab label="Food Items" value="savedfooditems" sx={{ flex: 1 }} />
                <Tab label="Settings" value="settings" sx={{ flex: 1 }} />
              </TabList>
            </Box>
          </Box>
          <TabPanel value="home">
            <Home />
          </TabPanel>
          <TabPanel value="savedfooditems">
            <SavedFoodItems />
          </TabPanel>
          <TabPanel value="settings">
            <Settings toggleTheme={toggleTheme} themeMode={themeMode} />
          </TabPanel>
        </TabContext>
      </nav>

      <Outlet />
    </>
  );
};

export default Layout;
