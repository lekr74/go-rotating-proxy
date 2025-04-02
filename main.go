// main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/armon/go-socks5"
)

func main() {
	// Charger la configuration utilisateur
	config, err := LoadConfig("users.yaml")
	if err != nil {
		log.Fatalf("Erreur de chargement de la config: %v", err)
	}

	// Charger la configuration IPv6
	subnetConfig, err := LoadIPv6Subnets("subnets.json")
	if err != nil {
		log.Fatalf("Erreur de chargement des subnets IPv6: %v", err)
	}

	rotator, err := NewIPv6Rotator(subnetConfig.Subnets)
	if err != nil {
		log.Fatalf("Erreur d'initialisation du rotator IPv6: %v", err)
	}

	// Appliquer les routes locales pour chaque bloc IPv6
	err = ApplyLocalRoutes(subnetConfig.Subnets, "enp6s18")
	if err != nil {
		log.Fatalf("Erreur application des routes locales: %v", err)
	}

	// Fonction de connexion avec IP source IPv6 aléatoire
	dialFunc := func(ctx context.Context, network, addr string) (net.Conn, error) {
		srcIP := rotator.RandomIPv6()
		localAddr := &net.TCPAddr{IP: srcIP}
		dialer := &net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
			LocalAddr: localAddr,
		}
		fmt.Printf("Connexion vers %s avec IP source %s\n", addr, srcIP)
		return dialer.DialContext(ctx, network, addr)
	}

	// Authentification
	authenticator := socks5.UserPassAuthenticator{
		Credentials: socks5.StaticCredentials(config.Users),
	}

	conf := &socks5.Config{
		AuthMethods: []socks5.Authenticator{&authenticator},
		Dial:        dialFunc,
	}

	server, err := socks5.New(conf)
	if err != nil {
		log.Fatalf("Erreur de création du serveur SOCKS5: %v", err)
	}

	addr := ":1080"
	fmt.Printf("Proxy SOCKS5 lancé sur %s\n", addr)

	err = server.ListenAndServe("tcp", addr)
	if err != nil {
		log.Fatalf("Erreur de démarrage du serveur: %v", err)
	}
}

func ApplyLocalRoutes(subnets []string, iface string) error {
	for _, subnet := range subnets {
		cmd := exec.Command("ip", "-6", "route", "add", "local", subnet, "dev", iface)
		output, err := cmd.CombinedOutput()
		if err != nil && !strings.Contains(string(output), "File exists") {
			return fmt.Errorf("impossible d'ajouter la route pour %s : %v (%s)", subnet, err, strings.TrimSpace(string(output)))
		} else if err == nil {
			log.Printf("Route locale ajoutée pour %s sur %s", subnet, iface)
		}
	}
	return nil
}
