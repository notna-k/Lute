# syntax=docker/dockerfile:1
#
# Admin panel — decoupled SPA served by nginx, proxying /api to the core
# container. Build context is the REPO ROOT (see infrastructure/dev/docker-compose.yml)
# so this stage can also pull in the shared nginx config.

# ---- Build the SPA ----
FROM node:25-alpine AS builder

ARG VITE_METRICS_UPDATE_INTERVAL_SECONDS=5

# Empty VITE_API_URL => same-origin: the SPA calls /api on its own host and
# nginx proxies to core. Keeps the panel free of a hard-coded backend URL.
ENV VITE_METRICS_UPDATE_INTERVAL_SECONDS=$VITE_METRICS_UPDATE_INTERVAL_SECONDS \
    VITE_API_URL= \
    NPM_CONFIG_CACHE=/root/.cache/npm

WORKDIR /app
COPY ui/package.json ui/package-lock.json ./
RUN --mount=type=cache,target=/root/.cache/npm \
    npm ci --prefer-offline --no-audit --no-fund
COPY ui .
RUN npm run build

# ---- Serve with nginx ----
FROM nginx:1.27-alpine

COPY infrastructure/dev/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
