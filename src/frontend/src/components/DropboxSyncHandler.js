import React, { useCallback } from 'react';
import { useSSE } from '../hooks/useSSE';
import { useDialog } from '../contexts/DialogContext';
import { apiService } from '../services/apiService';

export function DropboxSyncHandler() {
  const { showSyncErrorDialog } = useDialog();

  const handleSSEMessage = useCallback((message) => {
    if (message === "SHOW_SYNC_CONFLICT") {
      console.log("Showing sync conflict dialog due to SSE message");
      showSyncErrorDialog(
        () => {
          apiService.makeHttpRequest('POST', '/dropbox/upload-database');
        },
        () => {
          apiService.makeHttpRequest('GET', '/dropbox/download-database');
        }
      );
    }
  }, [showSyncErrorDialog]);

  useSSE(handleSSEMessage);

  return null;
}
