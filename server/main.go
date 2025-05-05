package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

var configFile = flag.String("config", "config.json", "config file")

func main() {
	config := NewDefaultConfig()
	if _, err := os.Stat(*configFile); err == nil {
		data, err := os.ReadFile(*configFile)
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal(data, config); err != nil {
			panic(err)
		}
	}
	data, _ := json.MarshalIndent(config, "", "  ")

	fmt.Printf("config:\n%s\n", data)

	server := NewServer(config)
	server.Run()
}
