package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"gslb/internal/config"
	"gslb/internal/lb"
	"gslb/internal/models"
	"gslb/internal/redisclient"
	"gslb/internal/region"

	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client
var ctx context.Context
var cfg models.Configuration
var verbose bool

func vLog(format string, args ...any) {
	if verbose {
		log.Printf(format, args...)
	}
}

func handleDNSQuery(w http.ResponseWriter, r *http.Request) {
	//Get the domain name from the query
	queryName := r.PathValue("queryName")
	vLog("Handling DNS query for %s", queryName)

	//Get the application from configuration.  We will use this configuration
	//to decide what load balancing method to use.
	app := lb.GetAppByHostname(cfg, queryName)
	if app == nil {
		http.Error(w, "Application not found", http.StatusNotFound)
		vLog("Application not found for %s", queryName)
		return
	}

	vLog("Matched application: %+v", app)

	//Search Redis for hosts that match the queried domain name
	cursor := uint64(0)
	keys, _, err := redisClient.Scan(ctx, cursor, queryName+":*", 0).Result()
	if err != nil {
		http.Error(w, "Failed to query Redis", http.StatusInternalServerError)
		return
	}
	vLog("Discovered %d keys for application", len(keys))

	//Determine the client's region using EDNS
	clientIP := r.Header.Get("x-remotebackend-remote")
	clientCIDR := r.Header.Get("X-Remotebackend-Real-Remote")

	var clientRegion string

	if clientCIDR != "" {
		clientRegion, err = region.GetCIDRRegion(clientCIDR, cfg)
		if err != nil {
			vLog("Failed to determine region from CIDR (%s): %v", clientCIDR, err)
		}
	} else {
		clientRegion, err = region.GetIPRegion(clientIP, cfg)
		if err != nil {
			vLog("Failed to determine region from IP (%s): %v", clientIP, err)
		}
	}

	vLog("Client region is %s", clientRegion)

	// Create variables to hold lists of healthy and in-region application instances
	var healthy []models.Instance
	var inRegion []models.Instance

	for _, key := range keys {
		val, err := redisClient.HGetAll(ctx, key).Result()
		if err != nil || val["healthy"] != "yes" {
			continue
		}

		instance := models.Instance{
			Ip:      val["ip"],
			Healthy: val["healthy"],
			Count:   0,
		}

		// Parse usage count
		instance.Count, _ = redisClient.HIncrBy(ctx, key, "count", 0).Result()
		vLog("Instance from Redis: %+v", instance)

		//Determine the instance region
		instance_region, err := region.GetIPRegion(val["ip"], cfg)
		if err != nil {
			vLog("Unable to determine region of instance")
			continue
		}

		if strings.EqualFold(clientRegion, instance_region) {
			inRegion = append(inRegion, instance)
		}

		healthy = append(healthy, instance)
	}

	if len(healthy) == 0 {
		http.Error(w, "No healthy backends", http.StatusServiceUnavailable)
		vLog("No healthy backends for %s", queryName)
		return
	}

	vLog("Total healthy instances: %d", len(healthy))
	vLog("In-region healthy instances: %d", len(inRegion))

	var selected models.Instance
	switch strings.ToLower(app.Method) {
	case "roundrobin":
		vLog("Using RoundRobin strategy")
		selected = lb.RoundRobin(ctx, redisClient, app.Hostname, healthy)
	case "failover":
		vLog("Using Failover strategy")
		selected = lb.Failover(healthy, app.Instances)
	case "region-aware":
		vLog("Using RegionAware strategy")
		selected = lb.RegionAware(healthy, inRegion, app.Instances)
	default:
		vLog("Unknown strategy '%s', defaulting to RegionAware", app.Method)
		selected = lb.RegionAware(healthy, inRegion, app.Instances)
	}

	err = lb.IncrementCount(ctx, redisClient, app.Hostname, selected)
	if err != nil {
		vLog("Error incrementing Redis count: %v", err)
	}

	vLog("Selected instance: %+v", selected)

	record := models.ARecord{
		Qtype:   "A",
		Qname:   queryName,
		Content: selected.Ip,
		TTL:     60,
	}

	result := models.AResult{Result: []models.ARecord{record}}
	jsonData, err := json.Marshal(result)
	if err != nil {
		http.Error(w, "JSON encoding error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)

	vLog("Response sent: %s", jsonData)
}

func ServeSoa(w http.ResponseWriter, r *http.Request) {
	// Extract queryName from the request path
	queryName := r.PathValue("queryName")

	// Build SOA content string using the config values
	content := cfg.Sextant.Fqdn + ". " +
		cfg.Sextant.Soa.Email + ". " +
		cfg.Sextant.Soa.Serial + " " +
		cfg.Sextant.Soa.Refresh + " " +
		cfg.Sextant.Soa.Retry + " " +
		cfg.Sextant.Soa.Expiration + " " +
		cfg.Sextant.Soa.TTL

	// Fill SOA response struct
	data := models.Soa{
		Qtype:    "SOA",
		Qname:    queryName,
		Content:  content,
		TTL:      3600,
		DomainID: -1,
	}

	result := models.SoaResult{Result: []models.Soa{data}}

	// Encode to JSON
	jsonData, err := json.Marshal(result)
	if err != nil {
		if verbose {
			log.Println("[ERROR] Failed to encode SOA response:", err)
		}
		http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
		return
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)

	vLog("Served SOA for %s: %s\n", queryName, content)
}

func ServeDomainMetadata(w http.ResponseWriter, r *http.Request) {
	data := models.DomMetadata{}
	data.Result.Presigned = []string{"0"}

	// Marshal the data into JSON format
	jsonData, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
		return
	}

	// Set the Content-Type header to indicate JSON response
	w.Header().Set("Content-Type", "application/json")

	// Write the JSON data to the response
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)

	vLog("Served Domain Metadata")

}

func main() {
	ctx = context.Background()

	var err error
	cfg, err = config.Load("sextant.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	verbose = cfg.Sextant.Verbose
	vLog("Verbose logging enabled")

	redisClient, err = redisclient.New(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	http.HandleFunc("GET /dns/getAllDomainMetadata/{queryName}", ServeDomainMetadata)
	http.HandleFunc("GET /dns/lookup/{queryName}/SOA", ServeSoa)
	http.HandleFunc("GET /dns/lookup/{queryName}/{queryType}", handleDNSQuery)
	log.Println("Resolver service started on :2112")
	http.ListenAndServe(":2112", nil)
}
