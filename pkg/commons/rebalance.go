package commons

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/andres-erbsen/clock"
	"github.com/cockroachdb/errors"
	"github.com/jmoiron/sqlx"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"

	"github.com/lncapital/torq/internal/database"
)

type Rebalancer struct {
	NodeId          int
	RebalanceId     int
	Status          Status
	CreatedOn       time.Time
	UpdateOn        time.Time
	GlobalCtx       context.Context
	RebalanceCtx    context.Context
	RebalanceCancel context.CancelFunc
	Runners         map[int]*RebalanceRunner
	Request         RebalanceRequest
}

type RebalanceRunner struct {
	RebalanceId       int
	OutgoingChannelId int
	IncomingChannelId int
	Invoices          map[uint64]*lnrpc.AddInvoiceResponse
	// FailedHops map[hopSourcePublicKey_hopDestinationPublicKey]amountMsat
	FailedHops  map[string]uint64
	FailedPairs []*lnrpc.NodePair
	Status      Status
	Ctx         context.Context
	Cancel      context.CancelFunc
}

func (runner *RebalanceRunner) addFailedHop(hopSourcePublicKey string, hopDestinationPublicKey string, amountMsat uint64) {
	runner.FailedHops[hopSourcePublicKey+"_"+hopDestinationPublicKey] = amountMsat
}

func (runner *RebalanceRunner) isFailedHop(hopSourcePublicKey string, hopDestinationPublicKey string, amountMsat uint64) bool {
	failedHopAmountMsat, exists := runner.FailedHops[hopSourcePublicKey+"_"+hopDestinationPublicKey]
	return exists &&
		getDeltaPerMille(failedHopAmountMsat, amountMsat) <
			REBALANCE_ROUTE_FAILED_HOP_ALLOWED_DELTA_PER_MILLE
}

type RebalanceResult struct {
	RebalanceId       int       `json:"rebalanceId"`
	OutgoingChannelId int       `json:"outgoingChannelId"`
	IncomingChannelId int       `json:"incomingChannelId"`
	Status            Status    `json:"status"`
	Hops              string    `json:"hops"`
	TotalTimeLock     uint32    `json:"total_time_lock"`
	TotalFeeMsat      uint64    `json:"total_fee_msat"`
	TotalAmountMsat   uint64    `json:"total_amount_msat"`
	Error             string    `json:"error"`
	CreatedOn         time.Time `json:"createdOn"`
	UpdateOn          time.Time `json:"updateOn"`

	Route *lnrpc.Route `json:"-"`
}

func RebalanceServiceStart(ctx context.Context, conn *grpc.ClientConn, db *sqlx.DB, nodeId int,
	rebalanceRequestChannel chan RebalanceRequest) {

	client := lnrpc.NewLightningClient(conn)
	router := routerrpc.NewRouterClient(conn)

	go initiateDelayedRebalancers(ctx, db, client, router)

	for {
		select {
		case <-ctx.Done():
			return
		case request := <-rebalanceRequestChannel:
			if request.NodeId != nodeId {
				continue
			}
			if request.RequestTime == nil {
				now := time.Now().UTC()
				request.RequestTime = &now
			}
			// Previous rebalance cleanup delay
			time.Sleep(time.Millisecond * REBALANCE_REBALANCE_DELAY_MILLISECONDS)
			processRebalanceRequest(ctx, db, request, nodeId)
		}
	}
}

func initiateDelayedRebalancers(ctx context.Context, db *sqlx.DB,
	client lnrpc.LightningClient, router routerrpc.RouterClient) {

	ticker := clock.New().Tick(REBALANCE_QUEUE_TICKER_SECONDS * time.Second)
	pending := Pending
	active := Active

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker:
			activeRebalancers := getRebalancers(&active)
			if len(activeRebalancers) > REBALANCE_MAXIMUM_CONCURRENCY {
				continue
			}

			pendingRebalancers := getRebalancers(&pending)
			if len(pendingRebalancers) > 0 {
				sort.Slice(pendingRebalancers, func(i, j int) bool {
					return pendingRebalancers[i].CreatedOn.Before(pendingRebalancers[j].CreatedOn)
				})

				if pendingRebalancers[0].CreatedOn.Before(time.Now()) {
					go pendingRebalancers[0].start(db, client, router,
						REBALANCE_RUNNER_TIMEOUT_SECONDS,
						REBALANCE_ROUTES_TIMEOUT_SECONDS,
						REBALANCE_ROUTE_TIMEOUT_SECONDS)
				}
			}
		}
	}
}

