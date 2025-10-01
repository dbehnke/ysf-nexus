package bridge

import (
	"context"
	"time"
)

// BridgeType represents the type of bridge connection
type BridgeType string

const (
	BridgeTypeYSF BridgeType = "ysf"
	BridgeTypeDMR BridgeType = "dmr"
)

// BridgeInterface defines the common interface for all bridge types
type BridgeInterface interface {
	// Lifecycle management
	RunPermanent(ctx context.Context)
	RunScheduled(ctx context.Context, duration time.Duration)
	Disconnect() error

	// Status and information
	GetStatus() BridgeStatus
	GetName() string
	GetType() BridgeType
	GetState() BridgeState

	// Packet routing
	HandlePacket(data []byte, addr string) error
}

// BridgeFactory creates bridge instances based on configuration
type BridgeFactory interface {
	CreateBridge(cfg interface{}, server NetworkServer, logger interface{}) (BridgeInterface, error)
}
