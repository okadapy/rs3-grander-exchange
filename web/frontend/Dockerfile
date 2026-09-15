FROM node:22-alpine AS build
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY combined.yaml /app/combined.yaml
COPY frontend/ ./
# Vite inlines VITE_* variables into the bundle at build time, so the gateway
# address has to be an image build argument. Without it every image shipped
# with client.ts's localhost:8080 fallback baked in and was undeployable.
ARG VITE_API_URL=http://localhost:8080
ENV VITE_API_URL=$VITE_API_URL
RUN npm run build

FROM nginx:1.27-alpine
ARG VITE_API_URL=http://localhost:8080
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
# The CSP must name the exact origin the bundle talks to, over both the HTTP
# and the WebSocket scheme, so it is derived from the same build argument.
RUN set -eu; \
    api_origin="${VITE_API_URL%/}"; \
    ws_origin="ws${api_origin#http}"; \
    sed -i "s|__API_ORIGIN__|${api_origin} ${ws_origin}|" /etc/nginx/conf.d/default.conf
COPY --from=build /app/frontend/dist /usr/share/nginx/html
