package lnd

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/chainrpc"
	"github.com/rs/zerolog/log"

	"github.com/lncapital/torq/pkg/commons"
)

type Tx struct {
	Timestamp             time.Time `json:"timestamp" db:"timestamp"`
	TransactionHash       *string   `json:"transactionHash" db:"tx_hash"`
	Amount                *int64    `json:"amount" db:"amount"`
	NumberOfConfirmations *int32    `json:"numberOfConfirmations" db:"num_confirmations"`
	BlockHash             *string   `json:"blockHash" db:"block_hash"`
	BlockHeight           *int32    `json:"blockHeight" db:"block_height"`
	TotalFees             *int64    `json:"totalFees" db:"total_fees"`
	DestinationAddresses  *[]string `json:"destinationAddresses" db:"dest_addresses"`
	RawTransactionHex     *string   `json:"rawTransactionHex" db:"raw_tx_hex"`
	Label                 *string   `json:"label" db:"label"`
	NodeId                int       `json:"nodeId" db:"node_id"`
}

func fetchLastTxHeight(db *sqlx.DB, nodeId int) (txHeight int32, err error) {

	sqlLatest := `select coalesce(max(block_height),1) from tx where node_id = $1;`

	row := db.QueryRow(sqlLatest, nodeId)
	err = row.Scan(&txHeight)

	if err != nil {
		return 1, errors.Wrap(err, "SQL row scan for tx height")
	}

	return txHeight, nil
}

// SubscribeAndStoreTransactions Subscribes to on-chain transaction events from LND and stores them in the
// database as a time series. It will also import unregistered transactions on startup.
func SubscribeAndStoreTransactions(ctx context.Context, client lnrpc.LightningClient, chain chainrpc.ChainNotifierClient, db *sqlx.DB,
	nodeSettings commons.ManagedNodeSettings, eventChannel chan interface{}) {
	var transactionHeight int32
	var err error
	var transactionDetails *lnrpc.TransactionDetails
	var storedTx Tx
	var stream chainrpc.ChainNotifier_RegisterBlockEpochNtfnClient
	var blockEpoch *chainrpc.BlockEpoch
	serviceStatus := commons.Inactive
	bootStrapping := true
	subscriptionStream := commons.TransactionStream

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if stream == nil {
			serviceStatus = SendStreamEvent(eventChannel, nodeSettings.NodeId, subscriptionStream, commons.Pending, serviceStatus)
			transactionHeight, err = fetchLastTxHeight(db, nodeSettings.NodeId)
			if err != nil {
				if errors.Is(ctx.Err(), context.Canceled) {
					return
				}
				log.Error().Err(err).Msgf("Failed to obtain last know transaction, will retry in %v seconds", commons.STREAM_ERROR_SLEEP_SECONDS)
				time.Sleep(commons.STREAM_ERROR_SLEEP_SECONDS * time.Second)
				continue
			}

			// transactionHeight + 1: otherwise that last transaction will be downloaded over-and-over.
			transactionDetails, err = client.GetTransactions(ctx, &lnrpc.GetTransactionsRequest{
				StartHeight: transactionHeight + 1,
			})
			if err != nil {
				if errors.Is(ctx.Err(), context.Canceled) {
					return
				}
				log.Error().Err(err).Msgf("Failed to obtain last transaction details, will retry in %v seconds", commons.STREAM_ERROR_SLEEP_SECONDS)
				time.Sleep(commons.STREAM_ERROR_SLEEP_SECONDS * time.Second)
				continue
			}
			stream, err = chain.RegisterBlockEpochNtfn(ctx, &chainrpc.BlockEpoch{Height: uint32(transactionHeight + 1)})
			if err != nil {
				log.Error().Err(err).Msgf("Obtaining stream (RegisterBlockEpochNtfn) from LND failed, will retry in %v seconds", commons.STREAM_ERROR_SLEEP_SECONDS)
				stream = nil
				time.Sleep(commons.STREAM_ERROR_SLEEP_SECONDS * time.Second)
				continue
			}
		} else {
			bootStrapping = false
			blockEpoch, err = stream.Recv()
			if err != nil {
				if errors.Is(ctx.Err(), context.Canceled) {
					return
				}
				serviceStatus = SendStreamEvent(eventChannel, nodeSettings.NodeId, subscriptionStream, commons.Pending, serviceStatus)
				log.Error().Err(err).Msgf("Receiving block epoch from the stream failed, will retry in %v seconds", commons.STREAM_ERROR_SLEEP_SECONDS)
				stream = nil
				time.Sleep(commons.STREAM_ERROR_SLEEP_SECONDS * time.Second)
				continue
			}
			if eventChannel != nil {
				eventChannel <- commons.BlockEvent{
					EventData: commons.EventData{
						EventTime: time.Now().UTC(),
						NodeId:    nodeSettings.NodeId,
					},
					Hash:   blockEpoch.Hash,
					Height: blockEpoch.Height,
				}
			}
			// transactionHeight + 1: otherwise that last transaction will be downloaded over-and-over.
			transactionDetails, err = client.GetTransactions(ctx, &lnrpc.GetTransactionsRequest{
				StartHeight: transactionHeight + 1,
			})
			if err != nil {
				if errors.Is(ctx.Err(), context.Canceled) {
					return
				}
				serviceStatus = SendStreamEvent(eventChannel, nodeSettings.NodeId, subscriptionStream, commons.Pending, serviceStatus)
				log.Error().Err(err).Msgf("Failed to obtain last transaction details, will retry in %v seconds", commons.STREAM_ERROR_SLEEP_SECONDS)
				stream = nil
				time.Sleep(commons.STREAM_ERROR_SLEEP_SECONDS * time.Second)
				continue
			}
		}

		if bootStrapping {
			serviceStatus = SendStreamEvent(eventChannel, nodeSettings.NodeId, subscriptionStream, commons.Initializing, serviceStatus)
		} else {
			serviceStatus = SendStreamEvent(eventChannel, nodeSettings.NodeId, subscriptionStream, commons.Active, serviceStatus)
		}
		for _, transaction := range transactionDetails.Transactions {
			storedTx, err = storeTransaction(db, transaction, nodeSettings.NodeId)
			if err != nil {
				// TODO FIXME This transaction is now missing
				log.Error().Err(err).Msg("Failed to store the transaction (transaction is now missing and can only be recovered by emptying the transactions table)")
			}
			if eventChannel != nil && !bootStrapping {
				eventChannel <- commons.TransactionEvent{
					EventData: commons.EventData{
						EventTime: time.Now().UTC(),
						NodeId:    nodeSettings.NodeId,
					},
					Timestamp:             storedTx.Timestamp,
					TransactionHash:       storedTx.TransactionHash,
					Amount:                storedTx.Amount,
					NumberOfConfirmations: storedTx.NumberOfConfirmations,
					BlockHash:             storedTx.BlockHash,
					BlockHeight:           storedTx.BlockHeight,
					TotalFees:             storedTx.TotalFees,
					DestinationAddresses:  storedTx.DestinationAddresses,
					RawTransactionHex:     storedTx.RawTransactionHex,
					Label:                 storedTx.Label,
				}
			}
			if *storedTx.BlockHeight > transactionHeight {
				transactionHeight = *storedTx.BlockHeight
			}
		}
	}
}