func processRebalanceRequest(ctx context.Context, db *sqlx.DB, request RebalanceRequest, nodeId int) {
	response := validateRebalanceRequest(request)
	if response != nil {
		sendResponse(request, *response)
		return
	}

	response = updateExistingRebalanceRequest(db, request)
	if response != nil {
		sendResponse(request, *response)
		return
	}

	pending := Pending
	pendingRebalancers := getRebalancers(&pending)

	var filteredChannelIds []int
	if request.IncomingChannelId != 0 {
		for _, channelId := range request.ChannelIds {
			// get rebalance attempts for the other direction
			latestResult := getLatestResult(channelId, request.IncomingChannelId, nil)
			if latestResult.RebalanceId == 0 || latestResult.UpdateOn.Before(time.Now().Add(-5*time.Minute)) {
				filteredChannelIds = rebalancePendingForOpositDirection(pendingRebalancers, channelId, request.IncomingChannelId, channelId, filteredChannelIds)
			} else {
				log.Info().Msgf(
					"ChannelId %d was removed because an opposite result already exists (IncomingChannelId: %d) "+
						"for origin: %v, originId: %v with reference number: %v",
					channelId, request.IncomingChannelId, request.Origin, request.OriginId, request.OriginReference)
			}
		}
	}
	if request.OutgoingChannelId != 0 {
		for _, channelId := range request.ChannelIds {
			// get rebalance attempts for the other direction
			latestResult := getLatestResult(request.OutgoingChannelId, channelId, nil)
			if latestResult.RebalanceId == 0 || latestResult.UpdateOn.Before(time.Now().Add(-5*time.Minute)) {
				filteredChannelIds = rebalancePendingForOpositDirection(pendingRebalancers, channelId, channelId, request.OutgoingChannelId, filteredChannelIds)
			} else {
				log.Info().Msgf(
					"ChannelId %d was removed because an opposite result already exists (OutgoingChannelId: %d) "+
						"for origin: %v, originId: %v with reference number: %v",
					channelId, request.OutgoingChannelId, request.Origin, request.OriginId, request.OriginReference)
			}
		}
	}
	if len(filteredChannelIds) == 0 {
		sendResponse(request, RebalanceResponse{
			Request: request,
			CommunicationResponse: CommunicationResponse{
				Status: Inactive,
				Error:  "No channelIds found after filtering based on historic records",
			},
		})
		return
	}
	request.ChannelIds = filteredChannelIds

	createdOn := time.Now().UTC()

	latestResult := getLatestResultByOrigin(request.Origin, request.OriginId, request.IncomingChannelId, request.OutgoingChannelId, nil)
	if latestResult.RebalanceId != 0 {
		runningFor := request.RequestTime.Sub(latestResult.UpdateOn)
		if runningFor.Seconds() < REBALANCE_MINIMUM_DELTA_SECONDS {
			sleepTime := REBALANCE_MINIMUM_DELTA_SECONDS*time.Second - runningFor
			createdOn.Add(sleepTime)
		}
	}

	rebalancer := &Rebalancer{
		NodeId:    nodeId,
		CreatedOn: createdOn,
		UpdateOn:  createdOn,
		GlobalCtx: ctx,
		Runners:   make(map[int]*RebalanceRunner),
		Request:   request,
		Status:    Pending,
	}
	rebalancerCtx, rebalancerCancel := context.WithTimeout(rebalancer.GlobalCtx,
		time.Second*time.Duration(REBALANCE_TIMEOUT_SECONDS))
	rebalancer.RebalanceCtx = rebalancerCtx
	rebalancer.RebalanceCancel = rebalancerCancel
	if !addRebalancer(rebalancer) {
		sendResponse(request, RebalanceResponse{
			Request: request,
			CommunicationResponse: CommunicationResponse{
				Status: Inactive,
				Error: fmt.Sprintf(
					"IncomingChannelId: %v already has a running rebalancer for origin: %v, originId: %v with reference number: %v",
					rebalancer.Request.IncomingChannelId, rebalancer.Request.Origin, rebalancer.Request.OriginId, rebalancer.Request.OriginReference),
			},
		})
		return
	}
	sendResponse(request, RebalanceResponse{
		Request: request,
		CommunicationResponse: CommunicationResponse{
			Status: Active,
		},
	})
}

func rebalancePendingForOpositDirection(pendingRebalancers []*Rebalancer, channelId int, incomingChannelId int, outgoingChannelId int, filteredChannelIds []int) []int {
	if len(pendingRebalancers) == 0 {
		return append(filteredChannelIds, channelId)
	}
	for _, rebalancer := range pendingRebalancers {
		if rebalancer.Request.IncomingChannelId == outgoingChannelId {
			for _, rebalanceOutgoingChannelId := range rebalancer.Request.ChannelIds {
				if rebalanceOutgoingChannelId == incomingChannelId {
					return filteredChannelIds
				}
			}
		}
		if rebalancer.Request.OutgoingChannelId == incomingChannelId {
			for _, rebalanceIncomingChannelId := range rebalancer.Request.ChannelIds {
				if rebalanceIncomingChannelId == outgoingChannelId {
					return filteredChannelIds
				}
			}
		}
	}
	return append(filteredChannelIds, channelId)
}

