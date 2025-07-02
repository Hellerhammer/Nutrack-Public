import SHA256 from 'crypto-js/sha256';
import Base64 from 'crypto-js/enc-base64';
import Utf8 from 'crypto-js/enc-utf8';

// Generate a random string for PKCE
export function generateCodeVerifier() {
    const array = new Uint8Array(32);
    for(let i = 0; i < array.length; i++) {
        array[i] = Math.floor(Math.random() * 256);
    }
    return base64URLEncode(array);
}

// Generate code challenge from verifier
export function generateCodeChallenge(verifier) {
    const wordArray = Utf8.parse(verifier);
    const hash = SHA256(wordArray);
    const base64 = Base64.stringify(hash);
    return base64
        .replace(/\+/g, '-')
        .replace(/\//g, '_')
        .replace(/=+$/, '');
}

// Base64URL encoding function
function base64URLEncode(buffer) {
    return btoa(String.fromCharCode(...new Uint8Array(buffer)))
        .replace(/\+/g, '-')
        .replace(/\//g, '_')
        .replace(/=+$/, '');
}