func storeTransaction(db *sqlx.DB, tx *lnrpc.Transaction, nodeId int) (Tx, error) {
	if tx == nil {
		return Tx{}, nil
	}

	// Here we're only storing the output addresses, not the output index, amount or if these
	// transactions are ours or not. We might want to add this.
	var destinationAddresses []string
	for _, output := range tx.OutputDetails {
		destinationAddresses = append(destinationAddresses, output.Address)
	}

	storedTx := Tx{
		Timestamp:             time.Unix(tx.TimeStamp, 0).UTC(),
		TransactionHash:       &tx.TxHash,
		Amount:                &tx.Amount,
		NumberOfConfirmations: &tx.NumConfirmations,
		BlockHash:             &tx.BlockHash,
		BlockHeight:           &tx.BlockHeight,
		TotalFees:             &tx.TotalFees,
		DestinationAddresses:  &destinationAddresses,
		RawTransactionHex:     &tx.RawTxHex,
		Label:                 &tx.Label,
		NodeId:                nodeId,
	}

	var insertTx = `INSERT INTO tx (timestamp, tx_hash, amount, num_confirmations, block_hash, block_height,
                total_fees, dest_addresses, raw_tx_hex, label, node_id)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
                ON CONFLICT (timestamp, tx_hash) DO NOTHING;`

	_, err := db.Exec(insertTx,
		time.Unix(tx.TimeStamp, 0).UTC(),
		tx.TxHash,
		tx.Amount,
		tx.NumConfirmations,
		tx.BlockHash,
		tx.BlockHeight,
		tx.TotalFees,
		pq.Array(destinationAddresses),
		tx.RawTxHex,
		tx.Label,
		nodeId,
	)

	if err != nil {
		return Tx{}, errors.Wrapf(err, `inserting transaction`)
	}

	return storedTx, nil
}
