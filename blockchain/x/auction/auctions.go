package auction

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	maxAuctionDuration endTime = 2 * 24 * 3600 / 5 // roughly 2 days, at 5s block time // 34560
	bidDuration        endTime = 3 * 3600 / 5      // roughly 3 hours, at 5s block time TODO better name // 2160
)

// Auction is an interface to several types of auction.
type Auction interface {
	GetID() auctionID
	SetID(auctionID)
	PlaceBid(currentBlockHeight endTime, bidder sdk.AccAddress, lot sdk.Coin, bid sdk.Coin) ([]bankOutput, []bankInput, sdk.Error)
	GetEndTime() endTime // auctions close at the end of the block with blockheight EndTime (ie bids placed in that block are valid)
	GetPayout() bankInput
}
type BaseAuction struct {
	ID         auctionID
	Initiator  sdk.AccAddress // Person who starts the auction. Giving away Lot (aka seller in a forward auction)
	Lot        sdk.Coin       // Amount of coins up being given by initiator (FA - amount for sale by seller, RA - cost of good by buyer (bid))
	Bidder     sdk.AccAddress // Person who bids in the auction. Receiver of Lot. (aka buyer in forward auction, seller in RA)
	Bid        sdk.Coin       // Amount of coins being given by the bidder (FA - bid, RA - amount being sold)
	EndTime    endTime        // Block height at which the auction closes. It closes at the end of this block
	MaxEndTime endTime        // Maximum closing time. Auctions can close before this but never after.
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

func (a BaseAuction) GetID() auctionID    { return a.ID }
func (a *BaseAuction) SetID(id auctionID) { a.ID = id }
func (a BaseAuction) GetEndTime() endTime { return a.EndTime }
func (a BaseAuction) GetPayout() bankInput {
	return bankInput{a.Bidder, a.Lot}
}

type ForwardAuction struct {
	BaseAuction
}

func NewForwardAuction(seller sdk.AccAddress, lot sdk.Coin, initialBid sdk.Coin, endTime endTime) (ForwardAuction, bankOutput) {
	auction := ForwardAuction{BaseAuction{
		// no ID
		Initiator:  seller,
		Lot:        lot,
		Bidder:     seller,     // send the proceeds from the first bid back to the seller
		Bid:        initialBid, // set this to zero most of the time
		EndTime:    endTime,
		MaxEndTime: endTime,
	}}
	output := bankOutput{seller, lot}
	return auction, output
}
func (a *ForwardAuction) PlaceBid(currentBlockHeight endTime, bidder sdk.AccAddress, lot sdk.Coin, bid sdk.Coin) ([]bankOutput, []bankInput, sdk.Error) {
	// TODO check lot size matches lot?
	// check auction has not closed
	if currentBlockHeight > a.EndTime {
		return []bankOutput{}, []bankInput{}, sdk.ErrInternal("auction has closed")
	}
	// check bid is greater than last bid
	if !a.Bid.IsLT(bid) { // TODO add minimum bid size
		return []bankOutput{}, []bankInput{}, sdk.ErrInternal("bid not greater than last bid")
	}
	// calculate coin movements
	outputs := []bankOutput{{bidder, bid}}                                  // new bidder pays bid now
	inputs := []bankInput{{a.Bidder, a.Bid}, {a.Initiator, bid.Sub(a.Bid)}} // old bidder is paid back, extra goes to seller

	// update auction
	a.Bidder = bidder
	a.Bid = bid
	// increment timeout // TODO into keeper?
	a.EndTime = endTime(min(int64(currentBlockHeight+bidDuration), int64(a.MaxEndTime))) // TODO is there a better way to structure these types?

	return outputs, inputs, nil
}

type ReverseAuction struct {
	BaseAuction
}

func NewReverseAuction(buyer sdk.AccAddress, bid sdk.Coin, initialLot sdk.Coin, endTime endTime) (ReverseAuction, bankOutput) {
	auction := ReverseAuction{BaseAuction{
		// no ID
		Initiator:  buyer,
		Lot:        initialLot,
		Bidder:     buyer, // send proceeds from the first bid to the buyer
		Bid:        bid,   // amount that the buyer it buying - doesn't change over course of auction
		EndTime:    endTime,
		MaxEndTime: endTime,
	}}
	output := bankOutput{buyer, initialLot}
	return auction, output
}
func (a *ReverseAuction) PlaceBid(currentBlockHeight endTime, bidder sdk.AccAddress, lot sdk.Coin, bid sdk.Coin) ([]bankOutput, []bankInput, sdk.Error) {

	// check bid size matches bid?
	// check auction has not closed
	if currentBlockHeight > a.EndTime {
		return []bankOutput{}, []bankInput{}, sdk.ErrInternal("auction has closed")
	}
	// check bid is less than last bid
	if !lot.IsLT(a.Lot) { // TODO add min bid decrements
		return []bankOutput{}, []bankInput{}, sdk.ErrInternal("lot not smaller than last lot")
	}
	// calculate coin movements
	outputs := []bankOutput{{bidder, a.Bid}}                                // new bidder pays bid now
	inputs := []bankInput{{a.Bidder, a.Bid}, {a.Initiator, a.Lot.Sub(lot)}} // old bidder is paid back, decrease in price for goes to buyer

	// update auction
	a.Bidder = bidder
	a.Lot = lot
	// increment timeout // TODO into keeper?
	a.EndTime = endTime(min(int64(currentBlockHeight+bidDuration), int64(a.MaxEndTime))) // TODO is there a better way to structure these types?

	return outputs, inputs, nil
}

type ForwardReverseAuction struct {
	BaseAuction
	MaxBid      sdk.Coin
	OtherPerson sdk.AccAddress // TODO rename, this is normally the original CDP owner
}

func NewForwardReverseAuction(seller sdk.AccAddress, lot sdk.Coin, initialBid sdk.Coin, endTime endTime, maxBid sdk.Coin, otherPerson sdk.AccAddress) (ForwardReverseAuction, bankOutput) {
	auction := ForwardReverseAuction{
		BaseAuction: BaseAuction{
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
	output := bankOutput{seller, lot}
	return auction, output
}

func (a *ForwardReverseAuction) PlaceBid(currentBlockHeight endTime, bidder sdk.AccAddress, lot sdk.Coin, bid sdk.Coin) (outputs []bankOutput, inputs []bankInput, err sdk.Error) {
	// check auction has not closed
	if currentBlockHeight > a.EndTime {
		return []bankOutput{}, []bankInput{}, sdk.ErrInternal("auction has closed")
	}

	// determine phase of auction
	switch {
	case a.Bid.IsLT(a.MaxBid) && bid.IsLT(a.MaxBid):
		// Forward auction phase
		if !a.Bid.IsLT(bid) { // TODO add min bid increments
			return []bankOutput{}, []bankInput{}, sdk.ErrInternal("bid not greater than last bid")
		}
		outputs = []bankOutput{{bidder, bid}}                                  // new bidder pays bid now
		inputs = []bankInput{{a.Bidder, a.Bid}, {a.Initiator, bid.Sub(a.Bid)}} // old bidder is paid back, extra goes to seller
	case a.Bid.IsLT(a.MaxBid):
		// Switch over phase
		if !bid.IsEqual(a.MaxBid) { // require bid == a.MaxBid
			return []bankOutput{}, []bankInput{}, sdk.ErrInternal("bid greater than the max bid")
		}
		outputs = []bankOutput{{bidder, bid}} // new bidder pays bid now
		inputs = []bankInput{
			{a.Bidder, a.Bid},               // old bidder is paid back
			{a.Initiator, bid.Sub(a.Bid)},   // extra goes to seller
			{a.OtherPerson, a.Lot.Sub(lot)}, //decrease in price for goes to original CDP owner
		}

	case a.Bid.IsEqual(a.MaxBid):
		// Reverse auction phase
		if !lot.IsLT(a.Lot) { // TODO add min bid decrements
			return []bankOutput{}, []bankInput{}, sdk.ErrInternal("lot not smaller than last lot")
		}
		outputs = []bankOutput{{bidder, a.Bid}}                                  // new bidder pays bid now
		inputs = []bankInput{{a.Bidder, a.Bid}, {a.OtherPerson, a.Lot.Sub(lot)}} // old bidder is paid back, decrease in price for goes to original CDP owner
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
