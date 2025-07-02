import { generateCodeVerifier, generateCodeChallenge } from '../utils/pkce';
import { apiService } from "../services/apiService";
import { getSecret } from '../utils/secrets';

const DROPBOX_AUTH_URL = 'https://www.dropbox.com/oauth2/authorize';

class DropboxService {
    constructor() {
        this.codeVerifier = null;
        this.baseUrl = process.env.REACT_APP_API_URL;
    }

    async initiateLogin() {
        try {
            // Generate PKCE code verifier and challenge
            this.codeVerifier = generateCodeVerifier();
            const codeChallenge = generateCodeChallenge(this.codeVerifier);

            // Save code verifier to session storage for later use
            sessionStorage.setItem('dropbox_code_verifier', this.codeVerifier);

            // Get client ID from secrets
            const clientId = await getSecret('DROPBOX_CLIENT_ID');

            // Construct the authorization URL
            const params = new URLSearchParams({
                client_id: clientId,
                response_type: 'code',
                code_challenge: codeChallenge,
                code_challenge_method: 'S256',
                token_access_type: 'offline'
            });

            // Open Dropbox authorization page in a new window/tab
            window.open(`${DROPBOX_AUTH_URL}?${params.toString()}`, '_blank');
        } catch (error) {
            console.error('Error initiating Dropbox login:', error);
            throw error;
        }
    }

    async submitCode(code) {
        // Retrieve code verifier from session storage
        const codeVerifier = sessionStorage.getItem('dropbox_code_verifier');
        if (!codeVerifier) {
            throw new Error('No code verifier found in session storage');
        }

        try {
            // Exchange code for tokens
            const response = await apiService.makeHttpRequest('POST', '/dropbox/token', {
                code,
                codeVerifier,
            });

            return response;
        } catch (error) {
            console.error('Error exchanging code for tokens:', error);
            throw error;
        } finally {
            // Clean up session storage
            sessionStorage.removeItem('dropbox_code_verifier');
        }
    }

    async isAuthenticated() {
        try {
            const data = await apiService.makeHttpRequest('GET', '/dropbox/status');
            return data?.isAuthenticated ?? false;
        } catch (error) {
            console.error('Error checking authentication status:', error);
            return false;
        }
    }

    async logout() {
        try {
            await apiService.makeHttpRequest('POST', '/dropbox/logout');
        } catch (error) {
            console.error('Error during logout:', error);
            throw error;
        }
    }

    async uploadDatabase() {
        try {
            if (!(await this.isAuthenticated())) {
                console.log('Not authenticated with Dropbox');
                return;
            }
            const response = await apiService.makeHttpRequest('POST', '/dropbox/upload-database');
            return response;
        } catch (error) {
            console.error('Error uploading database:', error);
            throw error;
        }
    }

    async downloadDatabase() {
        try {
            if (!(await this.isAuthenticated())) {
                console.log('Not authenticated with Dropbox');
                return;
            }
            const response = await apiService.makeHttpRequest('GET', '/dropbox/download-database');
            return response;
        } catch (error) {
            console.error('Error downloading database:', error);
            throw error;
        }
    }

    async sync(force = false) {
        console.log('Syncing with Dropbox...');
        try {
            if (!(await this.isAuthenticated())) {
                console.log('Not authenticated with Dropbox');
                return;
            }
            const response = await apiService.makeHttpRequest('POST', '/dropbox/sync', {
                force: force
            });
            return response;
        } catch (error) {
            console.error('Error syncing with Dropbox:', error);
            throw error;
        }
    }
}

export default new DropboxService();