func (rebalancer *Rebalancer) start(
	db *sqlx.DB,
	client lnrpc.LightningClient,
	router routerrpc.RouterClient,
	runnerTimeout int,
	routesTimeout int,
	routeTimeout int) {

	if rebalancer.Request.IncomingChannelId != 0 {
		incomingChannel := GetChannelSettingByChannelId(rebalancer.Request.IncomingChannelId)
		if incomingChannel.Capacity == 0 || incomingChannel.Status != Open {
			log.Error().Msgf("IncomingChannelId is invalid for origin: %v, originReference: %v and incomingChannelId: %v",
				rebalancer.Request.Origin, rebalancer.Request.OriginReference, rebalancer.Request.IncomingChannelId)
			rebalancer.RebalanceCancel()
			return
		}
	}
	if rebalancer.Request.OutgoingChannelId != 0 {
		outgoingChannel := GetChannelSettingByChannelId(rebalancer.Request.OutgoingChannelId)
		if outgoingChannel.Capacity == 0 || outgoingChannel.Status != Open {
			log.Error().Msgf("OutgoingChannelId is invalid for origin: %v, originReference: %v and outgoingChannelId: %v",
				rebalancer.Request.Origin, rebalancer.Request.OriginReference, rebalancer.Request.OutgoingChannelId)
			rebalancer.RebalanceCancel()
			return
		}
	}

	active := Active
	rebalancer.Status = Active
	previousSuccess := getLatestResultByOrigin(rebalancer.Request.Origin, rebalancer.Request.OriginId,
		rebalancer.Request.IncomingChannelId, rebalancer.Request.OutgoingChannelId, &active)
	if time.Since(previousSuccess.UpdateOn).Seconds() > REBALANCE_SUCCESS_TIMEOUT_SECONDS {
		previousSuccess = RebalanceResult{}
	}

	err := AddRebalanceAndChannels(db, rebalancer)
	if err != nil {
		log.Error().Err(err).Msgf("Storing rebalance for origin: %v, originReference: %v and incomingChannelId: %v",
			rebalancer.Request.Origin, rebalancer.Request.OriginReference, rebalancer.Request.IncomingChannelId)
		rebalancer.RebalanceCancel()
		return
	}

	if previousSuccess.Hops != "" && previousSuccess.Route != nil {
		dummyRunner := &RebalanceRunner{
			RebalanceId:       rebalancer.RebalanceId,
			OutgoingChannelId: previousSuccess.OutgoingChannelId,
			IncomingChannelId: previousSuccess.IncomingChannelId,
			Invoices:          make(map[uint64]*lnrpc.AddInvoiceResponse),
			FailedHops:        make(map[string]uint64),
			Status:            Active,
		}
		if rebalancer.Request.IncomingChannelId != 0 {
			// When incoming channel is provided then the runners loop over the outgoing channels
			rebalancer.Runners[previousSuccess.OutgoingChannelId] = dummyRunner
		} else {
			rebalancer.Runners[previousSuccess.IncomingChannelId] = dummyRunner
		}
		result := rebalancer.rerunPreviousSuccess(client, router, dummyRunner, previousSuccess.Route, routeTimeout)
		if result.Status == Active {
			removeRebalancer(rebalancer)
		}
		rebalancer.processResult(db, result)
		if result.Status == Active {
			return
		}
		if rebalancer.Request.IncomingChannelId != 0 {
			// When incoming channel is provided then the runners loop over the outgoing channels
			delete(rebalancer.Runners, previousSuccess.OutgoingChannelId)
		} else {
			delete(rebalancer.Runners, previousSuccess.IncomingChannelId)
		}
	}
	for i := 0; i < rebalancer.Request.MaximumConcurrency; i++ {
		go rebalancer.createRunner(db, client, router, runnerTimeout, routesTimeout, routeTimeout)
	}
}

