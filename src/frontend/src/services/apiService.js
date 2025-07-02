import {
  BACKEND_URL,
  configPromise,
  USE_ELECTRON_IPC,
  SELECTED_PROFILE_ID,
} from "../config";

class ApiService {
  async makeRequest(method, endpoint, data = null, urlParams = []) {
    await configPromise;
    if (USE_ELECTRON_IPC) {
      return this.makeStandardIORequest(method, endpoint, data, urlParams);
    } else {
      // Special handling for search endpoints
      if (endpoint.includes("search") && urlParams.length > 0) {
        const queryString = urlParams.join("&");
        // Remove possible "?" at the end of the endpoint
        const cleanEndpoint = endpoint.endsWith("?") ? endpoint.slice(0, -1) : endpoint;
        let fullEndpoint = `${cleanEndpoint}${
          cleanEndpoint.includes("?") ? "&" : "?"
        }${queryString}`;

        // // Add profile_id for non-profile endpoints
        // // Use SELECTED_PROFILE_ID only as a fallback if it is already set
        // if (!endpoint.includes("/profiles")) {
        //   if (method === "GET") {
        //     // GET requests automatically use the profile_id from the backend
        //     // based on the active profile, so no need to add it
        //   } else if (SELECTED_PROFILE_ID) {
        //     // Add profile_id for non-profile endpoints
        //     // Use SELECTED_PROFILE_ID only as a fallback if it is already set
        //     data = { ...data, profile_id: SELECTED_PROFILE_ID };
        //   }
        // }

        return this.makeHttpRequest(method, fullEndpoint, data);
      } else {
        // Normal handling for other endpoints
        let fullEndpoint =
          urlParams.length > 0
            ? `${endpoint}/${urlParams.join("/")}`
            : endpoint;

        // // Add profile_id for non-profile endpoints
        // // Use SELECTED_PROFILE_ID only as a fallback if it is already set
        // if (!endpoint.includes("/profiles")) {
        //   if (method === "GET") {
        //     // GET requests automatically use the profile_id from the backend
        //     // based on the active profile, so no need to add it
        //   } else if (SELECTED_PROFILE_ID) {
        //     // Add profile_id for non-profile endpoints
        //     // Use SELECTED_PROFILE_ID only as a fallback if it is already set
        //     data = { ...data, profile_id: SELECTED_PROFILE_ID };
        //   }
        // }
        return this.makeHttpRequest(method, fullEndpoint, data);
      }
    }
  }

  async makeHttpRequest(method, endpoint, data) {
    const options = {
      method,
      headers: {
        "Content-Type": "application/json",
      },
    };

    if (data && !["GET", "HEAD"].includes(method.toUpperCase())) {
      options.body = JSON.stringify(data);
    }

    const response = await fetch(`${BACKEND_URL}${endpoint}`, options);
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }
    
    // Check if there's any content to parse
    const contentType = response.headers.get("content-type");
    if (contentType && contentType.includes("application/json")) {
      return response.json();
    }
    return null;
  }

  async makeStandardIORequest(method, endpoint, data, urlParams) {
    if (window.electron) {
      try {
        const requestId = Math.random().toString(36).substring(7);
        console.debug(`Making request ${requestId}:`, {
          method,
          endpoint,
          data,
          urlParams,
        });

        // Send the request and wait for the response
        const response = await window.electron.invoke("backend-request", {
          method,
          endpoint,
          data: {
            ...data,
            urlParams,
          },
          type: "request",
          requestId,
        });

        console.debug(`Received response for ${requestId}:`, response);

        if (response.error) {
          throw new Error(response.error);
        }

        return response.data;
      } catch (error) {
        console.error("Error in StandardIO request:", error);
        throw error;
      }
    } else {
      throw new Error("Electron IPC is not available");
    }
  }

  // Method to get the active profile
  async getActiveProfile() {
    try {
      const response = await this.makeRequest("GET", "/profiles/active");
      return response && response.active ? response.profile_id : null;
    } catch (error) {
      console.error("Error fetching active profile:", error);
      return null;
    }
  }

  // Method to set the active profile
  async setActiveProfile(profileId) {
    try {
      await this.makeRequest("POST", "/profiles/active", {
        profile_id: profileId
      });
      return true;
    } catch (error) {
      console.error("Error setting active profile:", error);
      return false;
    }
  }
  
}

export const apiService = new ApiService();
