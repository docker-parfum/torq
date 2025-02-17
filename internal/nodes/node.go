package nodes

import (
	"time"

	"github.com/lncapital/torq/pkg/commons"
)

type Node struct {
	NodeId    int             `json:"nodeId" db:"node_id"`
	PublicKey string          `json:"publicKey" db:"public_key"`
	Chain     commons.Chain   `json:"chain" db:"chain"`
	Network   commons.Network `json:"network" db:"network"`
	CreatedOn time.Time       `json:"createdOn" db:"created_on"`
	// Will never be updated so no UpdatedOn...
}
