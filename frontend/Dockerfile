# Builder runs on the native build host (amd64 in CI); output is static
# assets, arch-independent, so no cross-compile flags needed beyond pinning.
FROM --platform=$BUILDPLATFORM node:22-alpine@sha256:8ea2348b068a9544dae7317b4f3aafcdc032df1647bb7d768a05a5cad1a7683f AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
ARG VITE_TEST_MODE=false
ENV VITE_TEST_MODE=$VITE_TEST_MODE
# Vite only inlines env vars prefixed with VITE_ at build time. The previous
# ARG/ENV pair was named APP_VERSION (no prefix) → Vite ignored it → the
# bundle contained `import.meta.env.VITE_APP_VERSION = undefined` and the
# VersionBadge component fell back to "unknown" in every deployed image.
# Renamed to match the prefix Vite recognises.
ARG VITE_APP_VERSION=unknown
ENV VITE_APP_VERSION=$VITE_APP_VERSION
RUN npm run build

FROM nginx:alpine@sha256:5616878291a2eed594aee8db4dade5878cf7edcb475e59193904b198d9b830de
RUN mkdir -p /var/cache/nginx/client_temp /var/cache/nginx/proxy_temp \
             /var/cache/nginx/fastcgi_temp /var/cache/nginx/uwsgi_temp \
             /var/cache/nginx/scgi_temp && \
    chown -R nginx:nginx /var/cache/nginx /run /var/run
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 3000
USER nginx
