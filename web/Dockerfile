FROM docker.io/library/node:24.11.1-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY index.html vite.config.mjs ./
COPY public ./public
COPY src ./src
RUN npm test && npm run build

FROM docker.io/library/nginx:1.29.2-alpine
ENV NGINX_ENTRYPOINT_LOCAL_RESOLVERS=1
COPY nginx.conf /etc/nginx/templates/default.conf.template
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
