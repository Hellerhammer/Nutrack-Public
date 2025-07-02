import React, { createContext, useContext, useState } from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
  Button,
  useTheme,
} from '@mui/material';

const DialogContext = createContext(undefined);

export const DialogProvider = ({ children }) => {
  const [dialogContent, setDialogContent] = useState(null);
  const [showSyncError, setShowSyncError] = useState(false);
  const [syncErrorCallbacks, setSyncErrorCallbacks] = useState(null);
  const theme = useTheme();

  const showDialog = (content) => {
    setDialogContent(content);
  };

  const hideDialog = () => {
    setDialogContent(null);
  };

  const showSyncErrorDialog = (onUploadMine, onDownloadRemote) => {
    setSyncErrorCallbacks({ onUploadMine, onDownloadRemote });
    setShowSyncError(true);
  };

  const hideSyncErrorDialog = () => {
    setShowSyncError(false);
    setSyncErrorCallbacks(null);
  };

  return (
    <DialogContext.Provider value={{ showDialog, hideDialog, showSyncErrorDialog }}>
      {children}
      
      <Dialog 
        open={dialogContent !== null} 
        onClose={hideDialog}
        maxWidth="sm"
        fullWidth
        PaperProps={{
          elevation: theme.shadows[8],
          sx: { borderRadius: theme.shape.borderRadius }
        }}
      >
        {dialogContent}
      </Dialog>

      <Dialog
        open={showSyncError}
        onClose={hideSyncErrorDialog}
        maxWidth="sm"
        fullWidth
        PaperProps={{
          elevation: theme.shadows[8],
          sx: { borderRadius: theme.shape.borderRadius }
        }}
      >
        <DialogTitle sx={{ color: theme.palette.text.primary }}>
          Sync Error
        </DialogTitle>
        <DialogContent sx={{ pb: theme.spacing(2) }}>
          <DialogContentText sx={{ color: theme.palette.text.secondary }}>
            There is a file conflict between your local data and the Dropbox backup.
          </DialogContentText>
        </DialogContent>
        <DialogActions sx={{ px: theme.spacing(3), pb: theme.spacing(2) }}>
          <Button 
            onClick={hideSyncErrorDialog} 
            sx={{ color: theme.palette.text.secondary }}
          >
            Cancel
          </Button>
          <Button
            onClick={() => {
              syncErrorCallbacks?.onUploadMine();
              hideSyncErrorDialog();
            }}
            color="primary"
          >
            Upload Mine
          </Button>
          <Button
            onClick={() => {
              syncErrorCallbacks?.onDownloadRemote();
              hideSyncErrorDialog();
            }}
            color="primary"
            variant="contained"
            sx={{ 
              boxShadow: theme.shadows[2],
              '&:hover': {
                boxShadow: theme.shadows[4]
              }
            }}
          >
            Download Remote
          </Button>
        </DialogActions>
      </Dialog>
    </DialogContext.Provider>
  );
};

export const useDialog = () => {
  const context = useContext(DialogContext);
  if (!context) {
    throw new Error('useDialog must be used within a DialogProvider');
  }
  return context;
};
