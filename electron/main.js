const { app, BrowserWindow, protocol, ipcMain } = require("electron");
const path = require("path");
const isDev = require("electron-is-dev");
const { spawn } = require("child_process");

let mainWindow;
let backendProcess;

app.setPath("userData", path.join(app.getPath("appData"), "Nutrack"));
app.setPath(
  "sessionData",
  path.join(app.getPath("userData"), "Session Storage")
);
app.setPath("userCache", path.join(app.getPath("userData"), "Cache"));

ipcMain.handle("backend-request", async (event, request) => {
  try {
    if (!backendProcess) {
      throw new Error("Backend process not running");
    }

    console.log("Sending request to backend:", request);
    backendProcess.stdin.write(JSON.stringify(request) + "\n");

    const response = await new Promise((resolve, reject) => {
      const handler = (data) => {
        try {
          const lines = data.toString().split("\n");
          for (const line of lines) {
            if (!line.trim()) continue;

            try {
              const parsedResponse = JSON.parse(line);
              console.log("Parsed response:", parsedResponse);

              if (
                parsedResponse.type === "response" &&
                parsedResponse.requestId === request.requestId
              ) {
                backendProcess.stdout.removeListener("data", handler);
                resolve(parsedResponse);
                return;
              }
            } catch (parseError) {
              if (isDev) {
                console.log("Debug output:", line);
              }
            }
          }
        } catch (error) {
          console.error("Error processing backend response:", error);
          resolve(error);
        }
      };

      backendProcess.stdout.on("data", handler);

      // Timeout after 30 seconds
      setTimeout(() => {
        backendProcess.stdout.removeListener("data", handler);
        reject(new Error("Backend request timed out"));
      }, 30000);
    });

    return response;
  } catch (error) {
    console.error("Error handling backend request:", error);
    throw error;
  }
});

function createWindow() {
  // Disable default file protocol handler
  protocol.interceptFileProtocol("file", (request, callback) => {
    let url = request.url.substr(8); // Strip "file:///" from start
    url = decodeURIComponent(url); // Handle URL encoding

    // Normalize backslashes to forward slashes
    url = url.replace(/\\/g, "/");

    console.log("Attempting to load:", url);

    try {
      // Check if the file exists
      if (require("fs").existsSync(url)) {
        callback({ path: url });
      } else {
        console.error("File not found:", url);
        callback({ error: -6 }); // ERR_FILE_NOT_FOUND
      }
    } catch (error) {
      console.error("Error handling file request:", error);
      callback({ error: -6 });
    }
  });

  mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    webPreferences: {
      nodeIntegration: true,
      contextIsolation: true,
      preload: path.join(__dirname, "preload.js"),
      javascript: true,
      webSecurity: !isDev,
    },
  });

  // Use an absolute path with forward slashes
  const indexPath = path
    .join(__dirname, "build", "index.html")
    .replace(/\\/g, "/");

  const fileUrl = `file:///${indexPath}`;

  console.log({
    indexPath,
    fileUrl,
    exists: require("fs").existsSync(indexPath),
    stats: require("fs").statSync(indexPath),
    dirContents: require("fs").readdirSync(path.dirname(indexPath)),
    __dirname,
    cwd: process.cwd(),
  });

  // Try to read the file directly
  const indexContent = require("fs").readFileSync(indexPath, "utf8");
  console.log("Index.html content length:", indexContent.length);

  mainWindow
    .loadURL(fileUrl)
    .then(() => {
      console.log("Frontend loaded successfully");
    })
    .catch((err) => {
      console.error("Error loading frontend:", err);
      // Fallback: Try to load the content directly
      mainWindow.loadURL(
        `data:text/html;charset=utf-8,${encodeURIComponent(indexContent)}`
      );
    });

  if (isDev) {
    mainWindow.webContents.openDevTools();
  }

  startBackend();
}

app.whenReady().then(async () => {
  await protocol.registerSchemesAsPrivileged([
    {
      scheme: "file",
      privileges: { secure: true, standard: true, supportFetchAPI: true },
    },
  ]);
  createWindow();
});
function startBackend() {
  // Determine the correct path to the backend
  let backendPath;
  if (isDev) {
    backendPath = path.join(__dirname, "../src/backend/main/backend");
  } else {
    backendPath = path.join(process.resourcesPath, "backend");
  }
  if (process.platform === "win32") {
    backendPath += ".exe";
  }

  // Create data directory in AppData
  const dataDir = path.join(app.getPath("userData"), "data");
  if (!require("fs").existsSync(dataDir)) {
    require("fs").mkdirSync(dataDir, { recursive: true });
  }

  console.log("Starting backend from:", backendPath);
  console.log("Data directory:", dataDir);

  try {
    backendProcess = spawn(backendPath, [], {
      env: {
        ...process.env,
        USE_ELECTRON_IPC: "1",
        ELECTRON_APP: "1",
        DATA_DIR: dataDir,
      },
      stdio: ["pipe", "pipe", "pipe"], // Enable stdin
    });

    backendProcess.stdout.on("data", (data) => {
      try {
        const lines = data.toString().split("\n");

        for (const line of lines) {
          if (!line.trim()) continue;

          try {
            const output = JSON.parse(line);
            if (output.type === "response") {
              console.log("Backend response:", output);
            } else if (output.type === "sse-message") {
              console.log("Broadcasting SSE message:", output);
              mainWindow?.webContents.send("sse-message", output.data);
            }
          } catch (error) {
            // Log non-JSON outputs only in development mode
            if (isDev) {
              console.log(`Backend stdout: ${data}`);
            }
          }
        }
      } catch (error) {
        console.error("Error processing backend output:", error);
      }
    });

    backendProcess.stderr.on("data", (data) => {
      console.error(`Backend stderr: ${data}`);
    });

    backendProcess.on("error", (err) => {
      console.error("Failed to start backend:", err);
    });

    backendProcess.on("close", (code) => {
      console.log(`Backend process exited with code ${code}`);
    });
  } catch (error) {
    console.error("Error starting backend:", error);
  }
}

app.whenReady().then(() => {
  createWindow();

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") {
    app.quit();
  }
  if (backendProcess) {
    backendProcess.kill();
  }
});

app.on("before-quit", () => {
  if (backendProcess) {
    backendProcess.kill();
  }
});

// Error handling
process.on("uncaughtException", (error) => {
  console.error("Uncaught Exception:", error);
});

process.on("unhandledRejection", (error) => {
  console.error("Unhandled Rejection:", error);
});
