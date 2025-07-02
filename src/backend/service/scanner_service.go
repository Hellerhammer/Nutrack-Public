package service

import (
	"encoding/json"
	"fmt"
	local_settings "nutrack/backend/settings"
	"os/exec"
)

type ScannerDevice struct {
	Name      string `json:"name"`
	VendorID  string `json:"vendor_id"`
	ProductID string `json:"product_id"`
	Path      string `json:"path"`
	IsActive  bool   `json:"is_active"`
}

// ListDevices runs the Python script that lists the available scanners
func (s *FoodService) ListDevices() ([]ScannerDevice, error) {
	cmd := exec.Command("python3", "-c", `
import evdev
import json
import sys

devices = []
for path in evdev.list_devices():
    try:
        device = evdev.InputDevice(path)
        devices.append({
            "name": device.name,
            "vendor_id": hex(device.info.vendor)[2:],  # Remove '0x' prefix
            "product_id": hex(device.info.product)[2:], # Remove '0x' prefix
            "path": device.path
        })
    except Exception as e:
        print(f"Error accessing device {path}: {e}", file=sys.stderr)

print(json.dumps(devices))
`)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list scanners: %v\nOutput: %s", err, string(output))
	}

	var devices []ScannerDevice
	if err := json.Unmarshal(output, &devices); err != nil {
		return nil, fmt.Errorf("failed to parse scanner list: %v\nOutput: %s", err, string(output))
	}

	fmt.Printf("Found %d devices from Python script\n", len(devices))
	for i, dev := range devices {
		fmt.Printf("Device %d: %+v\n", i+1, dev)
	}

	// Get active scanner from settings
	settings, _ := s.settingsStore.Load()
	if settings.ActiveScanner != nil {
		fmt.Printf("Active scanner from settings: %+v\n", settings.ActiveScanner)
		for i := range devices {
			if devices[i].VendorID == settings.ActiveScanner.VendorID &&
				devices[i].ProductID == settings.ActiveScanner.ProductID {
				devices[i].IsActive = true
				fmt.Printf("Marked device %d as active\n", i+1)
				break
			}
		}
	}

	return devices, nil
}

