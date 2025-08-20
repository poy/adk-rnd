package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(0)

	addr := flag.String("addr", ":8888", "address to listen on (e.g. :8888 or 127.0.0.1:9000)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <-addr=:8888> [UPSTREAM_MCP_PATH] <UPSTREAM_MCP_ARGS...>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}
	upstreamPath := flag.Arg(0)

	var args []string
	if len(os.Args) > 1 {
		args = append(args, os.Args[2:]...)
	}

	// Start upstream MCP over stdio.
	mcpClient, err := client.NewStdioMCPClient(upstreamPath, nil, args...)
	if err != nil {
		log.Fatalf("failed to start upstream: %v", err)
	}
	defer func() { _ = mcpClient.Close() }()

	// Mirror upstream stderr verbatim to our stderr.
	if r, ok := client.GetStderr(mcpClient); ok && r != nil {
		go mirrorStderr("upstream", r)
	}

	// Mirror upstream stderr to our stderr.
	if r, ok := client.GetStderr(mcpClient); ok && r != nil {
		go mirrorStderr("upstream", r)
	}

	// Initialize and list tools from upstream.
	ctx := context.Background()
	if _, err := mcpClient.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
		log.Fatalf("initialize failed: %v", err)
	}

	list, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		log.Fatalf("tools/list failed: %v", err)
	}

	// Create our local MCP server that proxies tools to the upstream.
	srv := server.NewMCPServer("http-stdio-proxy", "1.0.0")

	for _, t := range list.Tools {
		tool := t // capture
		srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			res, err := mcpClient.CallTool(ctx, req)

			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("forward error: %v", err)), nil
			}

			return res, nil
		})
		log.Printf("registered proxy tool: %s", tool.Name)
	}

	// Spin up HTTP server that speaks the MCP streaming protocol.
	handler := server.NewStreamableHTTPServer(srv, server.WithHeartbeatInterval(time.Second))
	httpSrv := &http.Server{
		Addr:    *addr,
		Handler: handler,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	idleConnsClosed := make(chan struct{})
	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		<-sigc
		log.Printf("shutdown requested, closing HTTP server...")
		_ = httpSrv.Shutdown(context.Background())
		close(idleConnsClosed)
	}()

	log.Printf("serving HTTP on address %s", *addr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("failed to serve HTTP: %v", err)
	}
	<-idleConnsClosed
}

// mirrorStderr forwards upstream stderr to our stderr with a prefix.
func mirrorStderr(prefix string, r io.Reader) {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			os.Stderr.Write([]byte("[" + prefix + "-stderr] "))
			_, _ = os.Stderr.Write(buf[:n])
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("stderr mirror error: %v", err)
			}
			return
		}
	}
}
