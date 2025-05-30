// main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/armon/go-socks5"
	"github.com/fsnotify/fsnotify"
)

// Resolver personnalisé pour forcer l'utilisation d'IPv6 uniquement
type IPv6OnlyResolver struct{}

func (r *IPv6OnlyResolver) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	ips, err := net.LookupIP(name)
	if err != nil {
		return ctx, nil, err
	}
	for _, ip := range ips {
		if ip.To4() == nil {
			return ctx, ip, nil
		}
	}
	return ctx, nil, fmt.Errorf("aucune IPv6 trouvée pour %s", name)
}

// Stockage dynamique des credentials
type DynamicCredentials struct {
	mu    sync.RWMutex
	users map[string]string
}

func (dc *DynamicCredentials) Valid(user, password string) bool {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	stored, ok := dc.users[user]
	return ok && stored == password
}

func (dc *DynamicCredentials) Update(newUsers map[string]string) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.users = newUsers
}

func main() {
	// Charger la configuration utilisateur
	config, err := LoadConfig("users.yaml")
	if err != nil {
		log.Fatalf("Erreur de chargement de la config: %v", err)
	}

	dynCreds := &DynamicCredentials{users: config.Users}

	// Charger la configuration IPv6
	subnetConfig, err := LoadIPv6Subnets("subnets.json")
	if err != nil {
		log.Fatalf("Erreur de chargement des subnets IPv6: %v", err)
	}

	rotator, err := NewIPv6Rotator(subnetConfig.Subnets, 10000) // rotation toutes les 1000 connexions
	if err != nil {
		log.Fatalf("Erreur d'initialisation du rotator IPv6: %v", err)
	}

	err = ApplyLocalRoutes(subnetConfig.Subnets, "enp6s18")
	if err != nil {
		log.Fatalf("Erreur application des routes locales: %v", err)
	}

	log.Println("Préfixes actifs pour bind IPv6 :")
	exec.Command("sh", "-c", "ip -6 route show table local | grep local").Run()

	// Watchers
	go watchSubnets(rotator, "subnets.json", "enp6s18")
	go watchUsers(dynCreds, "users.yaml")

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

	authenticator := socks5.UserPassAuthenticator{
		Credentials: dynCreds,
	}

	resolver := &IPv6OnlyResolver{}

	conf := &socks5.Config{
		AuthMethods: []socks5.Authenticator{&authenticator},
		Dial:        dialFunc,
		Resolver:    resolver,
	}

	server, err := socks5.New(conf)
	if err != nil {
		log.Fatalf("Erreur de création du serveur SOCKS5: %v", err)
	}

	addr := "[::]:1080"
	fmt.Printf("Proxy SOCKS5 lancé sur %s\n", addr)

	ln, err := reusePortListen("tcp", addr)
	if err != nil {
		log.Fatalf("Erreur lors de l'écoute TCP avec SO_REUSEPORT: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		err = server.Serve(ln)
		if err != nil && err != context.Canceled {
			log.Fatalf("Erreur lors du démarrage du serveur SOCKS5: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Arrêt demandé, fermeture du proxy...")
	ln.Close()
}

func ApplyLocalRoutes(subnets []string, iface string) error {
	for _, subnet := range subnets {
		cmd := exec.Command("ip", "-6", "route", "add", "local", subnet, "dev", iface)
		output, err := cmd.CombinedOutput()
		if err != nil && !strings.Contains(string(output), "File exists") {
			return fmt.Errorf("impossible d'ajouter la route pour %s : %v (%s)", subnet, err, strings.TrimSpace(string(output)))
		} else {
			log.Printf("Route locale ajoutée pour %s sur %s", subnet, iface)
		}
	}
	return nil
}

func watchSubnets(rotator *IPv6Rotator, path string, iface string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Erreur de création du watcher: %v", err)
		return
	}
	defer watcher.Close()

	err = watcher.Add(path)
	if err != nil {
		log.Printf("Erreur de surveillance du fichier %s: %v", path, err)
		return
	}

	log.Printf("Surveillance de %s pour mise à jour dynamique des blocs IPv6", path)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Println("Changement détecté dans subnets.json, rechargement...")
				conf, err := LoadIPv6Subnets(path)
				if err != nil {
					log.Printf("Erreur lors du rechargement de %s: %v", path, err)
					continue
				}
				err = ApplyLocalRoutes(conf.Subnets, iface)
				if err != nil {
					log.Printf("Erreur application des routes: %v", err)
				}
				rotator.UpdateSubnets(conf.Subnets)
				log.Println("Blocs IPv6 mis à jour dynamiquement.")
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Erreur watcher: %v", err)
		}
	}
}

func watchUsers(creds *DynamicCredentials, path string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Erreur création watcher utilisateurs: %v", err)
		return
	}
	defer watcher.Close()

	err = watcher.Add(path)
	if err != nil {
		log.Printf("Erreur de surveillance de %s: %v", path, err)
		return
	}

	log.Printf("Surveillance de %s pour mise à jour dynamique des utilisateurs", path)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Println("Changement détecté dans users.yaml, rechargement...")
				conf, err := LoadConfig(path)
				if err != nil {
					log.Printf("Erreur rechargement users: %v", err)
					continue
				}
				creds.Update(conf.Users)
				log.Println("Utilisateurs mis à jour dynamiquement.")
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Erreur watcher utilisateurs: %v", err)
		}
	}
}


func reusePortListen(network, address string) (net.Listener, error) {
	lc := net.ListenConfig{
		Control: func(_, _ string, c syscall.RawConn) error {
			var serr error
			c.Control(func(fd uintptr) {
				serr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
				if serr != nil {
					return
				}
				// 0x0F = SO_REUSEPORT
				serr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, 0x0F, 1)
			})
			return serr
		},
	}
	return lc.Listen(context.Background(), network, address)
}