func (rebalancer *Rebalancer) createRunner(
	db *sqlx.DB,
	client lnrpc.LightningClient,
	router routerrpc.RouterClient,
	runnerTimeout int,
	routesTimeout int,
	routeTimeout int) {

	if rebalancer.Status == Inactive {
		return
	}

	result := RebalanceResult{
		Status:            Initializing,
		RebalanceId:       rebalancer.RebalanceId,
		CreatedOn:         time.Now().UTC(),
		IncomingChannelId: rebalancer.Request.IncomingChannelId,
		OutgoingChannelId: rebalancer.Request.OutgoingChannelId,
	}
	channelId := rebalancer.getPendingChannelId()
	if channelId == 0 {
		for _, runner := range rebalancer.Runners {
			if runner.Status == Active {
				return
			}
		}
		removeRebalancer(rebalancer)
		rebalancer.RebalanceCancel()
		runningFor := time.Since(rebalancer.CreatedOn).Round(1 * time.Second)
		log.Info().Msgf("Pending ChannelId got exhausted for Origin: %v, OriginId: %v (%s)",
			rebalancer.Request.Origin, rebalancer.Request.OriginId, runningFor)
		if runningFor.Seconds() < REBALANCE_MINIMUM_DELTA_SECONDS {
			sleepTime := REBALANCE_MINIMUM_DELTA_SECONDS*time.Second - runningFor
			rebalancer.CreatedOn = time.Now().UTC()
			rebalancer.CreatedOn = rebalancer.CreatedOn.Add(sleepTime)
		}
		rebalancer.Runners = make(map[int]*RebalanceRunner)
		rebalancer.Status = Pending
		if !addRebalancer(rebalancer) {
			log.Error().Msgf("Failed to reschedule the rebalancer for Origin: %v, OriginId: %v",
				rebalancer.Request.Origin, rebalancer.Request.OriginId)
		}
		return
	}

	runnerCtx, runnerCancel := context.WithTimeout(rebalancer.RebalanceCtx, time.Second*time.Duration(runnerTimeout))
	defer runnerCancel()
	runner := rebalancer.addRunner(channelId, runnerCtx, runnerCancel)

	result.IncomingChannelId = runner.IncomingChannelId
	result.OutgoingChannelId = runner.OutgoingChannelId

	result = rebalancer.startRunner(db, client, router, runner, routesTimeout, routeTimeout, result)
	if result.Status == Active {
		removeRebalancer(rebalancer)
		rebalancer.RebalanceCancel()
		runningFor := time.Since(rebalancer.CreatedOn).Round(1 * time.Second)
		log.Info().Msgf("Successfully rebalanced for Origin: %v, OriginId: %v (%s)",
			rebalancer.Request.Origin, rebalancer.Request.OriginId, runningFor)
		rebalancer.Status = Inactive
	} else {
		runner.Cancel()
		runner.Status = Inactive

		rebalancer.createRunner(db, client, router, runnerTimeout, routesTimeout, routeTimeout)
	}
}

func (rebalancer *Rebalancer) startRunner(
	db *sqlx.DB,
	client lnrpc.LightningClient,
	router routerrpc.RouterClient,
	runner *RebalanceRunner,
	routesTimeout int,
	routeTimeout int,
	result RebalanceResult) RebalanceResult {

	routesCtx, routesCancel := context.WithTimeout(runner.Ctx, time.Second*time.Duration(routesTimeout))
	defer routesCancel()
	routes, err := runner.getRoutes(routesCtx, client, router, rebalancer.NodeId,
		rebalancer.Request.AmountMsat, rebalancer.Request.MaximumCostMsat)
	if err != nil {
		result.Status = Inactive
		result.Error = err.Error()
		if routesCtx.Err() == context.DeadlineExceeded {
			result.Error = routesCtx.Err().Error()
		}
		rebalancer.processResult(db, result)
	}
	routesCancel()

	for _, route := range routes {
		routeCtx, routeCancel := context.WithTimeout(runner.Ctx, time.Second*time.Duration(routeTimeout))
		result = runner.pay(routeCtx, client, router, rebalancer.Request.AmountMsat, route)
		routeCancel()
		if routeCtx.Err() == context.DeadlineExceeded {
			result.Error = routeCtx.Err().Error()
		}
		rebalancer.processResult(db, result)
		if result.Status == Active {
			return result
		}
	}

	if result.Status == Pending {
		result = rebalancer.startRunner(db, client, router, runner, routesTimeout, routeTimeout, result)
	}
	return result
}

func (rebalancer *Rebalancer) getPendingChannelId() int {
outer:
	for _, channelId := range rebalancer.Request.ChannelIds {
		for existingChannelId := range rebalancer.Runners {
			if existingChannelId == channelId {
				continue outer
			}
		}
		channelSettings := GetChannelSettingByChannelId(channelId)
		if channelSettings.Capacity == 0 || channelSettings.Status != Open {
			continue outer
		}
		return channelId
	}
	return 0
}

func (rebalancer *Rebalancer) rerunPreviousSuccess(
	client lnrpc.LightningClient,
	router routerrpc.RouterClient,
	runner *RebalanceRunner,
	route *lnrpc.Route,
	routeTimeout int) RebalanceResult {

	routeCtx, routeCancel := context.WithTimeout(rebalancer.RebalanceCtx, time.Second*time.Duration(routeTimeout))
	defer routeCancel()
	return runner.pay(routeCtx, client, router, rebalancer.Request.AmountMsat, route)
}

func (rebalancer *Rebalancer) addRunner(channelId int, runnerCtx context.Context, runnerCancel context.CancelFunc) *RebalanceRunner {
	runner := RebalanceRunner{
		RebalanceId: rebalancer.RebalanceId,
		Invoices:    make(map[uint64]*lnrpc.AddInvoiceResponse),
		FailedHops:  make(map[string]uint64),
		Ctx:         runnerCtx,
		Cancel:      runnerCancel,
		Status:      Active,
	}

	if rebalancer.Request.IncomingChannelId != 0 {
		runner.IncomingChannelId = rebalancer.Request.IncomingChannelId
		runner.OutgoingChannelId = channelId
	} else {
		runner.IncomingChannelId = channelId
		runner.OutgoingChannelId = rebalancer.Request.OutgoingChannelId
	}

	rebalancer.Runners[channelId] = &runner
	return &runner
}

