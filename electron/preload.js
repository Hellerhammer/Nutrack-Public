const { contextBridge, ipcRenderer } = require("electron");
const path = require("path");
const fs = require("fs");

contextBridge.exposeInMainWorld("electron", {
  loadConfig: () => {
    return new Promise((resolve, reject) => {
      // Try to load the config from the resources directory first
      let configPath = path.join(process.resourcesPath, "config.json");

      // In development mode, use the local config
      if (process.env.NODE_ENV !== "production") {
        configPath = path.join(__dirname, "config.json");
      }

      console.log("Loading config from:", configPath);

      // Fallback to build/resources/config.json if the others don't exist
      if (!fs.existsSync(configPath)) {
        configPath = path.join(__dirname, "build", "resources", "config.json");
      }

      fs.readFile(configPath, "utf8", (err, data) => {
        if (err) {
          console.error("Error reading config file:", err);
          // Fallback Konfiguration
          resolve({
            BACKEND_URL: "http://localhost",
            USE_ELECTRON_IPC: "1",
          });
          return;
        }
        try {
          const config = JSON.parse(data);
          console.log("Config loaded successfully:", config);
          resolve(config);
        } catch (error) {
          console.error("Error parsing config JSON:", error);
          reject(error);
        }
      });
    });
  },
  invoke: (channel, data) => ipcRenderer.invoke(channel, data),
  on: (channel, func) => {
    ipcRenderer.on(channel, (event, ...args) => func(...args));
  },
  removeListener: (channel, func) => {
    ipcRenderer.removeListener(channel, func);
  },
  receive: (channel, callback) => {
    const subscription = (event, ...args) => callback(...args);
    ipcRenderer.on(channel, subscription);
    // Return cleanup function
    return () => {
      ipcRenderer.removeListener(channel, subscription);
    };
  },
});

// Logging for IPC communication
ipcRenderer.on("error", (event, error) => {
  console.error("IPC Error:", error);
});
