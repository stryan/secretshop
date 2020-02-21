package main

import (
	"fmt"
	"path"

	"github.com/prologic/go-gopher"
)

type GopherConfig struct {
	Hostname string
	Port     string
	RootDir  string
}

func (c *GopherConfig) String() string {
	return fmt.Sprintf("Gopher Config: %v:%v Files:%v", c.Hostname, c.Port, c.RootDir)
}

type indexHandler struct {
	rootPath    string
	rootHandler gopher.Handler
}

func (f *indexHandler) ServeGopher(w gopher.ResponseWriter, r *gopher.Request) {
	upath := r.Selector
	if gopher.GetItemType(f.rootPath+upath) == gopher.DIRECTORY && upath != "/" {
		w.WriteItem(&gopher.Item{
			Type:        gopher.DIRECTORY,
			Selector:    path.Dir(upath),
			Description: "Go Back",
		})
	}
	f.rootHandler.ServeGopher(w, r)
}

func index(root gopher.FileSystem) *indexHandler {
	return &indexHandler{root.Name(), gopher.FileServer(root)}
}
