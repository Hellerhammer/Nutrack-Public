# Nutrack - Nutrition Tracking App

Nutrack is an application for tracking daily nutrition and calorie intake. The application consists of a frontend (React), a backend (Go), and can optionally be run as a desktop application using Electron, the last part is currently not fully implemented.

There is a mobile app for Android soon to be available on the Google Play Store.

This whole Project was first and formost created as an exercise for me to learn Go and Docker, as well as try out different technologies, frameworks, while creating a project that I could use for my personal needs. 

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

1. Copy docker-compose.yml and .env.example to the root directory of the project.

2. Adjust the values in the `.env` file as needed.

3. Pull the required Docker images:
   ```bash
   docker-compose pull
   ```

4. Start the application:
   ```bash
   docker-compose up -d
   ```

5. Open the application in your browser:
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

### secrets

When you build the images from the repository, the client ID for Dropbox is not included. The client ID for Dropbox is currently stored in the built images as an environment variable.
If you want to build the images yourself, you should set the environment variable `DROPBOX_CLIENT_ID` while building the images, which you can either just request per Mail from kachonkdev@gmail.com or use your own, i will probably just include the ID in the repo in the future.

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

