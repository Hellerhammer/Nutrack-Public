import React, { useState, useEffect } from 'react';
import {
    Button,
    TextField,
    Dialog,
    DialogTitle,
    DialogContent,
    DialogActions,
    Box,
    Alert,
    CircularProgress,
    Stack,
    Switch,
    FormControlLabel
} from '@mui/material';
import CloudUploadIcon from '@mui/icons-material/CloudUpload';
import CloudDownloadIcon from '@mui/icons-material/CloudDownload';
import dropboxService from '../services/dropboxService';
import { apiService } from "../services/apiService";

const Dropbox = () => {
    const [showCodeInput, setShowCodeInput] = useState(false);
    const [code, setCode] = useState('');
    const [error, setError] = useState('');
    const [isLoading, setIsLoading] = useState(false);
    const [operationStatus, setOperationStatus] = useState(null);
    const [autoSync, setAutoSync] = useState(false);
    const [isAuthenticated, setIsAuthenticated] = useState(() => {
        const savedAuth = localStorage.getItem('dropbox_auth_status');
        return savedAuth === 'true';
    });

    useEffect(() => {
        if (operationStatus && (operationStatus.type === 'success' || operationStatus.type === 'error')) {
            const timer = setTimeout(() => {
                setOperationStatus(null);
            }, 5000);
            return () => clearTimeout(timer);
        }
    }, [operationStatus]);

    useEffect(() => {
        localStorage.setItem('dropbox_auth_status', isAuthenticated);
    }, [isAuthenticated]);

    useEffect(() => {
        const checkAuth = async () => {
            try {
                const auth = await dropboxService.isAuthenticated();
                setIsAuthenticated(auth);
                if (auth) {
                    // Fetch auto-sync state if authenticated
                    const response = await apiService.makeRequest("GET", "/dropbox/autosync");
                    setAutoSync(response.enabled);
                }
            } catch (error) {
                console.error('Error checking authentication:', error);
            }
        };
        checkAuth();
    }, []);

    const handleLogin = async () => {
        try {
            setError('');
            setOperationStatus({ type: 'info', message: 'Initiating Dropbox login...' });
            await dropboxService.initiateLogin();
            setShowCodeInput(true);
            setOperationStatus({ type: 'info', message: 'Please enter the code from Dropbox' });
        } catch (error) {
            console.error('Error initiating Dropbox login:', error);
            setError('Error starting login process');
            setOperationStatus({ type: 'error', message: 'Failed to start login process' });
        }
    };

    const handleCodeSubmit = async (e) => {
        e.preventDefault();
        if (!code.trim()) {
            setError('Please enter the code');
            return;
        }

        setIsLoading(true);
        setError('');
        setOperationStatus({ type: 'info', message: 'Verifying code...' });

        try {
            await dropboxService.submitCode(code.trim());
            setShowCodeInput(false);
            setCode('');
            setIsAuthenticated(true);
            setOperationStatus({ type: 'success', message: 'Successfully connected to Dropbox' });
        } catch (error) {
            console.error('Error submitting code:', error);
            setError('Invalid code or connection error');
            setOperationStatus({ type: 'error', message: 'Failed to verify code' });
        } finally {
            setIsLoading(false);
        }
    };

    const handleLogout = async () => {
        try {
            setOperationStatus({ type: 'info', message: 'Disconnecting from Dropbox...' });
            await dropboxService.logout();
            setShowCodeInput(false);
            setCode('');
            setError('');
            setIsAuthenticated(false);
            setOperationStatus({ type: 'success', message: 'Successfully disconnected from Dropbox' });
        } catch (error) {
            console.error('Error during logout:', error);
            setError('Error during logout');
            setOperationStatus({ type: 'error', message: 'Failed to disconnect from Dropbox' });
        }
    };

    const handleAutoSyncChange = async (event) => {
        const newValue = event.target.checked;
        setAutoSync(newValue);
        try {
            await apiService.makeRequest("POST", "/dropbox/autosync", { enabled: newValue });
        } catch (error) {
            console.error("Failed to update auto-sync state:", error);
            setAutoSync(!newValue); // Revert on error
            setOperationStatus({
                type: 'error',
                message: 'Failed to update auto-sync setting'
            });
        }
    };

    const handleClose = () => {
        setShowCodeInput(false);
        setCode('');
        setError('');
    };

    const handleUploadDatabase = async () => {
        setOperationStatus({ type: 'info', message: 'Uploading database...' });
        setIsLoading(true);

        try {
            const response = await dropboxService.uploadDatabase();
            let message;
            console.log(response);
            switch (response.status) {
                case 'upToDate':
                    message = 'Database is already up to date';
                    break;
                case 'uploaded':
                    message = 'Database uploaded successfully';
                    break;
                case 'remoteNewer':
                    message = 'Remote database is newer';
                    break;
                default:
                    message = 'Unknown status';
            }
            setOperationStatus({ 
                type: 'success', 
                message: message
            });
        } catch (error) {
            console.error('Error uploading database:', error);
            setOperationStatus({ type: 'error', message: 'Error uploading database' });
        } finally {
            setIsLoading(false);
        }
    };

    const handleDownloadDatabase = async () => {
        setOperationStatus({ type: 'info', message: 'Downloading database...' });
        setIsLoading(true);

        try {
            const response = await dropboxService.downloadDatabase();
            let message;
            switch (response.status) {
                case 'upToDate':
                    message = 'Database is already up to date';
                    break;
                case 'downloaded':
                    message = 'Database downloaded successfully';
                    break;
                default:
                    message = 'Unknown status';
            }
            setOperationStatus({ 
                type: 'success', 
                message: message
            });
        } catch (error) {
            console.error('Error downloading database:', error);
            setOperationStatus({ type: 'error', message: 'Error downloading database' });
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, alignItems: 'center', width: '100%' }}>
            {error && (
                <Alert severity="error" onClose={() => setError('')} sx={{ width: '100%' }}>
                    {error}
                </Alert>
            )}

            {operationStatus && (
                <Alert severity={operationStatus.type} onClose={() => setOperationStatus(null)} sx={{ width: '100%' }}>
                    {operationStatus.message}
                </Alert>
            )}

            <Stack direction="column" spacing={2} sx={{ width: '100%' }}>
                <Button
                    variant="contained"
                    onClick={isAuthenticated ? handleLogout : handleLogin}
                    disabled={isLoading}
                    fullWidth
                >
                    {isAuthenticated ? 'Disconnect from Dropbox' : 'Connect with Dropbox'}
                </Button>

                {isAuthenticated && (
                    <Stack direction="column" spacing={2} sx={{ width: '100%' }}>
                        <Button
                            variant="contained"
                            startIcon={<CloudUploadIcon />}
                            onClick={handleUploadDatabase}
                            disabled={isLoading}
                            fullWidth
                        >
                            Upload Database
                        </Button>
                        <Button
                            variant="contained"
                            startIcon={<CloudDownloadIcon />}
                            onClick={handleDownloadDatabase}
                            disabled={isLoading}
                            fullWidth
                        >
                            Download Database
                        </Button>
                    </Stack>
                )}
            </Stack>

            {isAuthenticated && (
                    <FormControlLabel
                        control={
                            <Switch
                                checked={autoSync}
                                onChange={handleAutoSyncChange}
                                disabled={!isAuthenticated}
                            />
                        }
                        label="Auto-Sync mit Dropbox"
                        sx={{ mb: 2, display: 'block', width: '100%', justifyContent: 'center' }}
                    />
                )}

            <Dialog 
                open={showCodeInput && !isAuthenticated} 
                onClose={handleClose}
                maxWidth="sm"
                fullWidth
            >
                <DialogTitle>
                    Dropbox Authentication
                </DialogTitle>
                <DialogContent>
                    <Box component="form" onSubmit={handleCodeSubmit} sx={{ mt: 2 }}>
                        <TextField
                            autoFocus
                            fullWidth
                            label="Dropbox Code"
                            value={code}
                            onChange={(e) => setCode(e.target.value)}
                            disabled={isLoading}
                            placeholder="Enter code from Dropbox"
                            margin="normal"
                            error={!!error}
                            helperText={error || "Please enter the code you received from Dropbox"}
                        />
                    </Box>
                </DialogContent>
                <DialogActions sx={{ px: 3, pb: 3 }}>
                    <Button
                        onClick={handleClose}
                        disabled={isLoading}
                        color="inherit"
                    >
                        Cancel
                    </Button>
                    <Button
                        onClick={handleCodeSubmit}
                        disabled={isLoading || !code.trim()}
                        variant="contained"
                        startIcon={isLoading ? <CircularProgress size={20} /> : null}
                    >
                        Confirm Code
                    </Button>
                </DialogActions>
            </Dialog>
        </Box>
    );
};

export default Dropbox;
