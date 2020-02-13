package main

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigName("config")
	viper.AddConfigPath("/etc/secretshop/")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil { // Handle errors reading the config file
		log.Fatalf("Fatal error config file: %v \n", err)
	}
	viper.SetConfigType("yaml")

	//Load config
	active_capsules := viper.GetStringSlice("active_capsules")
	capsule_list := make([]Config, len(active_capsules))
	for i, c := range active_capsules {
		viper.UnmarshalKey(c, &(capsule_list[i]))
	}
	if len(capsule_list) < 1 {
		log.Println("No capsules defined. Shutting down.")
		return
	}
	log.Fatal(ListenAndServeTLS(capsule_list[0]))
}

type Config struct {
	Hostname string
	Port     string
	KeyFile  string
	CertFile string
	RootDir  string
	CGIDir   string
}

func (c *Config) String() string {
	return fmt.Sprintf("Config: %v:%v Files:%v CGI:%v", c.Hostname, c.Port, c.RootDir, c.CGIDir)
}
