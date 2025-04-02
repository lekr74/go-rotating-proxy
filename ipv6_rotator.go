package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"net"
	"os"
	"time"
	"log"
	"sync"
)

type SubnetConfig struct {
	Subnets []string `json:"subnets"`
}

type IPv6Rotator struct {
	subnets []*net.IPNet
	mu      sync.Mutex
}

func LoadIPv6Subnets(filename string) (*SubnetConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var conf SubnetConfig
	err = json.Unmarshal(data, &conf)
	if err != nil {
		return nil, err
	}
	return &conf, nil
}

func NewIPv6Rotator(subnets []string) (*IPv6Rotator, error) {
	var parsed []*net.IPNet
	for _, s := range subnets {
		_, ipnet, err := net.ParseCIDR(s)
		if err != nil {
			return nil, fmt.Errorf("erreur de parsing du subnet %s: %v", s, err)
		}
		if ipnet.IP.To16() == nil || ipnet.IP.To4() != nil {
			continue // skip non-IPv6
		}
		parsed = append(parsed, ipnet)
	}
	if len(parsed) == 0 {
		return nil, fmt.Errorf("aucun bloc IPv6 valide trouvé")
	}
	return &IPv6Rotator{subnets: parsed}, nil
}
func (r *IPv6Rotator) UpdateSubnets(subnets []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var parsed []*net.IPNet
	for _, s := range subnets {
		_, ipnet, err := net.ParseCIDR(s)
		if err != nil {
			log.Printf("Erreur parsing subnet %s: %v", s, err)
			continue
		}
		parsed = append(parsed, ipnet)
	}
	r.subnets = parsed
}

func (r *IPv6Rotator) RandomIPv6() net.IP {
	rand.Seed(time.Now().UnixNano())
	block := r.subnets[rand.Intn(len(r.subnets))]

	base := block.IP
	prefixLen, _ := block.Mask.Size()

	nBits := 128 - prefixLen
	max := new(big.Int).Lsh(big.NewInt(1), uint(nBits))
	randOffset := new(big.Int).Rand(rand.New(rand.NewSource(time.Now().UnixNano())), max)

	baseInt := big.NewInt(0).SetBytes(base)
	sum := big.NewInt(0).Add(baseInt, randOffset)

	full := sum.Bytes()
	if len(full) < 16 {
		// pad to 16 bytes
		padding := make([]byte, 16-len(full))
		full = append(padding, full...)
	}

	return net.IP(full)
}
