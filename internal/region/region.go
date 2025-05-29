package region

import (
	"errors"
	"net"

	"gslb/internal/models"
)

func subnetContains(outer, inner *net.IPNet) bool {
	// Check if the inner network address is within the outer network
	if !outer.Contains(inner.IP) {
		return false
	}

	// Calculate the mask length of the outer network
	outerOnes, _ := outer.Mask.Size()

	// Calculate the mask length of the inner network
	innerOnes, _ := inner.Mask.Size()

	// Check if the inner network has a more specific mask
	return innerOnes >= outerOnes
}

// This function returns the name of the region the given ip is a part of
func GetIPRegion(ip string, config models.Configuration) (string, error) {
	for _, region := range config.Regions {
		for _, subnet := range region.Subnets {
			ip := net.ParseIP(ip)

			_, ipNet, err := net.ParseCIDR(subnet)
			if err != nil {
				return "", errors.New("unable to parse subnet")
			}

			if ipNet.Contains(ip) {
				return region.Region, nil
			}
		}
	}

	return "", errors.New("no region found")
}

// This function returns the region the given CIDR is a part of
func GetCIDRRegion(client_cidr string, config models.Configuration) (string, error) {
	for _, region := range config.Regions {
		for _, subnet := range region.Subnets {
			_, client_network, err := net.ParseCIDR(client_cidr)

			if err != nil {
				return "", errors.New("unable to parse client subnet")
			}

			_, region_network, err := net.ParseCIDR(subnet)
			if err != nil {
				return "", errors.New("unable to parse region subnet")
			}

			if subnetContains(region_network, client_network) {
				return region.Region, nil
			}
		}
	}

	return "", errors.New("no region found")
}
