// Separate from api.ts because authService needs it too, and api.ts imports AuthContext (a cycle).
// Empty means same-origin (nginx and the dev server proxy /api); an absolute VITE_API_URL must be
// listed in the API's CORS_ALLOWED_ORIGINS.
export const API_URL: string = (() => {
  const raw = import.meta.env.VITE_API_URL;
  if (raw === undefined || raw === null) return '';
  return String(raw).trim().replace(/\/$/, '');
})();
