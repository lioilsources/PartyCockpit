# PartyCockpit: frontend build → Go build → distroless static image.
FROM node:22-alpine AS web
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

FROM golang:1.26-alpine AS go
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=web /src/frontend/dist/ ./internal/web/dist/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /partycockpit ./cmd/partycockpit

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go /partycockpit /partycockpit
ENV PC_ADDR=:8098 PC_DATA_DIR=/data
VOLUME /data
EXPOSE 8098
ENTRYPOINT ["/partycockpit"]
