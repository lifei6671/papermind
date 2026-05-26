FROM node:24-alpine AS build

WORKDIR /src/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

FROM nginx:1.27-alpine

COPY deployments/docker-compose/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/web/dist /usr/share/nginx/html

EXPOSE 80
