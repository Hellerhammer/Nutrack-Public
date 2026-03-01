# Use an official Go image (Debian-based)
FROM golang:1.23

# Define build argument for the Client ID
ARG DROPBOX_CLIENT_ID="wtxrzaiqahkjucf"

# Install Python and required packages
RUN apt-get update && \
    apt-get install -y python3 python3-pip python3-venv python3-evdev && \
    rm -rf /var/lib/apt/lists/*

RUN mkdir -p /app/data

# Set the working directory in the container
WORKDIR /app

# Create and activate virtual environment for additional packages
RUN python3 -m venv /app/venv
ENV PATH="/app/venv/bin:$PATH"

# Copy requirements.txt and install additional Python dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt && \
    pip install --no-cache-dir evdev

# Ensure that the input group exists and the user has the correct permissions
RUN groupadd -f input && \
    usermod -a -G input root && \
    chmod -R 660 /dev/input/* || true

# Copy the Go workspace file (go.work)
COPY go.work go.work

# Copy all modules into the container
COPY main/ /app/main/
COPY api/ /app/api/
COPY data/ /app/data/
COPY service/ /app/service/
COPY messaging/ /app/messaging/
COPY settings/ /app/settings/
COPY types/ /app/types/

# Set up the Go workspace (install dependencies)
RUN go work sync

# Build the Go project (adjusted to the module you want to start)
RUN go build -o /app/main/main ./main/

ENV BACKEND_URL=http://nutrack-backend:8080
ENV CONTAINER=true
ENV DROPBOX_CLIENT_ID=$DROPBOX_CLIENT_ID
ENV HOST_URL=localhost:8080

ARG TZ=Europe/Berlin
ENV TZ=$TZ
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

EXPOSE 8080

# Start the application
CMD ["/app/main/main"]