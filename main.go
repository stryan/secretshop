package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/prologic/go-gopher"
	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigName("config")
	viper.AddConfigPath("/etc/secretshop/")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Fatal error config file: %v \n", err)
	}
	viper.SetConfigType("yaml")

	//Load configs
	active_capsules := viper.GetStringSlice("active_capsules")
	active_holes := viper.GetStringSlice("active_holes")
	capsule_list := make([]GeminiConfig, len(active_capsules))
	hole_list := make([]GopherConfig, len(active_holes))
	for i, c := range active_capsules {
		viper.UnmarshalKey(c, &(capsule_list[i]))
		log.Printf("Loading capsule %v %v", i, capsule_list[i].Hostname)
	}
	for i, h := range active_holes {
		viper.UnmarshalKey(h, &(hole_list[i]))
		log.Printf("Loading hole %v %v", i, hole_list[i].Hostname)
	}
	if len(capsule_list) < 1 && len(hole_list) < 1 {
		log.Println("No capsules or gopherholes loaded. Shutting down.")
		return
	}
	log.Printf("%v capsules loaded, %v holes loaded", len(capsule_list), len(hole_list))
	// Intialize servers
	wg := new(sync.WaitGroup)
	wg.Add(len(capsule_list) + len(hole_list))
	for i, c := range capsule_list {
		log.Printf("Starting capsule %v %v", i, c.Hostname)
		go func(c interface{}) {
			log.Fatal(ListenAndServeTLS(c.(GeminiConfig)))
			wg.Done()
		}(c)
	}
	for i, h := range hole_list {
		log.Printf("Starting gopherhole %v %v", i, h.Hostname)
		go func(h interface{}) {
			hole := h.(GopherConfig)
			gopher.Handle("/", gopher.FileServer(gopher.Dir(hole.RootDir)))
			log.Fatal(gopher.ListenAndServe(hole.Hostname+":"+hole.Port, nil))
			wg.Done()
		}(h)
	}
	wg.Wait()
}

type GeminiConfig struct {
	Hostname string
	Port     string
	KeyFile  string
	CertFile string
	RootDir  string
	CGIDir   string
}

type GopherConfig struct {
	Hostname string
	Port     string
	RootDir  string
}

func (c *GeminiConfig) String() string {
	return fmt.Sprintf("Gemini Config: %v:%v Files:%v CGI:%v", c.Hostname, c.Port, c.RootDir, c.CGIDir)
}
func (c *GopherConfig) String() string {
	return fmt.Sprintf("Gopher Config: %v:%v Files:%v", c.Hostname, c.Port, c.RootDir)
}