func (rebalancer *Rebalancer) processResult(db *sqlx.DB, result RebalanceResult) {
	result.UpdateOn = time.Now().UTC()
	addRebalanceResult(rebalancer.Request.Origin, rebalancer.Request.OriginId,
		rebalancer.Request.IncomingChannelId, rebalancer.Request.OutgoingChannelId, result)
	err := AddRebalanceLog(db, result)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to add rebalance log entry for rebalanceId: %v", rebalancer.RebalanceId)
	}
}

func (runner *RebalanceRunner) getRoutes(
	ctx context.Context,
	client lnrpc.LightningClient,
	router routerrpc.RouterClient,
	nodeId int,
	amountMsat uint64,
	fixedFeeMsat uint64) ([]*lnrpc.Route, error) {

	outgoingChannel := GetChannelSettingByChannelId(runner.OutgoingChannelId)
	incomingChannel := GetChannelSettingByChannelId(runner.IncomingChannelId)
	var remoteNode ManagedNodeSettings
	if outgoingChannel.FirstNodeId == nodeId {
		remoteNode = GetNodeSettingsByNodeId(incomingChannel.SecondNodeId)
	} else {
		remoteNode = GetNodeSettingsByNodeId(incomingChannel.FirstNodeId)
	}
	remoteNodePublicKey, err := hex.DecodeString(remoteNode.PublicKey)
	if err != nil {
		return nil, errors.Wrapf(err, "Decoding public key for outgoing nodeId: %v", outgoingChannel.SecondNodeId)
	}

	routes, err := client.QueryRoutes(ctx, &lnrpc.QueryRoutesRequest{
		PubKey:            GetNodeSettingsByNodeId(nodeId).PublicKey,
		OutgoingChanId:    outgoingChannel.LndShortChannelId,
		LastHopPubkey:     remoteNodePublicKey,
		AmtMsat:           int64(amountMsat),
		UseMissionControl: true,
		FeeLimit:          &lnrpc.FeeLimit{Limit: &lnrpc.FeeLimit_FixedMsat{FixedMsat: int64(fixedFeeMsat)}},
		IgnoredPairs:      runner.FailedPairs,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "QueryRoutes for outgoing nodeId: %v, publicKey: %v", outgoingChannel.FirstNodeId, remoteNode.PublicKey)
	}

	var result []*lnrpc.Route
	for i := range routes.Routes {
		if runner.validateRoute(nodeId, routes.Routes[i]) {
			result = append(result, routes.Routes[i])
		}
	}
	if len(result) == 0 {
		return runner.getRoutes(ctx, client, router, nodeId, amountMsat, fixedFeeMsat)
	}
	return result, nil
}

func (runner *RebalanceRunner) pay(
	ctx context.Context,
	client lnrpc.LightningClient,
	router routerrpc.RouterClient,
	amountMsat uint64,
	route *lnrpc.Route) RebalanceResult {

	rebalanceResult := RebalanceResult{
		OutgoingChannelId: runner.OutgoingChannelId,
		IncomingChannelId: runner.IncomingChannelId,
		RebalanceId:       runner.RebalanceId,
		CreatedOn:         time.Now().UTC(),
		Status:            Inactive,
	}

	invoice, err := runner.createInvoice(ctx, client, amountMsat)
	if err != nil {
		rebalanceResult.Error = err.Error()
		return rebalanceResult
	}
	lastHop := route.Hops[len(route.Hops)-1]
	lastHop.MppRecord = &lnrpc.MPPRecord{
		PaymentAddr:  invoice.PaymentAddr,
		TotalAmtMsat: int64(amountMsat),
	}

	result, err := router.SendToRouteV2(ctx,
		&routerrpc.SendToRouteRequest{
			PaymentHash: invoice.RHash,
			Route:       route,
		})
	if result.Route != nil {
		rebalanceResult.Route = result.Route
		rebalanceResult.TotalFeeMsat = uint64(result.Route.TotalFeesMsat)
		rebalanceResult.TotalTimeLock = result.Route.TotalTimeLock
		rebalanceResult.TotalAmountMsat = uint64(result.Route.TotalAmtMsat)
	}
	if err != nil {
		rebalanceResult.Error = err.Error()
		return rebalanceResult
	}
	if result.Status == lnrpc.HTLCAttempt_FAILED {
		rebalanceResult.Status = Inactive
		if result.Failure.FailureSourceIndex >= uint32(len(route.Hops)) {
			rebalanceResult.Error = fmt.Sprintf("%s unknown hop index: %d. Maximum hop index: %d",
				result.Failure.Code.String(), result.Failure.FailureSourceIndex, len(route.Hops))
			return rebalanceResult
		}
		if result.Failure.FailureSourceIndex == 0 {
			rebalanceResult.Error = fmt.Sprintf("%s unknown hop index %d. Minimum hop index is greater than 0",
				result.Failure.Code.String(), result.Failure.FailureSourceIndex)
			return rebalanceResult
		}
		prevHop := route.Hops[result.Failure.FailureSourceIndex-1]
		failedHop := route.Hops[result.Failure.FailureSourceIndex]
		if result.Failure.Code == lnrpc.Failure_TEMPORARY_CHANNEL_FAILURE {
			rebalanceResult.Status = Pending
			runner.addFailedHop(prevHop.PubKey, failedHop.PubKey, uint64(prevHop.AmtToForwardMsat))
		}
		rebalanceResult.Error = fmt.Sprintf("error: %s occured at hop index %d (%v -> %v)",
			result.Failure.Code.String(), result.Failure.FailureSourceIndex, prevHop.PubKey, failedHop.PubKey)
		return rebalanceResult
	}
	delete(runner.Invoices, amountMsat)
	rebalanceResult.Status = Active
	return rebalanceResult
}

