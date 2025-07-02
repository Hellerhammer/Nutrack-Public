import React, { useEffect } from 'react';
import dropboxService from '../services/dropboxService';

const DropboxCallback = () => {
    useEffect(() => {
        const params = new URLSearchParams(window.location.hash.substring(1));
        const accessToken = params.get('access_token');
        const error = params.get('error');

        if (window.opener) {
            if (accessToken) {
                window.opener.postMessage({
                    type: 'DROPBOX_AUTH',
                    accessToken
                }, window.location.origin);
            } else if (error) {
                window.opener.postMessage({
                    type: 'DROPBOX_AUTH',
                    error
                }, window.location.origin);
            }
            window.close();
        }
    }, []);

    return (
        <div>
            <p>Authenticating with Dropbox...</p>
        </div>
    );
};

export default DropboxCallback;
