package main

import (
	"context"
	"log"
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

func healthcheck(config models.Configuration, redisClient *redis.Client) {
	var url string = ""
	var healthy string = "no"

	err := redisClient.Watch(ctx, func(tx *redis.Tx) error {
		_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			if first_run {
				//Initialize Redis Data
				err := pipe.FlushDB(ctx).Err()
				if err != nil {
					return err
				}
			}

			//For each application...
			for _, application := range config.Applications {
				//Perform the health check for each instance of the above application
				for _, instance := range application.Instances {
					url = application.Healthcheck.Type + "://" + instance.Ip + ":" + strconv.Itoa(application.Healthcheck.Port)
					req, err := http.NewRequest("GET", url, nil)
					if err != nil {
						log.Println(err.Error())
					}

					//Set Host header for HTTPS/SNI
					req.Host = application.Hostname

					client := &http.Client{}
					resp, err := client.Do(req)
					if err != nil {
						log.Println(err.Error())
					} else {
						defer resp.Body.Close()

						if resp.StatusCode == 200 {
							healthy = "yes"
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

					_, err = pipe.HSet(ctx, application.Hostname+".:"+instance.Ip, fields).Result()

					if err != nil {
						log.Println(err.Error())
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

	//Open configuration file and handle related errors
	configYaml, err := os.ReadFile("sextant.yml")

	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	//Read the configuration file data into a Configuration
	//struct and handle related errors

	var config models.Configuration

	err = yaml.Unmarshal(configYaml, &config)
	if err != nil {
		log.Println(err.Error())
		os.Exit(2)
	}

	log.Println("Successfully read configuration")

	//Connect to Redis
	ctx = context.Background()

	ctx := context.Background()
	redisClient, err := redisclient.New(ctx, config)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	//Main loop

	for {
		if time.Now().Unix() >= last_healthcheck+int64(config.Sextant.Healthchecks.Frequency) {
			last_healthcheck = time.Now().Unix()
			healthcheck(config, redisClient)
			time.Sleep(time.Second)
		}
	}

}