func (runner *RebalanceRunner) validateRoute(nodeId int, route *lnrpc.Route) bool {
	previousHopPublicKey := GetNodeSettingsByNodeId(nodeId).PublicKey
	for _, h := range route.Hops {
		if runner.isFailedHop(previousHopPublicKey, h.PubKey, uint64(h.AmtToForwardMsat)) {
			from, err := hex.DecodeString(previousHopPublicKey)
			if err != nil {
				return false
			}
			to, err := hex.DecodeString(h.PubKey)
			if err != nil {
				return false
			}
			runner.FailedPairs = append(runner.FailedPairs, &lnrpc.NodePair{From: from, To: to})
			return false
		}
		previousHopPublicKey = h.PubKey
	}
	return true
}

func (runner *RebalanceRunner) createInvoice(
	ctx context.Context,
	client lnrpc.LightningClient,
	amountMsat uint64) (*lnrpc.AddInvoiceResponse, error) {

	invoice, exists := runner.Invoices[amountMsat]
	if exists {
		return invoice, nil
	}
	invoice, err := client.AddInvoice(ctx, &lnrpc.Invoice{ValueMsat: int64(amountMsat),
		Memo:   "Rebalance attempt",
		Expiry: int64(REBALANCE_TIMEOUT_SECONDS)})
	if err != nil {
		return nil, errors.Wrapf(err, "AddInvoice for %v msat", amountMsat)
	}
	runner.Invoices[amountMsat] = invoice
	return invoice, nil
}

func sendResponse(request RebalanceRequest, response RebalanceResponse) {
	if request.ResponseChannel != nil {
		request.ResponseChannel <- response
	}
}

func validateRebalanceRequest(request RebalanceRequest) *RebalanceResponse {
	if request.IncomingChannelId == 0 && request.OutgoingChannelId == 0 {
		return &RebalanceResponse{
			Request: request,
			CommunicationResponse: CommunicationResponse{
				Status: Inactive,
				Error:  "IncomingChannelId and OutgoingChannelId are 0",
			},
		}
	}
	if request.IncomingChannelId != 0 && request.OutgoingChannelId != 0 {
		return &RebalanceResponse{
			Request: request,
			CommunicationResponse: CommunicationResponse{
				Status: Inactive,
				Error:  "IncomingChannelId and OutgoingChannelId are populated",
			},
		}
	}

	if request.IncomingChannelId != 0 {
		incomingChannel := GetChannelSettingByChannelId(request.IncomingChannelId)
		if incomingChannel.Capacity == 0 || incomingChannel.Status != Open {
			return &RebalanceResponse{
				Request: request,
				CommunicationResponse: CommunicationResponse{
					Status: Inactive,
					Error:  "IncomingChannelId is invalid",
				},
			}
		}
		if slices.Contains(request.ChannelIds, request.IncomingChannelId) {
			return &RebalanceResponse{
				Request: request,
				CommunicationResponse: CommunicationResponse{
					Status: Inactive,
					Error:  "ChannelIds also contain IncomingChannelId",
				},
			}
		}
	}
	if request.OutgoingChannelId != 0 {
		outgoingChannel := GetChannelSettingByChannelId(request.OutgoingChannelId)
		if outgoingChannel.Capacity == 0 || outgoingChannel.Status != Open {
			return &RebalanceResponse{
				Request: request,
				CommunicationResponse: CommunicationResponse{
					Status: Inactive,
					Error:  "OutgoingChannelId is invalid",
				},
			}
		}
		if slices.Contains(request.ChannelIds, request.OutgoingChannelId) {
			return &RebalanceResponse{
				Request: request,
				CommunicationResponse: CommunicationResponse{
					Status: Inactive,
					Error:  "ChannelIds also contain OutgoingChannelId",
				},
			}
		}
	}

	response := verifyNotZeroInt(request, int64(request.MaximumConcurrency), "MaximumConcurrency")
	if response != nil {
		return response
	}
	response = verifyNotZeroInt(request, int64(request.OriginId), "OriginId")
	if response != nil {
		return response
	}
	response = verifyNotZeroUint(request, request.AmountMsat, "AmountMsat")
	if response != nil {
		return response
	}
	response = verifyNotZeroUint(request, request.MaximumCostMsat, "MaximumCostMsat")
	if response != nil {
		return response
	}
	if len(request.ChannelIds) == 0 {
		return &RebalanceResponse{
			Request: request,
			CommunicationResponse: CommunicationResponse{
				Status: Inactive,
				Error:  "ChannelIds are not specified",
			},
		}
	}

	for _, channelId := range request.ChannelIds {
		channelSettings := GetChannelSettingByChannelId(channelId)
		if channelSettings.Capacity == 0 || channelSettings.Status != Open {
			return &RebalanceResponse{
				Request: request,
				CommunicationResponse: CommunicationResponse{
					Status: Inactive,
					Error:  "ChannelIds contain an invalid channelId",
				},
			}
		}
	}
	return nil
}

