package worker

import (
	"log"
	"time"

	"devsync/backend/repository"
	"devsync/backend/websocket"
)

// StartDeviceOfflineWorker starts a background ticker that checks if devices have gone offline
func StartDeviceOfflineWorker(deviceRepo *repository.DeviceRepository) {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			devices, err := deviceRepo.GetAll()
			if err != nil {
				log.Printf("[Device Worker] Error fetching devices: %v", err)
				continue
			}

			changed := false
			now := time.Now()
			for _, d := range devices {
				// If last seen is older than 30 seconds and device is online, mark as offline
				if d.Status == "online" && now.Sub(d.LastSeen) > 30*time.Second {
					log.Printf("[Device Worker] Device %s (%s) is inactive. Marking offline.", d.DeviceName, d.IPAddress)
					err := deviceRepo.UpdateStatus(d.IPAddress, "offline")
					if err != nil {
						log.Printf("[Device Worker] Error updating status for %s: %v", d.IPAddress, err)
					} else {
						changed = true
					}
				}
			}

			if changed {
				// Broadcast the updated device list to all connected websockets
				updatedList, err := deviceRepo.GetAll()
				if err == nil {
					websocket.GlobalHub.Broadcast("devices_list", updatedList)
				}
			}
		}
	}()
}
