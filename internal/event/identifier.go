package event

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"

	"github.com/denisbrodbeck/machineid"
)

var distinctId string

const (
	hashKey    = "charm"
	fallbackId = "unknown"
)

func getDistinctId() string {
	if id, err := machineid.ProtectedID(hashKey); err == nil {
		return id
	}
	if macAddr, err := getMacAddr(); err == nil {
		return hashString(macAddr)
	}
	return fallbackId
}

func getMacAddr() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 && len(iface.HardwareAddr) > 0 {
			if addrs, err := iface.Addrs(); err == nil && len(addrs) > 0 {
				return iface.HardwareAddr.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no active interface with mac address found")
}

func hashString(str string) string {
	hash := hmac.New(sha256.New, []byte(str))
	hash.Write([]byte(hashKey))
	return hex.EncodeToString(hash.Sum(nil))
}
