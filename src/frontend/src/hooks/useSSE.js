import { useEffect, useCallback } from "react";
import { sseService } from "../services/sseService";

export function useSSE(messageHandler) {
  const handleMessage = useCallback(
    (message) => {
      if (messageHandler) {
        messageHandler(message);
      }
    },
    [messageHandler]
  );

  useEffect(() => {
    // Connect to SSE when component mounts
    sseService.connect();

    // Subscribe to messages
    const unsubscribe = sseService.subscribe(handleMessage);

    // Cleanup on unmount
    return () => {
      unsubscribe();
      // Only disconnect if no other subscribers
      if (sseService.messageHandlers.size === 0) {
        sseService.disconnect();
      }
    };
  }, [handleMessage]);

  return sseService.isConnected;
}
