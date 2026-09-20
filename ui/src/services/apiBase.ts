/**
 * Where the API lives, resolved once for every caller.
 *
 * This sits in its own module rather than in `api.ts` because the auth service
 * needs it too, and `api.ts` reaches back into AuthContext for the access
 * token — importing it from there would close an import cycle.
 *
 * Empty string = same-origin, which is how the panel is actually served: nginx
 * fronts the SPA and proxies /api to core, and the dev server proxies the same
 * prefix. Point VITE_API_URL at an absolute URL only to talk to a core that is
 * somewhere else — that origin then has to be in CORS_ALLOWED_ORIGINS, or the
 * browser rejects the panel's own requests.
 */
export const API_URL: string = (() => {
  const raw = import.meta.env.VITE_API_URL;
  if (raw === undefined || raw === null) return '';
  return String(raw).trim().replace(/\/$/, '');
})();
