// Copyright (C) by Ubaldo Porcheddu <ubaldo@eja.it>

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/eja/maps"
)

const Name = "maps"
const Label = "Maps"
const Version = "0.1.12"

var (
	isLog    = flag.Bool("log", false, "Enable logging")
	logFile  = flag.String("log-file", "", "Path to log file")
	webPort  = flag.Int("web-port", 35248, "Port to listen on")
	webHost  = flag.String("web-host", "localhost", "Host address to bind to")
	webPath  = flag.String("web-path", "/"+Name+"/", "HTTP URL path prefix")
	webAuth  = flag.String("web-auth", "", `HTTP Basic Auth as "user:password" or base64 string`)
	filePath = flag.String("file-path", ".", "Path to server file or directory")
)

func help() {
	fmt.Println("Copyright:", "2026 by Ubaldo Porcheddu <ubaldo@eja.it>")
	fmt.Println("Version:", Version)
	fmt.Printf("Usage: %s [options]\n", os.Args[0])
	fmt.Println("Options:\n")
	flag.PrintDefaults()
	fmt.Println()
}

func main() {
	flag.Usage = help
	flag.Parse()

	if !*isLog {
		log.SetOutput(io.Discard)
	} else if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}
		log.SetOutput(f)
	}

	if _, err := os.Stat(*filePath); os.IsNotExist(err) {
		log.Fatalf("Error: The file root directory '%s' does not exist.", *filePath)
	}

	if *webAuth != "" {
		log.Println("Basic Authentication enabled.")
	}

	addr := fmt.Sprintf("%s:%d", *webHost, *webPort)
	handler := maps.New(*webPath, *filePath, *webAuth)

	mux := http.NewServeMux()
	mux.Handle(*webPath, handler)

	log.Printf("Serving %s on http://%s%s\n", Label, addr, *webPath)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
