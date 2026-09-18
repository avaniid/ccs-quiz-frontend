import axios from "axios";

// The backend serves on :2117 and authenticates with an http-only session
// cookie, so every request has to carry credentials.
export const API_BASE_URL =
  import.meta.env.VITE_API_URL?.replace(/\/$/, "") || "http://localhost:2117";

const api = axios.create({
  baseURL: API_BASE_URL,
  withCredentials: true,
});

export default api;
