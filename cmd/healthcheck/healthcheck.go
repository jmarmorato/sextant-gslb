package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"gslb/internal/models"
	"gslb/internal/redisclient"

	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v2"
)

var ctx context.Context
var first_run bool
var last_healthcheck int64

func init() {
	first_run = true
}

func healthcheck(config models.Configuration, redisClient *redis.Client, httpClient *http.Client) {
	err := redisClient.Watch(ctx, func(tx *redis.Tx) error {
		_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			if first_run {
				// Initialize Redis Data once
				if err := pipe.FlushDB(ctx).Err(); err != nil {
					return err
				}
			}

			// For each application...
			for _, application := range config.Applications {
				// Perform the health check for each instance of the above application
				for _, instance := range application.Instances {
					// Reset per-instance state
					healthy := "no"

					url := application.Healthcheck.Type + "://" + instance.Ip + ":" + strconv.Itoa(application.Healthcheck.Port)
					req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
					if err != nil {
						log.Println("new request:", err)
					} else {
						// Set Host header for HTTPS/SNI (and HTTP virtual host)
						req.Host = application.Hostname

						resp, err := httpClient.Do(req)
						if err != nil {
							log.Println("http:", err)
						} else {
							func() {
								defer resp.Body.Close()
								if resp.StatusCode == http.StatusOK {
									healthy = "yes"
								}
							}()
						}
					}

					fields := []string{
						"application", application.Name,
						"ip", instance.Ip,
						"healthy", healthy,
					}

					instance.Application = application.Name
					instance.Healthy = healthy

					log.Println("Instance", instance)

					if _, err := pipe.HSet(ctx, application.Hostname+".:"+instance.Ip, fields).Result(); err != nil {
						log.Println("redis HSet:", err)
					}
				}
			}
			return nil
		})
		return err
	})

	if err != nil {
		log.Fatalf("Transaction failed: %v", err)
	}

	log.Println("Transaction completed successfully!")
	first_run = false
	last_healthcheck = time.Now().Unix()
}

func main() {
	log.Println("Starting Sextant")

	// Open configuration file and handle related errors
	configYaml, err := os.ReadFile("sextant.yml")
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	// Unmarshal configuration
	var config models.Configuration
	if err := yaml.Unmarshal(configYaml, &config); err != nil {
		log.Println(err.Error())
		os.Exit(2)
	}
	log.Println("Successfully read configuration")

	// Global context (can be replaced with cancellable ctx if you wire up signals)
	ctx = context.Background()

	// Connect to Redis
	redisClient, err := redisclient.New(ctx, config)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Reusable HTTP client with timeouts (avoid per-request allocations)
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			// Reasonable defaults; tune as needed
			DialContext: (&net.Dialer{
				Timeout:   2 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   2 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	// ===== Main loop with low CPU usage =====
	freq := time.Duration(config.Sextant.Healthchecks.Frequency) * time.Second

	for {
		start := time.Now()

		healthcheck(config, redisClient, httpClient)

		// Sleep the remainder of the period; if we overran, loop again immediately
		if remain := freq - time.Since(start); remain > 0 {
			timer := time.NewTimer(remain)
			select {
			case <-timer.C:
				// continue
			case <-ctx.Done():
				timer.Stop()
				return
			}
		}
		// else: no sleep → immediate next run (overrun case)
	}
}
