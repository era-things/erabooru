import { browser } from '$app/environment';

// Get the API base URL dynamically
export const apiBase = browser 
    ? `${window.location.protocol}//${window.location.host}/api`
    : 'http://localhost:8080/api'; // fallback