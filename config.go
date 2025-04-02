// config.go
package main

import (
        "fmt"
        "io/ioutil"

        "gopkg.in/yaml.v2"
)

type Config struct {
        Users map[string]string `yaml:"users"`
}

func LoadConfig(path string) (*Config, error) {
        data, err := ioutil.ReadFile(path)
        if err != nil {
                return nil, fmt.Errorf("erreur lecture fichier config: %w", err)
        }

        var conf Config
        err = yaml.Unmarshal(data, &conf)
        if err != nil {
                return nil, fmt.Errorf("erreur parsing YAML: %w", err)
        }

        return &conf, nil
}
