/**
*  @file
*  @copyright defined in go-seele/LICENSE
 */

package node

import (
	"github.com/seeleteam/go-seele/p2p"
	"github.com/seeleteam/go-seele/seele"
)

// Config holds Node options.
type Config struct {
	// The name of the node
	Name string

	// The version of the node
	Version string

	// The file system folder of the node use to store data
	DataDir string

	// The configuration of p2p network
	P2P p2p.Config

	// The RPCAddr is the address on which to start RPC server.
	RPCAddr string

	// The HTTPAddr is the address of HTTP rpc service
	HTTPAddr string

	// HTTPCors is the Cross-Origin Resource Sharing header to send to requesting
	// clients. Please be aware that CORS is a browser enforced security, it's fully
	// useless for custom HTTP clients.
	HTTPCors []string

	// HTTPHostFilter is the whitelist of hostnames which are allowed on incoming requests.
	HTTPWhiteHost []string

	// The SeeleConfig is the configuration to create seele service.
	SeeleConfig seele.Config
}
