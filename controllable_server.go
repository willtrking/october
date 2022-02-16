package october

import (
	"context"
	"net/http"
)

type ControllableServer interface {

	Name() string

	// Start the server. Expected to block.
	// Returns boolean indicating if the server was shutdown from a call to Shutdown, and an error
	Start() (bool, error)

	Shutdown(ctx context.Context) error
}


type ControllableHttpServer struct {
	Server *http.Server
	ServerName string
}

func (c *ControllableHttpServer) Name() string {
	return c.ServerName
}

func (c *ControllableHttpServer) Start() (bool, error) {

	err := c.Server.ListenAndServe()

	return err == http.ErrServerClosed, err
}

func (c *ControllableHttpServer) Shutdown(ctx context.Context) error {
	return c.Server.Shutdown(ctx)
}


// Create a ControllableHttpServer from a http.Server and a name
// Expects the listening address to be set under http.Server.Addr
func ControlHttpServer(server *http.Server, name string ) ControllableServer {
	return &ControllableHttpServer{
		Server:server,
		ServerName:name,
	}
}