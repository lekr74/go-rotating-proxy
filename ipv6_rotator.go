package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"
)

type SubnetConfig struct {
	Subnets []string `json:"subnets"`
}

type IPv6Rotator struct {
	mu            sync.Mutex
	subnetPool    []*net.IPNet   // Les /42 ou /44 chargés depuis le fichier
	active48s     []*net.IPNet   // Un /48 choisi aléatoirement dans chaque bloc
	rotationCount int
	rotationLimit int
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

func NewIPv6Rotator(subnetStrings []string, rotationLimit int) (*IPv6Rotator, error) {
	var parsed []*net.IPNet
	for _, s := range subnetStrings {
		_, ipnet, err := net.ParseCIDR(s)
		if err != nil {
			log.Printf("Erreur parsing subnet %s: %v", s, err)
			continue
		}
		if ipnet.IP.To16() == nil || ipnet.IP.To4() != nil {
			continue // skip non-IPv6
		}
		parsed = append(parsed, ipnet)
	}
	if len(parsed) == 0 {
		return nil, fmt.Errorf("aucun bloc IPv6 valide trouvé")
	}

	r := &IPv6Rotator{
		subnetPool:    parsed,
		rotationLimit: rotationLimit,
	}
	r.rotatePools()
	return r, nil
}

func (r *IPv6Rotator) rotatePools() {
	var newPools []*net.IPNet
	for _, block := range r.subnetPool {
		prefixLen, _ := block.Mask.Size()
		offsetBits := 48 - prefixLen
		if offsetBits < 0 {
			offsetBits = 0
		}

		base := big.NewInt(0).SetBytes(block.IP)
		max := new(big.Int).Lsh(big.NewInt(1), uint(offsetBits))
		randOffset := new(big.Int).Rand(rand.New(rand.NewSource(time.Now().UnixNano())), max)
		randOffset.Lsh(randOffset, 128-48) // position de /48
		base.Add(base, randOffset)

		ip := base.Bytes()
		if len(ip) < 16 {
			pad := make([]byte, 16-len(ip))
			ip = append(pad, ip...)
		}

		_, ipnet, _ := net.ParseCIDR(fmt.Sprintf("%s/48", net.IP(ip).String()))
		newPools = append(newPools, ipnet)
		log.Printf("🎯 Pool actif : %s", ipnet.String())
	}
	r.active48s = newPools
}

func (r *IPv6Rotator) RandomIPv6() net.IP {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.rotationCount++
	if r.rotationCount >= r.rotationLimit {
		r.rotatePools()
		r.rotationCount = 0
	}

	selectedPool := r.active48s[rand.Intn(len(r.active48s))]
	base := selectedPool.IP
	prefixLen, _ := selectedPool.Mask.Size()

	nBits := 128 - prefixLen
	max := new(big.Int).Lsh(big.NewInt(1), uint(nBits))
	randOffset := new(big.Int).Rand(rand.New(rand.NewSource(time.Now().UnixNano())), max)
	baseInt := big.NewInt(0).SetBytes(base)
	sum := new(big.Int).Add(baseInt, randOffset)

	full := sum.Bytes()
	if len(full) < 16 {
		padding := make([]byte, 16-len(full))
		full = append(padding, full...)
	}

	return net.IP(full)
}

func (r *IPv6Rotator) UpdateSubnets(subnetStrings []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var parsed []*net.IPNet
	for _, s := range subnetStrings {
		_, ipnet, err := net.ParseCIDR(s)
		if err != nil {
			log.Printf("Erreur parsing subnet %s: %v", s, err)
			continue
		}
		parsed = append(parsed, ipnet)
	}
	r.subnetPool = parsed
	r.rotatePools()
}
