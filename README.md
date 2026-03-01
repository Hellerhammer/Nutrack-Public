# Nutrack - Nutrition Tracking App

Nutrack is an application for tracking daily nutrition and calorie intake. The application consists of a frontend (React), a backend (Go), and can optionally be run as a desktop application using Electron, the last part is currently not fully implemented.

There is a mobile app for Android soon to be available on the Google Play Store.

This whole Project was first and formost created as an exercise for me to learn a bit of Go, React and Docker, as well as try out different technologies, frameworks, while creating a project that I could use for my personal needs. 

![Nutrack](https://github.com/hellerhammer/nutrack-public/blob/main/images/nutrack-web.png)

## Key Features

- **Food Diary**: Log your meals and drinks
- **Food Database**: Add your own food items to a local database or choose from the open food facts database
- **Dishes**: Create your own dishes and add them to your diary
- **Profiles**: Create multiple profiles for different users
- **Dropbox Sync**: Synchronize your data across multiple devices and a mobile app
- **Statistics**: Track your progress
- **Barcode Scanner**: Connect a barcode scanner to scan and add food items to your diary. The barcode scanner can be directly connected to the host device (currently only tested with a Raspberry Pi) or you can call an API endpoint to add food items to your diary/local database.

## Prerequisites

- [Docker](https://www.docker.com/get-started) and [Docker Compose](https://docs.docker.com/compose/install/)
- (Optional) Node.js and npm for development

## Installation with Docker Compose

1. Start the application and build:
   ```bash
   docker-compose up -d --build
   ```

2. Open the application in your browser:
   ```
   http://localhost:82
   ```
   or on other devices:
   ```
   http://<host-ip>:82
   ```

## Configuration

### .env

- `HOST_URL`: The base URL of your application (default: `localhost:82`)

### docker-compose.yml

- `ALLOWED_IPS`: A comma-separated list of allowed IP addresses from where the frontend can access the backend

## API-Doc

When you host the backend, there should be a swagger doc for the api endpoints: "http://{host}:{port}/swagger/index.html".
You can also look this up under src\backend\api\docs.

## Project Structure

- `/src/frontend`: React-based web interface
- `/src/backend`: Go backend with REST API
- `/docker`: Docker configurations
- `/electron`: Desktop application configuration

## Development

### Start Frontend

```bash
cd src/frontend
npm install
npm start
```

### Start Backend

```bash
cd src/backend/main
go run .
```

