package main

import (
	"log"
	"nutrack/backend/api"
	"nutrack/backend/data"
	"nutrack/backend/messaging"
	"os"
)

func main() {
	data.InitDatabase()

	useElectronIPC := os.Getenv("USE_ELECTRON_IPC") == "1"
	log.Println("useElectronIPC:", useElectronIPC)
	if useElectronIPC {
		log.Println("Running with StandardIO interface")
		handler := api.NewStandardIOHandler()
		messaging.InitBroadcaster(useElectronIPC)
		handler.Start()
	} else {
		log.Println("Running with REST API interface")
		router := api.NewRouter()
		messaging.InitBroadcaster(useElectronIPC)
		router.SetupAndRunApiServer()
	}

}
