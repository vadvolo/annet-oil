# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./
COPY tsconfig.json ./

# Install dependencies without running prepare script
RUN npm ci --ignore-scripts

# Copy source code
COPY src/ ./src/

# Build the application
RUN npm run build

# Runtime stage
FROM node:20-alpine

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install production dependencies only
RUN npm ci --omit=dev --ignore-scripts

# Copy built application from builder stage
COPY --from=builder /app/dist ./dist

# Set environment variables (can be overridden at runtime)
# Default to host.docker.internal for accessing host services from container
ENV ANNET_OIL_API_URL=http://host.docker.internal:8080
ENV ANNET_OIL_AUTH_TOKEN=change-me-in-production
ENV NODE_ENV=production

# Create non-root user
RUN addgroup -g 1001 -S nodejs && \
    adduser -S nodejs -u 1001

USER nodejs

# Expose the stdio transport
EXPOSE 3000

# Start the MCP server
CMD ["node", "dist/index.js"]