const target = process.env.BUILD_TARGET; // 'electron' or 'docker'
const shared = path.join(__dirname, "../src");
const targetDir = path.join(__dirname, `../${target}`);

//copy shared code to target directory
copyDir(shared, targetDir);

// Set target-specific configuration
if (target === "electron") {
  // Electron-specific configuration
  process.env.USE_ELECTRON_IPC = "1";
} else {
  // Docker-specific configuration
  process.env.USE_ELECTRON_IPC = "0";
}
