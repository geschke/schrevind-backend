# Stage 1: Vue build
FROM node:22-alpine AS frontend
ARG FRONTEND_HASH=dev
RUN apk add --no-cache git
# Cache-bust: forces rebuild when FRONTEND_HASH changes
RUN echo "Frontend hash: $FRONTEND_HASH"
RUN git clone https://github.com/geschke/schrevind-ui.git /ui
WORKDIR /ui
RUN npm ci && npm run build

# Stage 2: Go build
FROM golang:1.26-alpine AS backend
ARG BACKEND_HASH=dev_docker
ARG FRONTEND_HASH=dev_docker
WORKDIR /app
COPY . .
COPY --from=frontend /ui/dist ./web/dist
# write version.json into web/dist/ to embed 
RUN printf '{"backend":"%s","frontend":"%s"}' \
    "$BACKEND_HASH" "$FRONTEND_HASH" \
    > ./web/dist/version.json
RUN go build -o schrevind .

# Stage 3: Final
FROM alpine:latest
COPY --from=backend /app/schrevind /schrevind
EXPOSE 8080
CMD ["/schrevind", "serve"]