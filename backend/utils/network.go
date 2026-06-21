package utils

import (
	"net"
)

// GetLocalIP returns the first non-loopback IPv4 address found
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, address := range addrs {
		// Check the address type and if it is not a loopback
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				// Avoid docker interface IPs if possible (typically 172.17.x.x or 172.18.x.x)
				ipStr := ipnet.IP.String()
				if !isDockerIP(ipStr) {
					return ipStr
				}
			}
		}
	}
	// Fallback to loopback if nothing else is found
	return "127.0.0.1"
}

func isDockerIP(ip string) bool {
	// Simple heuristic to ignore common docker subnet ranges
	// Docker default bridge is 172.17.0.0/16
	if len(ip) >= 7 && ip[:7] == "172.17." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.18." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.19." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.20." {
		return true
	}
	return false
}
