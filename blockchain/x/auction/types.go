package auction

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	maxAuctionDuration endTime = 2 * 24 * 3600 / 5 // roughly 2 days, at 5s block time
	bidDuration        endTime = 3 * 3600 / 5      // roughly 3 hours, at 5s block time TODO better name

)

// Auction is an interface to several types of auction.
type Auction interface {
	GetID() auctionID
	SetID(auctionID)
	PlaceBid(currentBlockHeight endTime, bidder sdk.AccAddress, lot sdk.Coin, bid sdk.Coin) ([]bankOutput, []bankInput, sdk.Error)
	GetEndTime() endTime
	HasEnded(currentBlockHeight endTime) bool // TODO needed?
	GetPayout() []bankInput
}
type baseAuction struct {
	ID         auctionID
	Initiator  sdk.AccAddress // Person who starts the auction. Giving away Lot (aka seller in a forward auction)
	Lot        sdk.Coin       // Amount of coins up being given by initiator (FA - amount for sale by seller, RA - cost of good by buyer (bid))
	Bidder     sdk.AccAddress // Person who bids in the auction. Receiver of Lot. (aka buyer in forward auction, seller in RA)
	Bid        sdk.Coin       // Amount of coins being given by the bidder (FA - bid, RA - amount being sold)
	EndTime    endTime        // TODO check if an auction is closed on or after this specified block height
	MaxEndTime endTime        // closing time
}

type auctionID uint64 // copied from how the gov module IDs its proposals
type endTime int64    // type of BlockHeight TODO does it help to have this as it's own type?
// Initially the input and output types from the bank module where used here. But they use sdk.Coins instad of sdk.Coin. So it caused a lot of type conversion as auction mainly uses sdk.Coin.
type bankInput struct {
	Address sdk.AccAddress
	Coin    sdk.Coin
}
type bankOutput struct {
	Address sdk.AccAddress
	Coin    sdk.Coin
}

func (a baseAuction) GetID() auctionID    { return a.ID }
func (a baseAuction) SetID(id auctionID)  { a.ID = id }
func (a baseAuction) GetEndTime() endTime { return a.EndTime }

func (a baseAuction) GetPayout() []bankInput {
	return []bankInput{{a.Bidder, a.Lot}}
}
func (a baseAuction) HasEnded(currentBlockHeight endTime) bool {
	return currentBlockHeight > a.EndTime // > or ≥ ?
}

type ForwardAuction struct {
	baseAuction
}

func NewForwardAuction(seller sdk.AccAddress, lot sdk.Coin, initialBid sdk.Coin, endTime endTime) (ForwardAuction, []bankOutput) {
	auction := ForwardAuction{baseAuction{
		// no ID
		Initiator:  seller,
		Lot:        lot,
		Bidder:     seller,     // send the proceeds from the first bid back to the seller
		Bid:        initialBid, // set this to zero most of the time
		EndTime:    endTime,
		MaxEndTime: endTime,
	}}
	outputs := []bankOutput{{seller, lot}}
	return auction, outputs
}
func (a ForwardAuction) PlaceBid(currentBlockHeight endTime, bidder sdk.AccAddress, lot sdk.Coin, bid sdk.Coin) ([]bankOutput, []bankInput, sdk.Error) {
	// check lot size matches lot?
	// check is has not closed?
	// check bid is greater than last bid
	if !bid.IsGTE(a.Bid) { // TODO this should be just GT. TODO add minimum bid size
		return []bankOutput{}, []bankInput{}, sdk.ErrInternal("bid not greater than last bid")
	}
	// calculate coin movements
	outputs := []bankOutput{{bidder, bid}}                                    // new bidder pays bid now
	inputs := []bankInput{{a.Bidder, a.Bid}, {a.Initiator, bid.Minus(a.Bid)}} // old bidder is paid back, extra goes to seller

	// update auction
	a.Bidder = bidder
	a.Bid = bid
	// increment timeout // TODO into keeper?
	a.EndTime = endTime(min(int64(currentBlockHeight+bidDuration), int64(a.MaxEndTime))) // TODO is there a better way to structure these types?

	return outputs, inputs, nil
}

type ReverseAuction struct {
	baseAuction
}

