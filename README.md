# socks5proxy

Un proxy SOCKS5 en Go avec rotation d'adresses IPv6 aléatoires à partir de blocs multiples non contigus.  
Compatible avec l'authentification utilisateur et conçu pour gérer efficacement des milliers de connexions simultanées.

---

## ✨ Fonctionnalités

- 🎯 Rotation d'adresses IPv6 aléatoires dans des subnets configurés
- 🔐 Authentification SOCKS5 par login/mot de passe
- ⚙️ Lecture des blocs IPv6 depuis un fichier JSON (`subnets.json`)
- 🧠 Ajout automatique des routes `local` pour rendre toutes les IP bindables
- 🖥️ Configurable facilement et déployable rapidement

---

## 🔧 Prérequis

- **Go 1.22 ou supérieur**
- Système Linux avec support IPv6 complet
- Droits `sudo` pour ajouter des routes au démarrage
- Activation de l'option `net.ipv6.ip_nonlocal_bind=1` dans `sysctl`

---

## 🔒 Configuration système requise

Pour que le proxy puisse binder des adresses IPv6 non assignées à l'interface, il faut activer :
```bash
sudo sysctl -w net.ipv6.ip_nonlocal_bind=1
```
Pour le rendre permanent : 
```bash
echo 'net.ipv6.ip_nonlocal_bind=1' | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```


## 📦 Installation

### 1. Installer Go

```bash
cd /tmp
curl -LO https://go.dev/dl/go1.24.2.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.24.2.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

Vérifie :

```bash
go version
```

---

### 2. Cloner le repo et installer les dépendances

```bash
git clone https://github.com/lekr74/go-rotating-proxy.git
cd socks5proxy
go mod tidy
go get github.com/fsnotify/fsnotify
```

---

## 🚀 Lancer le proxy

```bash
go run .
```

Par défaut, le proxy écoute sur le port `1080`.

---

## ⚙️ Configuration

### `users.yaml` – utilisateurs autorisés

```yaml
users:
  user1: pass1
  user2: pass2
```

---

### `subnets.json` – blocs IPv6 disponibles

```json
{
  "subnets": [
    "2a01:cb15:2c4:f080::/57",
    "2a0a:6044:b600::/40"
  ]
}
```

---

## 🧠 Fonctionnement

Au démarrage, le proxy :
- Lit les blocs IPv6 définis
- Ajoute une route `local` pour chacun (`ip -6 route add local ... dev enp6s18`)
- Génère une adresse IPv6 aléatoire dans l’un des blocs à chaque requête sortante
- Établit la connexion avec cette IP comme source

---

## 🛠️ Personnalisation

- L'interface réseau par défaut est `enp6s18` → modifiable dans `main.go` ou via une variable d'env (prochaines étapes).
- Le port d'écoute est `1080` → modifiable dans `main.go`

---

## ⚡ Installation automatisée (recommandée)

Tu peux installer le proxy et tout son environnement automatiquement avec ce script :

```bash
curl -fsSL https://raw.githubusercontent.com/lekr74/go-rotating-proxy/refs/heads/main/install_socks5proxy.sh | bash
```

> 🔒 **Remarque** : le script requiert `sudo` pour :
> - Installer Go dans `/usr/local`
> - Ajouter des routes IPv6 locales
> - Créer un service `systemd`
> - Activer `net.ipv6.ip_nonlocal_bind=1`

---

📦 Le script fait tout pour toi :
- Télécharge et installe Go 1.24.2
- Clone ce repo dans `/opt/socks5proxy`
- Installe les dépendances Go (`fsnotify`, etc.)
- Compile le binaire
- Crée le service `systemd`
- Active les options réseau nécessaires

---

💡 Une fois installé :
- Place tes fichiers `users.yaml` et `subnets.json` dans `/opt/socks5proxy`
- Le proxy démarre automatiquement au boot

--- 

## 🔐 Sécurité

Ce proxy **ne doit pas être exposé publiquement** sans contrôle d'accès (auth SOCKS5 activée par défaut).  
Les IPs bindées ne sont pas assignées à l'interface, elles sont rendues bindables grâce aux routes `local`.

---

## 📜 Licence

MIT – utilise [armon/go-socks5](https://github.com/armon/go-socks5)
