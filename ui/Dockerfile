# syntax=docker/dockerfile:1
# Build stage
FROM node:25-alpine AS builder

WORKDIR /app

# Accept build arguments for Vite environment variables
ARG VITE_API_URL
ARG VITE_METRICS_UPDATE_INTERVAL_SECONDS

# Set as environment variables for build
ENV VITE_API_URL=$VITE_API_URL
ENV VITE_METRICS_UPDATE_INTERVAL_SECONDS=$VITE_METRICS_UPDATE_INTERVAL_SECONDS
ENV NPM_CONFIG_CACHE=/root/.cache/npm

# Copy package files
COPY package*.json ./

# Install dependencies (persist tarball cache across builds — requires BuildKit)
RUN --mount=type=cache,target=/root/.cache/npm \
    npm ci --prefer-offline --no-audit --no-fund

# Copy source code
COPY . .

# Build the application
RUN npm run build

# Runtime stage
FROM nginx:alpine

# Copy built files to nginx
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy nginx configuration
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Expose port
EXPOSE 80

# Start nginx
CMD ["nginx", "-g", "daemon off;"]
