// Command career107 serves the Career Assessment 107 API and web UI.
//
//	career107 -addr :8080 -data ./data
package main

import (
	"flag"
	"log"
	"net/http"

	"career107/api"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	data := flag.String("data", "", "directory to save the session in (empty keeps it in memory)")
	flag.Parse()

	session, err := api.NewSession(*data)
	if err != nil {
		log.Fatalf("starting session: %v", err)
	}
	log.Printf("Career Assessment 107 listening on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, api.NewHandler(session)))
}
