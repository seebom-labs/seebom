# ---- Build stage ----
FROM node:22-alpine AS builder

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npx ng build --configuration=production

# ---- Serve stage ----
FROM nginx:1.27-alpine

# Remove default nginx config.
RUN rm /etc/nginx/conf.d/default.conf

# Add our custom nginx config.
COPY nginx.conf /etc/nginx/conf.d/seebom.conf

# Copy built Angular assets.
COPY --from=builder /app/dist/ui/browser /usr/share/nginx/html

# Create an empty custom-theme.css fallback (overridable via volume mount).
RUN touch /usr/share/nginx/html/custom-theme.css

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]

