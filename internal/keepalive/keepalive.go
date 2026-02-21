package keepalive

import (
	"log"
	"net/http"
	"time"
)

// Start pings the specified URL every interval to keep the service alive.
func Start(url string, interval time.Duration) {
	if url == "" {
		log.Println("Keep-alive: No URL provided, background pinger not started")
		return
	}

	log.Printf("Keep-alive: Starting pinger for %s every %v", url, interval)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			resp, err := http.Get(url)
			if err != nil {
				log.Printf("Keep-alive error: Failed to ping %s: %v", url, err)
				continue
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				log.Println("Keep-alive: Ping successful")
			} else {
				log.Printf("Keep-alive: Received non-OK status: %s", resp.Status)
			}
		}
	}()
}
