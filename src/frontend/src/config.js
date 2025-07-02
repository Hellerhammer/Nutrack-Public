let BACKEND_URL = "localhost:8080";
let USE_ELECTRON_IPC = false;
let SELECTED_PROFILE_ID = null;
let configLoaded = false;

const loadConfig = async () => {
  console.log("loading config..");

  if (window.electron) {
    try {
      const config = await window.electron.loadConfig();
      USE_ELECTRON_IPC = config.USE_ELECTRON_IPC === "1";
      console.log("Using Electron IPC:", USE_ELECTRON_IPC);
      configLoaded = true;
    } catch (error) {
      console.error("Error loading config in Electron:", error);
      configLoaded = true;
    }
    return;
  }

  return fetch("/config.json")
    .then((response) => response.json())
    .then((config) => {
      BACKEND_URL = "http://" + config.BACKEND_URL + "/api";
      USE_ELECTRON_IPC = config.USE_ELECTRON_IPC === "1";
      console.log("Loaded Backend URL:", BACKEND_URL);
      console.log("Using Electron IPC:", USE_ELECTRON_IPC);
      configLoaded = true;
    })
    .catch((error) => {
      console.error("Error loading config:", error);
      configLoaded = true;
    });
};

const changeSelectedProfile = (profileId) => {
  SELECTED_PROFILE_ID = profileId;
  console.log("Changed selected profile to:", profileId);
};

const configPromise = loadConfig();

export {
  BACKEND_URL,
  USE_ELECTRON_IPC,
  configPromise,
  configLoaded,
  SELECTED_PROFILE_ID,
  changeSelectedProfile as updateProfileId,
};
