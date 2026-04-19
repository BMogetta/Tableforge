# Builder runs on the native build host (amd64 in CI); output is static
# assets, arch-independent, so no cross-compile flags needed beyond pinning.
FROM --platform=$BUILDPLATFORM node:20-alpine@sha256:afdf98210b07b586eb71fa22ba2e432e058e4cd1304d31ed60888755b8c865fb AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
ARG VITE_TEST_MODE=false
ENV VITE_TEST_MODE=$VITE_TEST_MODE
ARG APP_VERSION=unknown
ENV APP_VERSION=$APP_VERSION
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