func verifyNotZeroUint(request RebalanceRequest, value uint64, label string) *RebalanceResponse {
	if value == 0 {
		return &RebalanceResponse{
			Request: request,
			CommunicationResponse: CommunicationResponse{
				Status: Inactive,
				Error:  label + " is 0",
			},
		}
	}
	return nil
}

func verifyNotZeroInt(request RebalanceRequest, value int64, label string) *RebalanceResponse {
	if value == 0 {
		return &RebalanceResponse{
			Request: request,
			CommunicationResponse: CommunicationResponse{
				Status: Inactive,
				Error:  label + " is 0",
			},
		}
	}
	return nil
}

func updateExistingRebalanceRequest(db *sqlx.DB, request RebalanceRequest) *RebalanceResponse {
	rebalancer := getRebalancer(request.Origin, request.OriginId)
	if rebalancer == nil {
		return nil
	}
	if request.RequestTime != nil && rebalancer.UpdateOn.After(*request.RequestTime) {
		return &RebalanceResponse{
			Request: request,
			CommunicationResponse: CommunicationResponse{
				Status: Inactive,
				Error: fmt.Sprintf(
					"IncomingChannelId: %v, OutgoingChannelId: %v already has a more recent running rebalancer for origin: %v with originId: %v (ref: %v)",
					rebalancer.Request.IncomingChannelId, rebalancer.Request.OutgoingChannelId, rebalancer.Request.Origin, rebalancer.Request.OriginId,
					rebalancer.Request.OriginReference),
			},
		}
	}
	if rebalancer.Request.IncomingChannelId != request.IncomingChannelId || rebalancer.Request.OutgoingChannelId != request.OutgoingChannelId {
		removeRebalancer(rebalancer)
		rebalancer.RebalanceCancel()
		return nil
	}
	var err error
	if rebalancer.Request.AmountMsat != request.AmountMsat {
		err = setRebalancer(db, request, rebalancer)
	} else if rebalancer.Request.MaximumCostMsat != request.MaximumCostMsat {
		err = setRebalancer(db, request, rebalancer)
	} else if rebalancer.Request.MaximumConcurrency != request.MaximumConcurrency {
		err = setRebalancer(db, request, rebalancer)
	} else if len(rebalancer.Request.ChannelIds) != len(request.ChannelIds) {
		err = setRebalancer(db, request, rebalancer)
	} else {
		for _, channelId := range rebalancer.Request.ChannelIds {
			if !slices.Contains(request.ChannelIds, channelId) {
				err = setRebalancer(db, request, rebalancer)
				break
			}
		}
	}
	if err != nil {
		return &RebalanceResponse{
			Request: request,
			CommunicationResponse: CommunicationResponse{
				Status: Inactive,
				Error: fmt.Sprintf(
					"(%v) for IncomingChannelId: %v, OutgoingChannelId: %v that already has a running rebalancer for origin: %v with originId: %v (ref: %v)",
					err.Error(), rebalancer.Request.IncomingChannelId, rebalancer.Request.OutgoingChannelId,
					rebalancer.Request.Origin, rebalancer.Request.OriginId, rebalancer.Request.OriginReference),
			},
		}
	}
	return &RebalanceResponse{
		Request: request,
		CommunicationResponse: CommunicationResponse{
			Status: Active,
			Message: fmt.Sprintf(
				"Updated existing rebalancer for origin: %v with originId: %v (ref: %v)",
				rebalancer.Request.Origin, rebalancer.Request.OriginId, rebalancer.Request.OriginReference),
		},
	}
}

func setRebalancer(db *sqlx.DB, request RebalanceRequest, rebalancer *Rebalancer) error {
	rebalancer.UpdateOn = time.Now().UTC()
	rebalancer.Request = request
	err := SetRebalanceAndChannels(db, *rebalancer)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to add rebalance log entry for rebalanceId: %v", rebalancer.RebalanceId)
	}
	return errors.Wrapf(err,
		"Updating the database with the new rebalance settings for origin: %v with originId: %v (ref: %v)",
		rebalancer.Request.Origin, rebalancer.Request.OriginId, rebalancer.Request.OriginReference)
}

