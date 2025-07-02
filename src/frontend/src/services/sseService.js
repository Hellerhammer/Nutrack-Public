import { configPromise, BACKEND_URL, USE_ELECTRON_IPC } from "../config";

class SSEService {
  constructor() {
    this.eventSource = null;
    this.messageHandlers = new Set();
    this.isConnected = false;
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
    this.reconnectDelay = 1000;
  }

  async connect() {
    await configPromise;
    if (USE_ELECTRON_IPC) {
      this.connectStandardIO();
    } else {
      await this.connectSSE();
    }
  }

  async connectSSE() {
    try {
      if (this.eventSource) {
        this.eventSource.close();
      }

      this.eventSource = new EventSource(`${BACKEND_URL}/sse`, {
        withCredentials: true,
      });

      this.eventSource.onopen = () => {
        console.log("SSE connection established");
        this.isConnected = true;
        this.reconnectAttempts = 0;
      };

      this.eventSource.onmessage = (event) => {
        this.handleMessage(event.data);
      };

      this.eventSource.onerror = (error) => {
        console.error("SSE connection error:", error);
        this.isConnected = false;
        this.handleReconnect();
      };
    } catch (error) {
      console.error("Error connecting to SSE:", error);
      this.handleReconnect();
    }
  }

  connectStandardIO() {
    if (window.electron) {
      // Store the cleanup function
      this.cleanupListener = window.electron.receive(
        "sse-message",
        (message) => {
          this.handleMessage(message);
        }
      );
      this.isConnected = true;
    } else {
      console.error("StandardIO mode enabled but electron is not available");
    }
  }

  disconnect() {
    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
    }
    // Clean up the electron listener if it exists
    if (this.cleanupListener) {
      this.cleanupListener();
      this.cleanupListener = null;
    }
    this.isConnected = false;
    this.messageHandlers.clear();
  }

  handleReconnect() {
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts++;
      console.log(
        `Attempting to reconnect (${this.reconnectAttempts}/${this.maxReconnectAttempts})...`
      );
      setTimeout(
        () => this.connect(),
        this.reconnectDelay * this.reconnectAttempts
      );
    } else {
      console.error("Max reconnection attempts reached");
    }
  }

  handleMessage(data) {
    let message;
    try {
      if (typeof data === "string") {
        message = data;
      } else if (data.type === "sse") {
        message = data.data;
      } else {
        message = data;
      }

      this.messageHandlers.forEach((handler) => {
        try {
          handler(message);
        } catch (error) {
          console.error("Error in SSE message handler:", error);
        }
      });
    } catch (error) {
      console.error("Error parsing SSE message:", error);
    }
  }

  subscribe(handler) {
    this.messageHandlers.add(handler);
    return () => this.unsubscribe(handler);
  }

  unsubscribe(handler) {
    this.messageHandlers.delete(handler);
  }
}

export const sseService = new SSEService();
