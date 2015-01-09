package chess

// Represents a move from a square to a square and possibly the promotion piece
// type.
//
// Castling moves are identified only by the movement of the king.
//
// Null moves are supported.
type Move struct {
	fromSquare int
	toSquare   int
	promotion  PieceTypes
}

func NewMove(fromSquare, toSquare int, promotion PieceTypes) *Move {
	return &Move{fromSquare, toSquare, promotion}
}

func (m *Move) Equals(move *Move) bool {
    return m.fromSquare == move.fromSquare && m.toSquare == move.toSquare &&
        m.promotion == move.promotion
}

func (m *Move) String() string {
    return SquareNames[m.fromSquare]+"->"+SquareNames[m.toSquare]
}

// Gets an UCI string for the move.
//
// For example a move from A7 to A8 would be `a7a8` or `a7a8q` if it is
// a promotion to a queen. The UCI representatin of null moves is `0000`.
func (m *Move) Uci() string {
	if m != nil {
		return SquareNames[m.fromSquare] + SquareNames[m.toSquare] + PieceSymbols[m.promotion]
	}
	return "0000"
}

// Parses an UCI string.
//
// Returns nil if the UCI string is invalid.
func MoveFromUci(uci string) *Move {
	if uci == "0000" {
		return nil
	} else if len(uci) == 4 {
		var fromSquare, toSquare int
		for i := range SquareNames {
			if SquareNames[i] == uci[0:2] {
				fromSquare = i
			}
			if SquareNames[i] == uci[2:4] {
				toSquare = i
			}
		}
		return NewMove(fromSquare, toSquare, None)
	} else if len(uci) == 5 {
		var fromSquare, toSquare int
		var promotion PieceTypes
		for i := range SquareNames {
			if SquareNames[i] == uci[0:2] {
				fromSquare = i
			}
			if SquareNames[i] == uci[2:4] {
				toSquare = i
			}
		}
		for pieceType, pieceSymbol := range PieceSymbols {
			if string(uci[4]) == pieceSymbol {
				promotion = PieceTypes(pieceType)
				break
			}
		}
		return NewMove(fromSquare, toSquare, promotion)
	}
	return nil
}

// Gets a null move.
//
// A null move just passes the turn to the other side (and possibly
// forfeits en-passant capturing).
func NullMove() *Move {
	return nil
}