// SetActiveScanner sets the active scanner and starts it
func (s *FoodService) SetActiveScanner(devicePath string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	fmt.Printf("Setting active scanner with path: %s\n", devicePath)

	// If no path is provided, deactivate the scanner
	if devicePath == "" || devicePath == "null" {
		if s.activeCmd != nil && s.activeCmd.Process != nil {
			fmt.Printf("Stopping active scanner\n")
			if err := s.StopListening(); err != nil {
				return fmt.Errorf("failed to stop scanner: %v", err)
			}
		}
		// Remove active scanner from settings
		settings, _ := s.settingsStore.Load()
		settings.ActiveScanner = nil
		return s.settingsStore.Save(settings)
	}

	// Find the device in the list
	devices, err := s.ListDevices()
	if err != nil {
		return fmt.Errorf("failed to list devices: %v", err)
	}

	var activeDevice *ScannerDevice
	for _, device := range devices {
		if device.Path == devicePath {
			activeDevice = &device
			fmt.Printf("Found matching device: %+v\n", device)
			break
		}
	}

	if activeDevice == nil {
		return fmt.Errorf("device not found: %s", devicePath)
	}

	// Stop previous scanner, if one is running
	if s.activeCmd != nil && s.activeCmd.Process != nil {
		fmt.Printf("Stopping previous scanner\n")
		if err := s.StopListening(); err != nil {
			return fmt.Errorf("failed to stop previous scanner: %v", err)
		}
	}

	// Save the active scanner in the settings
	settings, _ := s.settingsStore.Load()
	settings.ActiveScanner = &local_settings.ScannerSettings{
		VendorID:  activeDevice.VendorID,
		ProductID: activeDevice.ProductID,
		Name:      activeDevice.Name,
		Path:      activeDevice.Path,
	}
	if err := s.settingsStore.Save(settings); err != nil {
		fmt.Printf("Failed to save scanner settings: %v\n", err)
	}

	cmd := exec.Command("python3", "-c", fmt.Sprintf(`
import evdev
import requests
import time
import sys
import traceback
import threading
from datetime import datetime
from queue import Queue
from collections import Counter

# Enhanced logging function
def log_message(message):
    timestamp = datetime.now().strftime("%%Y-%%m-%%d %%H:%%M:%%S")
    print(f"[SCANNER LOG {timestamp}] {message}")

# Output device details
try:
    device = evdev.InputDevice("%s")
    log_message(f"Scanner started on device: {device.name}")
    log_message(f"Device path: {device.path}")
    log_message(f"Device information: Vendor={hex(device.info.vendor)}, Product={hex(device.info.product)}")
    log_message(f"Supported events: {device.capabilities(verbose=True)}")
except Exception as e:
    log_message(f"ERROR opening device: {e}")
    log_message(traceback.format_exc())
    sys.exit(1)

barcode = ""
last_key_time = 0
TIMEOUT = 0.5  # Timeout for barcode input
BATCH_TIMEOUT = 5.0  # 5 seconds timeout for collecting barcodes
nextLetterUppercase = False

# Queues and locks for batch processing
barcode_queue = Queue()
processing_lock = threading.Lock()
is_processing = False
last_barcode_time = 0

def send_barcode_batch(barcode_list):
    if not barcode_list:
        return
        
    url = "http://127.0.0.1:8080/api/foodItems/check-insert-and-consume-batch"
    
    # Count frequency of each barcode
    barcode_counter = Counter(barcode_list)
    log_message(f"Sending barcode batch with {len(barcode_list)} barcodes ({len(barcode_counter)} unique) to {url}")
    
    items = []
    for code, count in barcode_counter.items():
        log_message(f"Barcode {code} was scanned {count} times")
        items.append({
            "barcode": code,
            "consumed_quantity": count,  # Use the number of scans as quantity
        })
    
    payload = {
        "items": items,
        "force_sync": True
    }
    
    log_message(f"Payload: {payload}")
    headers = {
        "Content-Type": "application/json"
    }
    
    try:
        log_message("Sending batch API request...")
        response = requests.post(url, json=payload, headers=headers)
        log_message(f"API response status: {response.status_code}")
        response.raise_for_status()
        log_message(f"API response: {response.json()}")
    except Exception as e:
        log_message(f"ERROR sending barcode batch: {e}")
        log_message(traceback.format_exc())

def batch_processor():
    global is_processing, last_barcode_time
    
    while True:
        current_time = time.time()
        
        # If 5 seconds have passed since the last barcode and there are barcodes in the queue
        if not is_processing and not barcode_queue.empty() and (current_time - last_barcode_time) >= BATCH_TIMEOUT:
            with processing_lock:
                is_processing = True
                
                # Collect all barcodes from the queue
                barcode_list = []
                while not barcode_queue.empty():
                    barcode_list.append(barcode_queue.get())
                
                log_message(f"Processing batch with {len(barcode_list)} barcodes after {BATCH_TIMEOUT}s timeout")
                send_barcode_batch(barcode_list)
                is_processing = False
        
        time.sleep(0.1)  # Short pause to reduce CPU load

# Start the batch processor in a separate thread
batch_thread = threading.Thread(target=batch_processor, daemon=True)
batch_thread.start()

try:
    log_message(f"Starting scanner loop for device: {device.name}")
    log_message(f"Batch mode activated: Collecting barcodes for {BATCH_TIMEOUT} seconds")
    
    for event in device.read_loop():
        if event.type == evdev.ecodes.EV_KEY:
            key_event = evdev.categorize(event)
            if key_event.keystate == key_event.key_down:
                current_time = time.time()
                if current_time - last_key_time > TIMEOUT:
                    if barcode:
                        log_message(f"Timeout reached, resetting barcode. Old value: {barcode}")
                    barcode = ""
                last_key_time = current_time
                
                key = evdev.ecodes.KEY[key_event.scancode]
                log_message(f"Key event: {key}, Scancode: {key_event.scancode}")
                
                if key == "KEY_ENTER":
                    if barcode:
                        log_message(f"ENTER pressed, adding barcode to batch: {barcode}")
                        barcode_queue.put(barcode)
                        last_barcode_time = time.time()  # Update time of last barcode
                        barcode = ""
                    else:
                        log_message("ENTER pressed, but barcode is empty")
                elif key == "KEY_LEFTSHIFT":
                    log_message("SHIFT pressed, next letter will be uppercase")
                    nextLetterUppercase = True
                else:
                    key = key.replace("KEY_", "").lower()
                    if nextLetterUppercase:
                        barcode += key.upper()
                        log_message(f"Adding uppercase letter: {key.upper()}, current barcode: {barcode}")
                        nextLetterUppercase = False
                    else:
                        barcode += key
                        log_message(f"Adding letter: {key}, current barcode: {barcode}")
except KeyboardInterrupt:
    log_message("Scanner terminated by user (Keyboard Interrupt)")
except Exception as e:
    log_message(f"CRITICAL ERROR in scanner: {e}")
    log_message(traceback.format_exc())
`, activeDevice.Path))
	s.activeCmd = cmd
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start scanner: %v", err)
	}

	fmt.Printf("Scanner started successfully\n")
	return nil
}

// StopListening stops the active scanner process
func (s *FoodService) StopListening() error {
	if s.activeCmd == nil || s.activeCmd.Process == nil {
		return nil
	}

	if err := s.activeCmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill scanner process: %v", err)
	}

	s.activeCmd = nil
	return nil
}
