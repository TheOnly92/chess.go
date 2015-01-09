package chess

import (
	"strings"
)

// A piece with type and color.
type Piece struct {
	pieceType PieceTypes
	color     Colors
}

func NewPiece(pieceType PieceTypes, color Colors) *Piece {
	return &Piece{pieceType, color}
}

// Gets the symbol `P`, `N`, `B`, `R`, `Q` or `K` for white pieces or the
// lower-case variants for the black pieces.
func (p *Piece) String() string {
	if p.color == White {
		return strings.ToUpper(PieceSymbols[p.pieceType])
	}
	return PieceSymbols[p.pieceType]
}

// Creates a piece instance from a piece symbol.
// Returns nil if the symbol is invalid.
func PieceFromSymbol(symbol string) *Piece {
	if strings.ToLower(symbol) == symbol {
		for pieceType, pieceSymbol := range PieceSymbols {
			if pieceSymbol == symbol {
				return NewPiece(PieceTypes(pieceType), Black)
			}
		}
	}
	for pieceType, pieceSymbol := range PieceSymbols {
		if pieceSymbol == strings.ToLower(symbol) {
			return NewPiece(PieceTypes(pieceType), White)
		}
	}

	return nil
}
