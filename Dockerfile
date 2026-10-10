FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.24-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server
# distroless has no shell, so create the media directory here, owned by the nonroot user.
RUN mkdir -p /out/data/media

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=backend /out/server /app/server
COPY --from=web /web/dist /app/web
COPY --from=backend --chown=65532:65532 /out/data /data
ENV PORT=8080 WEB_DIST_DIR=/app/web MEDIA_DIR=/data/media
EXPOSE 8080
ENTRYPOINT ["/app/server"]
