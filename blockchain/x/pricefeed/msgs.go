package pricefeed

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// TypeMsgPostPrice type of PostPrice msg
	TypeMsgPostPrice = "post_price"
)

// MsgPostPrice struct representing a posted price message.
// Used by oracles to input prices to the pricefeed
type MsgPostPrice struct {
	From      sdk.AccAddress // client that sent in this address
	AssetCode string         // asset code used by exchanges/api
	Price     sdk.Dec        // price in decimal (max precision 18)
	Expiry    sdk.Int        // block height
}

// NewMsgPostPrice creates a new post price msg
func NewMsgPostPrice(
	from sdk.AccAddress,
	assetCode string,
	price sdk.Dec,
	expiry sdk.Int) MsgPostPrice {
	return MsgPostPrice{
		From:      from,
		AssetCode: assetCode,
		Price:     price,
		Expiry:    expiry,
	}
}

// Route Implements Msg.
func (msg MsgPostPrice) Route() string { return RouterKey }

// Type Implements Msg
func (msg MsgPostPrice) Type() string { return TypeMsgPostPrice }

// ValidateBasic Implements Msg.
func (msg MsgPostPrice) ValidateBasic() sdk.Error {
	return nil
}

// GetSignBytes Implements Msg.
func (msg MsgPostPrice) GetSignBytes() []byte {
	bz := msgCdc.MustMarshalJSON(msg) // TODO define msgCdc in codec.go as they seem to do in gov module
	return sdk.MustSortJSON(bz)
}

// GetSigners Implements Msg.
func (msg MsgPostPrice) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.From}
}
