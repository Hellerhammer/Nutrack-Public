let runtimeConfig = null;

const loadRuntimeConfig = async () => {
    if (!runtimeConfig) {
        try {
            const response = await fetch('/config.json');
            if (response.ok) {
                runtimeConfig = await response.json();
            } else {
                console.error('Failed to load runtime configuration');
                runtimeConfig = {};
            }
        } catch (error) {
            console.error('Error loading runtime configuration:', error);
            runtimeConfig = {};
        }
    }
    return runtimeConfig;
};

export const getSecret = async (key) => {
    // Load runtime configuration if not already loaded
    const config = await loadRuntimeConfig();
    
    // Check if the key exists in runtime config
    if (config && config[key]) {
        return config[key];
    }

    // Fallback to environment variables
    const envKey = `REACT_APP_${key}`;
    if (process.env[envKey]) {
        return process.env[envKey];
    }

    console.error(`Secret ${key} not found in runtime configuration or environment variables`);
    throw new Error(`Secret ${key} not found`);
};