func NewReverseAuction(buyer sdk.AccAddress, bid sdk.Coin, initialLot sdk.Coin, endTime endTime) (ReverseAuction, []bankOutput) {
	auction := ReverseAuction{baseAuction{
		// no ID
		Initiator:  buyer,
		Lot:        initialLot,
		Bidder:     buyer, // send proceeds from the first bid to the buyer
		Bid:        bid,   // amount that the buyer it buying - doesn't change over course of auction
		EndTime:    endTime,
		MaxEndTime: endTime,
	}}
	outputs := []bankOutput{{buyer, initialLot}}
	return auction, outputs
}
func (a ReverseAuction) PlaceBid(currentBlockHeight endTime, bidder sdk.AccAddress, lot sdk.Coin, bid sdk.Coin) ([]bankOutput, []bankInput, sdk.Error) {

	// check bid size matches bid?
	// check is has not closed?
	// check bid is less than last bid
	if !lot.IsLT(a.Lot) { // TODO add min bid decrements
		return []bankOutput{}, []bankInput{}, sdk.ErrInternal("lot not smaller than last lot")
	}
	// calculate coin movements
	outputs := []bankOutput{{bidder, a.Bid}}                                  // new bidder pays bid now}
	inputs := []bankInput{{a.Bidder, a.Bid}, {a.Initiator, a.Lot.Minus(lot)}} // old bidder is paid back, decrease in price for goes to buyer

	// update auction
	a.Bidder = bidder
	a.Lot = lot
	// increment timeout // TODO into keeper?
	a.EndTime = endTime(min(int64(currentBlockHeight+bidDuration), int64(a.MaxEndTime))) // TODO is there a better way to structure these types?

	return outputs, inputs, nil
}

type ForwardReverseAuction struct {
	baseAuction
	MaxBid      sdk.Coin
	OtherPerson sdk.AccAddress // TODO rename
	// 	// MaxBid only needs to be "set" when you want a flip auction. Otherwise it should be "infinity", or the max possible value.
	// 	// sdk.NewInt(2 ^ 255 - 1) // maximum size a sdk.Int can be, according to cosmos-sdk/types/int.go
	// 	MaxBid sdk.Coins // TODO shouldn't be coins type, TODO any way to make this optional?
}

func NewForwardReverseAuction(seller sdk.AccAddress, lot sdk.Coin, initialBid sdk.Coin, endTime endTime, maxBid sdk.Coin, otherPerson sdk.AccAddress) (ForwardReverseAuction, []bankOutput) {
	auction := ForwardReverseAuction{
		baseAuction: baseAuction{
			// no ID
			Initiator:  seller,
			Lot:        lot,
			Bidder:     seller,     // send the proceeds from the first bid back to the seller
			Bid:        initialBid, // 0 most of the time
			EndTime:    endTime,
			MaxEndTime: endTime},
		MaxBid:      maxBid,
		OtherPerson: otherPerson,
	}
	outputs := []bankOutput{{seller, lot}}
	return auction, outputs
}

func (a ForwardReverseAuction) PlaceBid(currentBlockHeight endTime, bidder sdk.AccAddress, lot sdk.Coin, bid sdk.Coin) (outputs []bankOutput, inputs []bankInput, err sdk.Error) {
	// determine phase of auction
	switch {
	case a.Bid.IsLT(a.MaxBid) && bid.IsLT(a.MaxBid):
		// Forward auction phase
		if !bid.IsGTE(a.Bid) { // TODO This should be just GT. TOOadd min bid increments
			return []bankOutput{}, []bankInput{}, sdk.ErrInternal("bid not greater than last bid")
		}
		outputs = []bankOutput{{bidder, bid}}                                    // new bidder pays bid now
		inputs = []bankInput{{a.Bidder, a.Bid}, {a.Initiator, bid.Minus(a.Bid)}} // old bidder is paid back, extra goes to seller
	case a.Bid.IsLT(a.MaxBid):
		// Switch over phase
		// require bid == a.MaxBid
		if !bid.IsEqual(a.MaxBid) {
			return []bankOutput{}, []bankInput{}, sdk.ErrInternal("bid greater than the max bid")
		}
		outputs = []bankOutput{{bidder, bid}} // new bidder pays bid now
		inputs = []bankInput{
			{a.Bidder, a.Bid},                 // old bidder is paid back
			{a.Initiator, bid.Minus(a.Bid)},   // extra goes to seller
			{a.OtherPerson, a.Lot.Minus(lot)}, //decrease in price for goes to original CDP owner
		}

	case a.Bid.IsEqual(a.MaxBid):
		// Reverse auction phase
		if !lot.IsLT(a.Lot) { // TODO add min bid decrements
			return []bankOutput{}, []bankInput{}, sdk.ErrInternal("lot not smaller than last lot")
		}
		outputs = []bankOutput{{bidder, a.Bid}}                                    // new bidder pays bid now
		inputs = []bankInput{{a.Bidder, a.Bid}, {a.OtherPerson, a.Lot.Minus(lot)}} // old bidder is paid back, decrease in price for goes to original CDP owner
	default:
		panic("should never be reached") // TODO
	}

	// update auction
	a.Bidder = bidder
	a.Lot = lot
	a.Bid = bid
	// increment timeout
	a.EndTime = endTime(min(int64(currentBlockHeight+bidDuration), int64(a.MaxEndTime))) // TODO is there a better way to structure these types?

	return outputs, inputs, nil
}
