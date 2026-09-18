import axios from "axios";

// The backend serves on :2117 and authenticates with an http-only session
// cookie, so every request has to carry credentials.
export const API_BASE_URL =
  import.meta.env.VITE_API_URL?.replace(/\/$/, "") || "http://localhost:2117";

const TOKEN_KEY = "ccs_session_token";

// The cookie alone is not dependable here: the frontend and the API are on
// different sites, which makes the session cookie a third-party cookie that
// Edge, Safari, Firefox and every incognito mode drop. So the OAuth callback
// also hands the token back in the URL fragment, we keep a copy, and send it as
// a bearer token. The cookie still works wherever the browser allows it.
export function storeSessionToken(token) {
  try {
    if (token) localStorage.setItem(TOKEN_KEY, token);
  } catch {
    // Private mode or blocked storage: fall back to the cookie.
  }
}

export function getSessionToken() {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function clearSessionToken() {
  try {
    localStorage.removeItem(TOKEN_KEY);
  } catch {
    // Nothing to clear.
  }
}

const api = axios.create({
  baseURL: API_BASE_URL,
  withCredentials: true,
});

api.interceptors.request.use((config) => {
  const token = getSessionToken();
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

export default api;
