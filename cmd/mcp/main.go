package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/meschbach/mcp-imap/pkg/mcpimap"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/sys/unix"
)

func main() {
	procCtx, done := signal.NotifyContext(context.Background(), unix.SIGTERM, unix.SIGINT, unix.SIGSTOP)
	defer done()

	ctx := procCtx

	//setup server
	mailbox := os.Getenv("MCP_MAILBOX")
	host := os.Getenv("MCP_HOST")
	password := os.Getenv("MCP_PASSWORD")
	imapClient := mcpimap.NewEmersionImap(mailbox, host, password)
	defer func() {
		shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		err := imapClient.Close(shutdown)
		if err != nil {
			fmt.Printf("WARNING: Failed to close imap client: %v\n", err)
		}
	}()

	//clientTransport, serverTransport := mcp.NewInMemoryTransports()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "mcp-imap",
		Version: "v0.0.1",
	}, &mcp.ServerOptions{
		Instructions: "An IMAP client is attached via mcp with the protocol `mcp-imap`.",
	})

	b := mcpimap.NewIMAPClient(imapClient)
	b.Attach(server)

	serverTransport := &mcp.LoggingTransport{
		Transport: &mcp.StdioTransport{},
		Writer:    os.Stderr,
	}
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		log.Fatal(err)
	}

	<-procCtx.Done()

	serverSession.Wait()
}
