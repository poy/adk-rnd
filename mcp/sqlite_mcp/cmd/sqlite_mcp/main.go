package main

import (
	"flag"
	"log"

	"github.com/mark3labs/mcp-go/server"
	"github.com/poy/adk-rnd/mcp/sqlite_mcp/pkg/mcpserver"
)

var dataDir = flag.String("data-dir", "/tmp/sqlite_mcp", "The directory to store the data")

func main() {
	log.SetFlags(0)
	flag.Parse()
	srv := mcpserver.New(*dataDir)
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("failed to serve stdio: %v", err)
	}
}
