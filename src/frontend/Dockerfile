# Build stage
FROM node:16-alpine AS build

WORKDIR /app

# Copy package files first for better caching
COPY package*.json ./

# Install dependencies
RUN npm install

# Copy the rest of the application
COPY . .

# Build arguments and environment variables
ARG REACT_APP_BACKEND_URL
ARG REACT_APP_USE_ELECTRON_IPC
ARG DROPBOX_CLIENT_ID
ENV REACT_APP_BACKEND_URL=$REACT_APP_BACKEND_URL
ENV REACT_APP_USE_ELECTRON_IPC=$REACT_APP_USE_ELECTRON_IPC
ENV REACT_APP_DROPBOX_CLIENT_ID=$DROPBOX_CLIENT_ID

# Build the application
RUN npm run build:docker

# Production stage
FROM nginx:alpine

# Copy the built application
COPY --from=build /app/build /usr/share/nginx/html

# Copy nginx configuration and entrypoint script
COPY nginx.conf /etc/nginx/nginx.conf
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Create directory for runtime configuration
RUN mkdir -p /usr/share/nginx/html/runtime-config

# Set the entrypoint
ENTRYPOINT ["/entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]