func AddRebalanceAndChannels(db *sqlx.DB, rebalancer *Rebalancer) error {
	var incomingChannelId *int
	if rebalancer.Request.IncomingChannelId != 0 {
		incomingChannelId = &rebalancer.Request.IncomingChannelId
	}
	var outgoingChannelId *int
	if rebalancer.Request.OutgoingChannelId != 0 {
		outgoingChannelId = &rebalancer.Request.OutgoingChannelId
	}
	err := db.QueryRowx(`
			INSERT INTO rebalance (incoming_channel_id, outgoing_channel_id, status,
			                       origin, origin_id, origin_reference,
			                       amount_msat, maximum_concurrency, maximum_costmsat,
			                       created_on, updated_on)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING rebalance_id;`,
		incomingChannelId, outgoingChannelId, rebalancer.Status,
		rebalancer.Request.Origin, rebalancer.Request.OriginId, rebalancer.Request.OriginReference,
		rebalancer.Request.AmountMsat, rebalancer.Request.MaximumConcurrency, rebalancer.Request.MaximumCostMsat,
		rebalancer.CreatedOn, rebalancer.UpdateOn).
		Scan(&rebalancer.RebalanceId)
	if err != nil {
		return errors.Wrap(err, database.SqlExecutionError)
	}
	for _, rebalanceChannelId := range rebalancer.Request.ChannelIds {
		_, err = db.Exec(`
				INSERT INTO rebalance_channel (channel_id, status, rebalance_id, created_on, updated_on)
				VALUES ($1, $2, $3, $4, $5);`,
			rebalanceChannelId, Active, rebalancer.RebalanceId, rebalancer.CreatedOn, rebalancer.UpdateOn)
		if err != nil {
			return errors.Wrap(err, database.SqlExecutionError)
		}
	}
	return nil
}

func SetRebalanceAndChannels(db *sqlx.DB, rebalancer Rebalancer) error {
	tx := db.MustBegin()
	_, err := tx.Exec(`
			UPDATE rebalance
			SET origin_reference=$1, amount_msat=$2, maximum_concurrency=$3, maximum_costmsat=$4, updated_on=$5
			WHERE rebalance_id=$6;`,
		rebalancer.Request.OriginReference, rebalancer.Request.AmountMsat, rebalancer.Request.MaximumConcurrency, rebalancer.Request.MaximumCostMsat,
		rebalancer.UpdateOn, rebalancer.RebalanceId)
	if err != nil {
		if rb := tx.Rollback(); rb != nil {
			log.Error().Err(rb).Msg(database.SqlRollbackTransactionError)
		}
		return errors.Wrap(err, database.SqlExecutionError)
	}
	_, err = tx.Exec(`UPDATE rebalance_channel SET status=$1 WHERE rebalance_id=$2;`, Inactive, rebalancer.RebalanceId)
	if err != nil {
		if rb := tx.Rollback(); rb != nil {
			log.Error().Err(rb).Msg(database.SqlRollbackTransactionError)
		}
		return errors.Wrap(err, database.SqlExecutionError)
	}
	for _, rebalanceChannelId := range rebalancer.Request.ChannelIds {
		res, err := tx.Exec(`UPDATE rebalance_channel SET status=$1 WHERE rebalance_id=$2 AND channel_id=$3;`,
			Active, rebalancer.RebalanceId, rebalanceChannelId)
		if err != nil {
			if rb := tx.Rollback(); rb != nil {
				log.Error().Err(rb).Msg(database.SqlRollbackTransactionError)
			}
			return errors.Wrap(err, database.SqlExecutionError)
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			return errors.Wrap(err, database.SqlAffectedRowsCheckError)
		}
		if rowsAffected == 0 {
			_, err = db.Exec(`
				INSERT INTO rebalance_channel (channel_id, status, rebalance_id, created_on, updated_on)
				VALUES ($1, $2, $3, $4, $5);`,
				rebalanceChannelId, Active, rebalancer.RebalanceId, rebalancer.CreatedOn, rebalancer.UpdateOn)
			if err != nil {
				if rb := tx.Rollback(); rb != nil {
					log.Error().Err(rb).Msg(database.SqlRollbackTransactionError)
				}
				return errors.Wrap(err, database.SqlExecutionError)
			}
		}
	}
	err = tx.Commit()
	if err != nil {
		return errors.Wrap(err, database.SqlCommitTransactionError)
	}
	return nil
}

func AddRebalanceLog(db *sqlx.DB, rebalanceResult RebalanceResult) error {
	_, err := db.Exec(`INSERT INTO rebalance_log (incoming_channel_id, outgoing_channel_id, hops, status,
                           total_time_lock, total_fee_msat, total_amount_msat, error, rebalance_id, created_on, updated_on)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);`,
		rebalanceResult.IncomingChannelId, rebalanceResult.OutgoingChannelId, rebalanceResult.Hops, rebalanceResult.Status,
		rebalanceResult.TotalTimeLock, rebalanceResult.TotalFeeMsat, rebalanceResult.TotalAmountMsat,
		rebalanceResult.Error, rebalanceResult.RebalanceId, rebalanceResult.CreatedOn, rebalanceResult.UpdateOn)
	if err != nil {
		return errors.Wrap(err, database.SqlExecutionError)
	}
	return nil
}
