// internal/lb/lb.go
package lb

import (
	"context"
	"log"
	"math/rand"
	"sort"
	"strings"
	"time"

	"gslb/internal/models"

	"github.com/redis/go-redis/v9"
)

// RoundRobin rotates between healthy instances using Redis to track the counter.
func RoundRobin(ctx context.Context, redisClient *redis.Client, queryName string, instances []models.Instance) models.Instance {
	if len(instances) == 0 {
		return models.Instance{}
	}

	key := queryName + ":rr_index"
	index, err := redisClient.Incr(ctx, key).Result()
	if err := redisClient.Expire(ctx, key, 5*time.Minute).Err(); err != nil {
		log.Printf("Failed to set TTL on round-robin key %s: %v", key, err)
	}

	if err != nil {
		// fallback to random on Redis failure
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		return instances[r.Intn(len(instances))]
	}

	selected := int(index) % len(instances)
	return instances[selected]
}

// Failover returns the first healthy instance in config order.
func Failover(instances []models.Instance, configOrder []models.Instance) models.Instance {
	// map healthy instances by IP for fast lookup
	ipMap := make(map[string]models.Instance)
	for _, inst := range instances {
		ipMap[inst.Ip] = inst
	}

	// iterate through config-ordered instances
	for _, preferred := range configOrder {
		if inst, ok := ipMap[preferred.Ip]; ok {
			return inst
		}
	}

	// fallback if none from configOrder are healthy
	if len(instances) > 0 {
		return instances[0]
	}

	// fallback if no healthy instances at all
	return models.Instance{}
}

// RegionAware returns the least-used healthy instance in-region, falling back to global best.
func RegionAware(global, inRegion []models.Instance) models.Instance {
	candidates := inRegion
	if len(inRegion) == 0 {
		//There are no in-region backends, so we fall back to all backends
		candidates = global
	}

	//Return the least used candidate of the candidates
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Count < candidates[j].Count
	})

	return candidates[0]
}

// Find application by query name
func GetAppByHostname(config models.Configuration, hostname string) *models.Application {

	//Remove trailing . sent by PowerDNS
	hostname = strings.TrimSuffix(hostname, ".")

	for _, app := range config.Applications {
		if strings.EqualFold(app.Hostname, hostname) {
			return &app
		}
	}
	return nil
}

// IncrementCount increases the usage counter for the selected instance in Redis.
func IncrementCount(ctx context.Context, redisClient *redis.Client, queryName string, instance models.Instance) error {
	key := queryName + ".:" + instance.Ip
	return redisClient.HIncrBy(ctx, key, "count", 1).Err()
}
