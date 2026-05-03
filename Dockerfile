# Stage 1: Vue build
FROM node:22-alpine AS frontend
RUN apk add --no-cache git
RUN git clone https://github.com/geschke/schrevind-ui.git /ui
WORKDIR /ui
RUN npm ci && npm run build

# Stage 2: Go build
FROM golang:1.26-alpine AS backend
WORKDIR /app
COPY . .
COPY --from=frontend /ui/dist ./web/dist
RUN go build -o schrevind .

# Stage 3: Final
FROM alpine:latest
COPY --from=backend /app/schrevind /schrevind
EXPOSE 8080
CMD ["/schrevind", "serve"]