package chess

import (
	"fmt"
	"strconv"
	"strings"
)

// A bitboard and additional information representing a position.
//
// Provides move generation, validation, parsing, attack generation,
// game end detection, move counters and the capability to make and unmake
// moves.
//
// The bitboard is initialized to the starting position, unless otherwise
// specified in the optional `fen` argument.
type Bitboard struct {
	pawns   uint64
	knights uint64
	bishops uint64
	rooks   uint64
	queens  uint64
	kings   uint64

	occupiedCo  [2]uint64
	occupied    uint64
	occupiedL90 uint64
	occupiedL45 uint64
	occupiedR45 uint64

	kingSquares [2]int
	pieces      [64]PieceTypes

	epSquare       int
	castlingRights int
	turn           Colors
	fullMoveNumber int
	halfMoveClock  int

	halfMoveClockStack     *Stack
	capturedPieceStack     *Stack
	castlingRightStack     *Stack
	epSquareStack          *Stack
	moveStack              *Stack
	incrementalZobristHash uint64
	transpositions         map[uint64]int
}

func NewBitboard(fen string) *Bitboard {
	result := &Bitboard{}
	if fen == "" {
		result.Reset()
	} else {
		result.halfMoveClockStack = new(Stack)
		result.capturedPieceStack = new(Stack)
		result.castlingRightStack = new(Stack)
		result.epSquareStack = new(Stack)
		result.moveStack = new(Stack)
		result.transpositions = map[uint64]int{}
		result.SetFen(fen)
	}
	return result
}

func (b *Bitboard) GetPieces() [64]PieceTypes {
	return b.pieces
}

func (b *Bitboard) CheckSquareColor(square int) Colors {
    if b.occupiedCo[Black]&BBSquares[square] > 0 {
        return Black
    }
    return White
}

func (b *Bitboard) GetTurn() Colors {
	return b.turn
}

// Restores the starting position.
func (b *Bitboard) Reset() {
	b.pawns = BBRank2 | BBRank7
	b.knights = BBB1 | BBG1 | BBB8 | BBG8
	b.bishops = BBC1 | BBF1 | BBC8 | BBF8
	b.rooks = BBA1 | BBH1 | BBA8 | BBH8
	b.queens = BBD1 | BBD8
	b.kings = BBE1 | BBE8

	b.occupiedCo = [2]uint64{BBRank1 | BBRank2, BBRank7 | BBRank8}
	b.occupied = BBRank1 | BBRank2 | BBRank7 | BBRank8

	b.occupiedL90 = BBVoid
	b.occupiedL45 = BBVoid
	b.occupiedR45 = BBVoid

	b.kingSquares = [2]int{E1, E8}
	b.pieces = [64]PieceTypes{}

	for i := 0; i < 64; i++ {
		mask := BBSquares[i]
		if mask&b.pawns > 0 {
			b.pieces[i] = Pawn
		} else if mask&b.knights > 0 {
			b.pieces[i] = Knight
		} else if mask&b.bishops > 0 {
			b.pieces[i] = Bishop
		} else if mask&b.rooks > 0 {
			b.pieces[i] = Rook
		} else if mask&b.queens > 0 {
			b.pieces[i] = Queen
		} else if mask&b.kings > 0 {
			b.pieces[i] = King
		}
	}

	b.epSquare = 0
	b.castlingRights = Castling
	b.turn = White
	b.fullMoveNumber = 1
	b.halfMoveClock = 0

	for i := 0; i < 64; i++ {
		if BBSquares[i]&b.occupied > 0 {
			b.occupiedL90 |= BBSquaresL90[i]
			b.occupiedR45 |= BBSquaresR45[i]
			b.occupiedL45 |= BBSquaresL45[i]
		}
	}

	b.halfMoveClockStack = new(Stack)
	b.capturedPieceStack = new(Stack)
	b.castlingRightStack = new(Stack)
	b.epSquareStack = new(Stack)
	b.moveStack = new(Stack)
	b.incrementalZobristHash = b.BoardZobristHash(PolyglotRandomArray)
	b.transpositions = map[uint64]int{b.ZobristHash(nil): 1}
}

// Clears the board.
//
// Resets move stacks and move counters. The side to move is white. There
// are no rooks or kings, so castling is not allowed.
//
// In order to be in a valid `status()` at least kings need to be put on
// the board. This is required for move generation and validation to work
// properly.
func (b *Bitboard) Clear() {
	b.pawns = BBVoid
	b.knights = BBVoid
	b.bishops = BBVoid
	b.rooks = BBVoid
	b.queens = BBVoid
	b.kings = BBVoid

	b.occupiedCo = [2]uint64{BBVoid, BBVoid}
	b.occupied = BBVoid

	b.occupiedL90 = BBVoid
	b.occupiedR45 = BBVoid
	b.occupiedL45 = BBVoid

	b.kingSquares = [2]int{E1, E8}
	for i := 0; i < 64; i++ {
		b.pieces[i] = None
	}

	b.halfMoveClockStack = new(Stack)
	b.capturedPieceStack = new(Stack)
	b.castlingRightStack = new(Stack)
	b.epSquareStack = new(Stack)
	b.moveStack = new(Stack)

	b.epSquare = 0
	b.castlingRights = CastlingNone
	b.turn = White
	b.fullMoveNumber = 1
	b.halfMoveClock = 0
	b.incrementalZobristHash = b.BoardZobristHash(PolyglotRandomArray)
	b.transpositions = map[uint64]int{b.ZobristHash(nil): 1}
}

// Gets the piece at the given square.
func (b *Bitboard) PieceAt(square int) *Piece {
	mask := BBSquares[square]
	var color Colors
	if b.occupiedCo[Black]&mask > 0 {
		color = Black
	} else {
		color = White
	}

	pieceType := b.PieceTypeAt(square)

	if pieceType > None {
		return NewPiece(pieceType, color)
	}

	return nil
}

// Gets the piece type at the given square.
func (b *Bitboard) PieceTypeAt(square int) PieceTypes {
	return b.pieces[square]
}

// Removes a piece from the given square if present.
func (b *Bitboard) RemovePieceAt(square int) {
	pieceType := b.pieces[square]

	if pieceType == None {
		return
	}

	mask := BBSquares[square]

	if pieceType == Pawn {
		b.pawns ^= mask
	} else if pieceType == Knight {
		b.knights ^= mask
	} else if pieceType == Bishop {
		b.bishops ^= mask
	} else if pieceType == Rook {
		b.rooks ^= mask
	} else if pieceType == Queen {
		b.queens ^= mask
	} else {
		b.kings ^= mask
	}

	var color Colors
	if b.occupiedCo[Black]&mask > 0 {
		color = Black
	} else {
		color = White
	}

	b.pieces[square] = None
	b.occupied ^= mask
	b.occupiedCo[color] ^= mask
	b.occupiedL90 ^= BBSquares[SquaresL90[square]]
	b.occupiedR45 ^= BBSquares[SquaresR45[square]]
	b.occupiedL45 ^= BBSquares[SquaresL45[square]]

	// Update incremental zobrist hash.
	pieceIndex := (int(pieceType)-1)*2 + 1
	if color == Black {
		pieceIndex = (int(pieceType) - 1) * 2
	}

	b.incrementalZobristHash ^= PolyglotRandomArray[64*pieceIndex+8*rankIndex(square)+fileIndex(square)]
}

// Sets a piece at the given square. An existing piece is replaced.
func (b *Bitboard) SetPieceAt(square int, piece *Piece) {
	b.RemovePieceAt(square)

	b.pieces[square] = piece.pieceType

	mask := BBSquares[square]

	if piece.pieceType == Pawn {
		b.pawns |= mask
	} else if piece.pieceType == Knight {
		b.knights |= mask
	} else if piece.pieceType == Bishop {
		b.bishops |= mask
	} else if piece.pieceType == Rook {
		b.rooks |= mask
	} else if piece.pieceType == Queen {
		b.queens |= mask
	} else if piece.pieceType == King {
		b.kings |= mask
		b.kingSquares[piece.color] = square
	}

	b.occupied ^= mask
	b.occupiedCo[piece.color] ^= mask
	b.occupiedL90 ^= BBSquares[SquaresL90[square]]
	b.occupiedR45 ^= BBSquares[SquaresR45[square]]
	b.occupiedL45 ^= BBSquares[SquaresL45[square]]

	// Update incremental zorbist hash.
	pieceIndex := (int(piece.pieceType)-1)*2 + 1
	if piece.color == Black {
		pieceIndex = (int(piece.pieceType) - 1) * 2
	}

	b.incrementalZobristHash ^= PolyglotRandomArray[64*pieceIndex+8*rankIndex(square)+fileIndex(square)]
}

func debugPrintBoard(board uint64) {
	for y := 7; y >= 0; y-- {
		for x := 0; x < 8; x++ {
			if BBSquares[Squares[y*8+x]]&board > 0 {
				fmt.Printf("x ")
			} else {
				fmt.Printf(". ")
			}
		}
		fmt.Printf("\n")
	}
	fmt.Printf("\n")
}

func (b *Bitboard) GeneratePseudoLegalMoves(castling, pawns, knights, bishops, rooks, queens, king bool) []*Move {
	result := []*Move{}
	if b.turn == White {
		if castling {
			// Castling short.
			if (b.castlingRights&CastlingWhiteKingSide > 0) && ((BBF1|BBG1)&b.occupied) == 0 {
				if !b.IsAttackedBy(Black, E1) && !b.IsAttackedBy(Black, F1) && !b.IsAttackedBy(Black, G1) {
					result = append(result, NewMove(E1, G1, None))
				}
			}

			// Castling long.
			if (b.castlingRights&CastlingWhiteQueenSide > 0) && ((BBB1|BBC1|BBD1)&b.occupied) == 0 {
				if !b.IsAttackedBy(Black, C1) && !b.IsAttackedBy(Black, D1) && !b.IsAttackedBy(Black, E1) {
					result = append(result, NewMove(E1, C1, None))
				}
			}
		}

		if pawns {
			// En-passant moves.
			movers := b.pawns & b.occupiedCo[White]
			if b.epSquare > 0 {
				moves := BBPawnAttacks[Black][b.epSquare] & movers

				fromSquare := bitScan(moves, 0)
				for fromSquare != -1 {
					result = append(result, NewMove(fromSquare, b.epSquare, None))
					fromSquare = bitScan(moves, fromSquare+1)
				}
			}

			// Pawn captures.
			moves := shiftUpRight(movers) & b.occupiedCo[Black]
			toSquare := bitScan(moves, 0)
			for toSquare != -1 {
				fromSquare := toSquare - 9
				if rankIndex(toSquare) != 7 {
					result = append(result, NewMove(fromSquare, toSquare, None))
				} else {
					result = append(result, NewMove(fromSquare, toSquare, Queen))
					result = append(result, NewMove(fromSquare, toSquare, Knight))
					result = append(result, NewMove(fromSquare, toSquare, Rook))
					result = append(result, NewMove(fromSquare, toSquare, Bishop))
				}
				toSquare = bitScan(moves, toSquare+1)
			}

			moves = shiftUpLeft(movers) & b.occupiedCo[Black]
			toSquare = bitScan(moves, 0)
			for toSquare != -1 {
				fromSquare := toSquare - 7
				if rankIndex(toSquare) != 7 {
					result = append(result, NewMove(fromSquare, toSquare, None))
				} else {
					result = append(result, NewMove(fromSquare, toSquare, Queen))
					result = append(result, NewMove(fromSquare, toSquare, Knight))
					result = append(result, NewMove(fromSquare, toSquare, Rook))
					result = append(result, NewMove(fromSquare, toSquare, Bishop))
				}
				toSquare = bitScan(moves, toSquare+1)
			}

			// Pawns one forward.
			moves = shiftUp(movers) & ^b.occupied
			movers = moves
			toSquare = bitScan(moves, 0)
			for toSquare != -1 {
				fromSquare := toSquare - 8
				if rankIndex(toSquare) != 7 {
					result = append(result, NewMove(fromSquare, toSquare, None))
				} else {
					result = append(result, NewMove(fromSquare, toSquare, Queen))
					result = append(result, NewMove(fromSquare, toSquare, Knight))
					result = append(result, NewMove(fromSquare, toSquare, Rook))
					result = append(result, NewMove(fromSquare, toSquare, Bishop))
				}
				toSquare = bitScan(moves, toSquare+1)
			}

			// Pawns two forward.
			moves = shiftUp(movers) & BBRank4 & ^b.occupied
			toSquare = bitScan(moves, 0)
			for toSquare != -1 {
				fromSquare := toSquare - 16
				result = append(result, NewMove(fromSquare, toSquare, None))
				toSquare = bitScan(moves, toSquare+1)
			}
		}
	} else {
		if castling {
			// Castling short.
			if (b.castlingRights&CastlingBlackKingSide > 0) && ((BBF8|BBG8)&b.occupied) == 0 {
				if !b.IsAttackedBy(White, E8) && !b.IsAttackedBy(White, F8) && !b.IsAttackedBy(White, G8) {
					result = append(result, NewMove(E8, G8, None))
				}
			}

			// Castling long.
			if (b.castlingRights&CastlingBlackQueenSide > 0) && ((BBB8|BBC8|BBD8)&b.occupied) == 0 {
				if !b.IsAttackedBy(White, C8) && !b.IsAttackedBy(White, D8) && !b.IsAttackedBy(White, E8) {
					result = append(result, NewMove(E8, C8, None))
				}
			}
		}

		if pawns {
			// En-passant moves.
			movers := b.pawns & b.occupiedCo[Black]
			if b.epSquare > 0 {
				moves := BBPawnAttacks[White][b.epSquare] & movers

				fromSquare := bitScan(moves, 0)
				for fromSquare != -1 {
					result = append(result, NewMove(fromSquare, b.epSquare, None))
					fromSquare = bitScan(moves, fromSquare+1)
				}
			}

			// Pawn captures.
			moves := shiftDownLeft(movers) & b.occupiedCo[White]
			toSquare := bitScan(moves, 0)
			for toSquare != -1 {
				fromSquare := toSquare + 9
				if rankIndex(toSquare) != 0 {
					result = append(result, NewMove(fromSquare, toSquare, None))
				} else {
					result = append(result, NewMove(fromSquare, toSquare, Queen))
					result = append(result, NewMove(fromSquare, toSquare, Knight))
					result = append(result, NewMove(fromSquare, toSquare, Rook))
					result = append(result, NewMove(fromSquare, toSquare, Bishop))
				}
				toSquare = bitScan(moves, toSquare+1)
			}

			moves = shiftDownRight(movers) & b.occupiedCo[White]
			toSquare = bitScan(moves, 0)
			for toSquare != -1 {
				fromSquare := toSquare + 7
				if rankIndex(toSquare) != 0 {
					result = append(result, NewMove(fromSquare, toSquare, None))
				} else {
					result = append(result, NewMove(fromSquare, toSquare, Queen))
					result = append(result, NewMove(fromSquare, toSquare, Knight))
					result = append(result, NewMove(fromSquare, toSquare, Rook))
					result = append(result, NewMove(fromSquare, toSquare, Bishop))
				}
				toSquare = bitScan(moves, toSquare+1)
			}

			// Pawns one forward.
			moves = shiftDown(movers) & ^b.occupied
			movers = moves
			toSquare = bitScan(moves, 0)
			for toSquare != -1 {
				fromSquare := toSquare + 8
				if rankIndex(toSquare) != 0 {
					result = append(result, NewMove(fromSquare, toSquare, None))
				} else {
					result = append(result, NewMove(fromSquare, toSquare, Queen))
					result = append(result, NewMove(fromSquare, toSquare, Knight))
					result = append(result, NewMove(fromSquare, toSquare, Rook))
					result = append(result, NewMove(fromSquare, toSquare, Bishop))
				}
				toSquare = bitScan(moves, toSquare+1)
			}

			// Pawns two forward.
			moves = shiftDown(movers) & BBRank5 & ^b.occupied
			toSquare = bitScan(moves, 0)
			for toSquare != -1 {
				fromSquare := toSquare + 16
				result = append(result, NewMove(fromSquare, toSquare, None))
				toSquare = bitScan(moves, toSquare+1)
			}
		}
	}

	if knights {
		// Knight moves.
		movers := b.knights & b.occupiedCo[b.turn]
		fromSquare := bitScan(movers, 0)
		for fromSquare != -1 {
			moves := b.KnightAttacksFrom(fromSquare) & ^b.occupiedCo[b.turn]
			toSquare := bitScan(moves, 0)
			for toSquare != -1 {
				result = append(result, NewMove(fromSquare, toSquare, None))
				toSquare = bitScan(moves, toSquare+1)
			}
			fromSquare = bitScan(movers, fromSquare+1)
		}
	}

	if bishops {
		// Bishop moves.
		movers := b.bishops & b.occupiedCo[b.turn]
		fromSquare := bitScan(movers, 0)
		for fromSquare != -1 {
			moves := b.BishopAttacksFrom(fromSquare) & ^b.occupiedCo[b.turn]
			toSquare := bitScan(moves, 0)
			for toSquare != -1 {
				result = append(result, NewMove(fromSquare, toSquare, None))
				toSquare = bitScan(moves, toSquare+1)
			}
			fromSquare = bitScan(movers, fromSquare+1)
		}
	}

	if rooks {
		// Rook moves.
		movers := b.rooks & b.occupiedCo[b.turn]
		fromSquare := bitScan(movers, 0)
		for fromSquare != -1 {
			moves := b.RookAttacksFrom(fromSquare) & ^b.occupiedCo[b.turn]
			toSquare := bitScan(moves, 0)
			for toSquare != -1 {
				result = append(result, NewMove(fromSquare, toSquare, None))
				toSquare = bitScan(moves, toSquare+1)
			}
			fromSquare = bitScan(movers, fromSquare+1)
		}
	}

	if queens {
		// Queen moves.
		movers := b.queens & b.occupiedCo[b.turn]
		fromSquare := bitScan(movers, 0)
		for fromSquare != -1 {
			moves := b.QueenAttacksFrom(fromSquare) & ^b.occupiedCo[b.turn]
			toSquare := bitScan(moves, 0)
			for toSquare != -1 {
				result = append(result, NewMove(fromSquare, toSquare, None))
				toSquare = bitScan(moves, toSquare+1)
			}
			fromSquare = bitScan(movers, fromSquare+1)
		}
	}

	if king {
		// King moves.
		fromSquare := b.kingSquares[b.turn]
		moves := b.KingAttacksFrom(fromSquare) & ^b.occupiedCo[b.turn]
		toSquare := bitScan(moves, 0)
		for toSquare != -1 {
			result = append(result, NewMove(fromSquare, toSquare, None))
			toSquare = bitScan(moves, toSquare+1)
		}
	}

	return result
}

// In a way duplicates GeneratePseudoLegalMoves() in order to use
// population counts instead of counting actually yielded moves.
func (b *Bitboard) PseudoLegalMoveCount() int {
	count := 0
	if b.turn == White {
		// Castling short.
		if (b.castlingRights&CastlingWhiteKingSide > 0) && ((BBF1|BBG1)&b.occupied) == 0 {
			if !b.IsAttackedBy(Black, E1) && !b.IsAttackedBy(Black, F1) && !b.IsAttackedBy(Black, G1) {
				count++
			}
		}

		// Castling long.
		if (b.castlingRights&CastlingWhiteQueenSide > 0) && ((BBB1|BBC1|BBD1)&b.occupied) == 0 {
			if !b.IsAttackedBy(Black, C1) && !b.IsAttackedBy(Black, D1) && !b.IsAttackedBy(Black, E1) {
				count++
			}
		}
		// En-passant moves.
		movers := b.pawns & b.occupiedCo[White]
		if b.epSquare > 0 {
			moves := BBPawnAttacks[Black][b.epSquare] & movers
			count += popCount(moves)
		}

		// Pawn captures.
		moves := shiftUpRight(movers) & b.occupiedCo[Black]
		count += popCount(moves&BBRank8) * 3
		count += popCount(moves)

		moves = shiftUpLeft(movers) & b.occupiedCo[Black]
		count += popCount(moves&BBRank8) * 3
		count += popCount(moves)

		// Pawns one forward.
		moves = shiftUp(movers) & ^b.occupied
		movers = moves
		count += popCount(moves&BBRank8) * 3
		count += popCount(moves)

		// Pawns two forward.
		moves = shiftUp(movers) & BBRank4 & ^b.occupied
		count += popCount(moves)
	} else {
		// Castling short.
		if (b.castlingRights&CastlingBlackKingSide > 0) && ((BBF8|BBG8)&b.occupied) == 0 {
			if !b.IsAttackedBy(White, E8) && !b.IsAttackedBy(White, F8) && !b.IsAttackedBy(White, G8) {
				count++
			}
		}

		// Castling long.
		if (b.castlingRights&CastlingBlackQueenSide > 0) && ((BBB8|BBC8|BBD8)&b.occupied) == 0 {
			if !b.IsAttackedBy(White, C8) && !b.IsAttackedBy(White, D8) && !b.IsAttackedBy(White, E8) {
				count++
			}
		}
		// En-passant moves.
		movers := b.pawns & b.occupiedCo[Black]
		if b.epSquare > 0 {
			moves := BBPawnAttacks[White][b.epSquare] & movers
			count += popCount(moves)
		}

		// Pawn captures.
		moves := shiftDownLeft(movers) & b.occupiedCo[White]
		count += popCount(moves&BBRank1) * 3
		count += popCount(moves)

		moves = shiftDownRight(movers) & b.occupiedCo[White]
		count += popCount(moves&BBRank1) * 3
		count += popCount(moves)

		// Pawns one forward.
		moves = shiftDown(movers) & ^b.occupied
		movers = moves
		count += popCount(moves&BBRank1) * 3

		// Pawns two forward.
		moves = shiftDown(movers) & BBRank5 & ^b.occupied
		count += popCount(moves)
	}

	// Knight moves.
	movers := b.knights & b.occupiedCo[b.turn]
	fromSquare := bitScan(movers, 0)
	for fromSquare != -1 {
		moves := b.KnightAttacksFrom(fromSquare) & ^b.occupiedCo[b.turn]
		count += popCount(moves)
		fromSquare = bitScan(movers, fromSquare+1)
	}

	// Bishop moves.
	movers = b.bishops & b.occupiedCo[b.turn]
	fromSquare = bitScan(movers, 0)
	for fromSquare != -1 {
		moves := b.BishopAttacksFrom(fromSquare) & ^b.occupiedCo[b.turn]
		count += popCount(moves)
		fromSquare = bitScan(movers, fromSquare+1)
	}

	// Rook moves.
	movers = b.rooks & b.occupiedCo[b.turn]
	fromSquare = bitScan(movers, 0)
	for fromSquare != -1 {
		moves := b.RookAttacksFrom(fromSquare) & ^b.occupiedCo[b.turn]
		count += popCount(moves)
		fromSquare = bitScan(movers, fromSquare+1)
	}

	// Queen moves.
	movers = b.queens & b.occupiedCo[b.turn]
	fromSquare = bitScan(movers, 0)
	for fromSquare != -1 {
		moves := b.QueenAttacksFrom(fromSquare) & ^b.occupiedCo[b.turn]
		count += popCount(moves)
		fromSquare = bitScan(movers, fromSquare+1)
	}

	// King moves.
	fromSquare = b.kingSquares[b.turn]
	moves := b.KingAttacksFrom(fromSquare) & ^b.occupiedCo[b.turn]
	count += popCount(moves)

	return count
}

// Checks if the given side attacks the given square. Pinned pieces still
// count as attackers.
func (b *Bitboard) IsAttackedBy(color Colors, square int) bool {
	if (BBPawnAttacks[color^1][square] & (b.pawns | b.bishops) & b.occupiedCo[color]) > 0 {
		return true
	}

	if b.KnightAttacksFrom(square)&b.knights&b.occupiedCo[color] > 0 {
		return true
	}

	if b.BishopAttacksFrom(square)&(b.bishops|b.queens)&b.occupiedCo[color] > 0 {
		return true
	}

	if b.RookAttacksFrom(square)&(b.rooks|b.queens)&b.occupiedCo[color] > 0 {
		return true
	}

	if b.KingAttacksFrom(square)&(b.kings|b.queens)&b.occupiedCo[color] > 0 {
		return true
	}

	return false
}

func (b *Bitboard) AttackerMask(color Colors, square int) uint64 {
	attackers := BBPawnAttacks[color^1][square] & b.pawns
	attackers |= b.KnightAttacksFrom(square) & b.knights
	attackers |= b.BishopAttacksFrom(square) & (b.bishops | b.queens)
	attackers |= b.RookAttacksFrom(square) & (b.rooks | b.queens)
	attackers |= b.KingAttacksFrom(square) & b.kings
	return attackers & b.occupiedCo[color]
}

// Gets a set of attackers of the given color for the given square.
//
// Returns a set of squares.
func (b *Bitboard) Attackers(color Colors, square int) *SquareSet {
	return NewSquareSet(b.AttackerMask(color, square))
}

// Checks if the current side to move is in check.
func (b *Bitboard) IsCheck() bool {
	return b.IsAttackedBy(b.turn^1, b.kingSquares[b.turn])
}

func (b *Bitboard) PawnMovesFrom(square int) uint64 {
	targets := BBPawnF1[b.turn][square] & ^b.occupied

	if targets > 0 {
		targets |= BBPawnF2[b.turn][square] & ^b.occupied
	}

	if b.epSquare == 0 {
		targets |= BBPawnAttacks[b.turn][square] & b.occupiedCo[b.turn^1]
	} else {
		targets |= BBPawnAttacks[b.turn][square] & (b.occupiedCo[b.turn^1] | BBSquares[b.epSquare])
	}

	return targets
}

func (b *Bitboard) KnightAttacksFrom(square int) uint64 {
	return BBKnightAttacks[square]
}

func (b *Bitboard) KingAttacksFrom(square int) uint64 {
	return BBKingAttacks[square]
}

func (b *Bitboard) RookAttacksFrom(square int) uint64 {
	return (BBRankAttacks[square][(b.occupied>>((uint(square) & ^uint(7))+1))&63] |
		BBFileAttacks[square][(b.occupiedL90>>(((uint(square)&7)<<3)+1))&63])
}

func (b *Bitboard) BishopAttacksFrom(square int) uint64 {
	return (BBR45Attacks[square][(b.occupiedR45>>BBShiftR45[square])&63] |
		BBL45Attacks[square][(b.occupiedL45>>BBShiftL45[square])&63])
}

func (b *Bitboard) QueenAttacksFrom(square int) uint64 {
	return b.RookAttacksFrom(square) | b.BishopAttacksFrom(square)
}

// Checks if the given move would move would leave the king in check or
// put it into check.
func (b *Bitboard) IsIntoCheck(move *Move) bool {
	b.Push(move)
	isCheck := b.WasIntoCheck()
	b.Pop()
	return isCheck
}

// Checks if the king of the other side is attacked. Such a position is not
// valid and could only be reached by an illegal move.
func (b *Bitboard) WasIntoCheck() bool {
	return b.IsAttackedBy(b.turn, b.kingSquares[b.turn^1])
}

func (b *Bitboard) GenerateLegalMoves(castling, pawns, knights, bishops, rooks, queens, kings bool) []*Move {
	result := []*Move{}
	pseudo := b.GeneratePseudoLegalMoves(castling, pawns, knights, bishops, rooks, queens, kings)
	for _, move := range pseudo {
		if !b.IsIntoCheck(move) {
			result = append(result, move)
		}
	}

	return result
}

func (b *Bitboard) IsPseudoLegal(move *Move) bool {
	// Null moves are not pseudo legal.
	if move == nil {
		return false
	}

	// Source square must not be vacant.
	piece := b.PieceTypeAt(move.fromSquare)
	if piece == None {
		return false
	}

	// Get square masks.
	fromMask := BBSquares[move.fromSquare]
	toMask := BBSquares[move.toSquare]

	// Check turn.
	if (b.occupiedCo[b.turn] & fromMask) == 0 {
		return false
	}

	// Destination square can not be occupied.
	if (b.occupiedCo[b.turn] & toMask) > 0 {
		return false
	}

	// Only pawns can promote and only on the backrank.
	if move.promotion > None {
		if piece != Pawn {
			return false
		}

		if b.turn == White && rankIndex(move.toSquare) != 7 {
			return false
		} else if b.turn == Black && rankIndex(move.toSquare) != 0 {
			return false
		}
	}

	// Handle moves by piece type.
	if piece == King {
		// Castling.
		if b.turn == White && move.fromSquare == E1 {
			if move.toSquare == G1 && b.castlingRights&CastlingWhiteKingSide > 0 && (BBF1|BBG1)&b.occupied == 0 {
				return true
			} else if move.toSquare == C1 && b.castlingRights&CastlingWhiteQueenSide > 0 && (BBB1|BBC1|BBD1)&b.occupied == 0 {
				return true
			}
		} else if b.turn == Black && move.fromSquare == E8 {
			if move.toSquare == G8 && b.castlingRights&CastlingBlackKingSide > 0 && (BBF8|BBG8)&b.occupied == 0 {
				return true
			} else if move.toSquare == C8 && b.castlingRights&CastlingBlackQueenSide > 0 && (BBB8|BBC8|BBD8)&b.occupied == 0 {
				return true
			}
		}

		return b.KingAttacksFrom(move.fromSquare)&toMask > 0
	} else if piece == Pawn {
		// Require promotion type if on promotion rank.
		if move.promotion == None {
			if b.turn == White && rankIndex(move.toSquare) == 7 {
				return false
			}
			if b.turn == Black && rankIndex(move.toSquare) == 0 {
				return false
			}
		}

		return b.PawnMovesFrom(move.fromSquare)&toMask > 0
	} else if piece == Queen {
		return b.QueenAttacksFrom(move.fromSquare)&toMask > 0
	} else if piece == Rook {
		return b.RookAttacksFrom(move.fromSquare)&toMask > 0
	} else if piece == Bishop {
		return b.BishopAttacksFrom(move.fromSquare)&toMask > 0
	} else if piece == Knight {
		return b.KnightAttacksFrom(move.fromSquare)&toMask > 0
	}

	return false
}

func (b *Bitboard) IsLegal(move *Move) bool {
	return b.IsPseudoLegal(move) && !b.IsIntoCheck(move)
}

// Checks if the game is over due to checkmate, stalemate, insufficient
// mating material, the seventyfive-move rule or fivefold repitition.
func (b *Bitboard) IsGameOver() bool {
	// Seventyfive-move rule.
	if b.halfMoveClock >= 150 {
		return true
	}

	// Insufficient material.
	if b.IsInsufficientMaterial() {
		return true
	}

	// Stalemate or checkmate.
	if len(b.GenerateLegalMoves(true, true, true, true, true, true, true)) == 0 {
		return true
	}

	// Fivefold repitition.
	if b.IsFivefoldRepitition() {
		return true
	}

	return false
}

// Checks if the current position is a checkmate.
func (b *Bitboard) IsCheckmate() bool {
	if !b.IsCheck() {
		return false
	}

	if len(b.GenerateLegalMoves(true, true, true, true, true, true, true)) > 0 {
		return false
	}

	return true
}

// Checks if the current position is a stalemate.
func (b *Bitboard) IsStalemate() bool {
	if b.IsCheck() {
		return false
	}

	if len(b.GenerateLegalMoves(true, true, true, true, true, true, true)) > 0 {
		return false
	}

	return true
}

// Checks for a draw due to insufficient mating material.
func (b *Bitboard) IsInsufficientMaterial() bool {
	// Enough material to mate.
	if b.pawns > 0 || b.rooks > 0 || b.queens > 0 {
		return false
	}

	// A single knight or a single bishop.
	if popCount(b.occupied) <= 3 {
		return true
	}

	// More than a single knight.
	if b.knights > 0 {
		return false
	}

	// All bishops on the same color.
	if b.bishops&BBDarkSquares == 0 {
		return true
	} else if b.bishops&BBLightSquares == 0 {
		return true
	}

	return false
}

// Since the first of July 2014 a game is automatically drawn (without
// a claim by one of the players) if the half move clock since a capture
// or pawn move is equal to or grather than 150. Other means to end a game
// take precedence.
func (b *Bitboard) IsSeventyfiveMoves() bool {
	if b.halfMoveClock >= 150 {
		if len(b.GenerateLegalMoves(true, true, true, true, true, true, true)) > 0 {
			return true
		}
	}

	return false
}

// Since the first of July 2014 a game is automatically drawn (without
// a claim by one of the players) if a position occurs for the fifth time
// on consecutive alternating moves.
func (b *Bitboard) IsFivefoldRepitition() bool {
	zobristHash := b.ZobristHash(nil)

	// A minimum amount of moves must have been played and the position
	// in question must have appeared at least five times.
	if b.moveStack.Len() < 16 || b.transpositions[zobristHash] < 5 {
		return false
	}

	switchyard := new(Stack)

	for i := 0; i < 4; i++ {
		// Go back two full moves, each.
		for j := 0; j < 4; j++ {
			switchyard.Push(b.Pop())
		}

		// Check the position was the same before.
		if b.ZobristHash(nil) != zobristHash {
			for switchyard.Len() > 0 {
				b.Push(switchyard.Pop().(*Move))
			}

			return false
		}
	}

	for switchyard.Len() > 0 {
		b.Push(switchyard.Pop().(*Move))
	}

	return true
}

// Checks if the side to move can claim a draw by the fifty-move rule or
// by threefold repitition.
func (b *Bitboard) CanClaimDraw() bool {
	return b.CanClaimFiftyMoves() || b.CanClaimThreefoldRepitition()
}

// Draw by the fifty-move rule can be claimed once the clock of halfmoves
// since the last capture or pawn move becomes equal or greater to 100
// and the side to move still has a legal move they can make.
func (b *Bitboard) CanClaimFiftyMoves() bool {
	// Fifty-move rule.
	if b.halfMoveClock >= 100 {
		if len(b.GenerateLegalMoves(true, true, true, true, true, true, true)) > 0 {
			return true
		}
	}

	return false
}

// Draw by threefold repitition can be claimed if the position on the
// board occured for the third time or if such a repitition is reached
// with one of the possible legal moves.
func (b *Bitboard) CanClaimThreefoldRepitition() bool {
	// Threefold repition occured.
	if b.transpositions[b.ZobristHash(nil)] >= 3 {
		return true
	}

	// The next legal move is a threefold repitition.
	for _, move := range b.GeneratePseudoLegalMoves(true, true, true, true, true, true, true) {
		b.Push(move)

		if !b.WasIntoCheck() && b.transpositions[b.ZobristHash(nil)] >= 3 {
			b.Pop()
			return true
		}

		b.Pop()
	}

	return false
}

// Updates the position with the given move and puts it onto a stack.
//
// Null moves just increment the move counters, switch turns and forfeit
// en passant capturing.
//
// No validation is performed. For performance moves are assumed to be at
// least pseudo legal. Otherwise there is no guarantee that the previous
// board state can be restored.
func (b *Bitboard) Push(move *Move) {
	// Increment fullmove number.
	if b.turn == Black {
		b.fullMoveNumber++
	}

	// Remember game state.
	capturedPiece := None
	if move != nil {
		capturedPiece = b.PieceTypeAt(move.toSquare)
	}
	b.halfMoveClockStack.Push(b.halfMoveClock)
	b.castlingRightStack.Push(b.castlingRights)
	b.capturedPieceStack.Push(capturedPiece)
	b.epSquareStack.Push(b.epSquare)
	b.moveStack.Push(move)

	// On a null move simply swap turns.
	if move == nil {
		b.turn ^= 1
		b.epSquare = 0
		b.halfMoveClock++
		return
	}

	// Update half move counter.
	pieceType := b.PieceTypeAt(move.fromSquare)
	if pieceType == Pawn || capturedPiece != None {
		b.halfMoveClock = 0
	} else {
		b.halfMoveClock++
	}

	// Promotion.
	if move.promotion != None {
		pieceType = move.promotion
	}

	// Remove piece from target square.
	b.RemovePieceAt(move.fromSquare)

	// Handle special pawn moves.
	b.epSquare = 0
	if pieceType == Pawn {
		diff := move.toSquare - move.fromSquare
		if diff < 0 {
			diff = -diff
		}

		// Remove pawns captured en-passant.
		if (diff == 7 || diff == 9) && b.occupied&BBSquares[move.toSquare] == 0 {
			if b.turn == White {
				b.RemovePieceAt(move.toSquare - 8)
			} else {
				b.RemovePieceAt(move.toSquare + 8)
			}
		}

		// Set en-passant square.
		if diff == 16 {
			if b.turn == White {
				b.epSquare = move.toSquare - 8
			} else {
				b.epSquare = move.toSquare + 8
			}
		}
	}

	// Castling rights.
	if move.fromSquare == E1 {
		b.castlingRights &= ^CastlingWhite
	} else if move.fromSquare == E8 {
		b.castlingRights &= ^CastlingBlack
	} else if move.fromSquare == A1 || move.toSquare == A1 {
		b.castlingRights &= ^CastlingWhiteQueenSide
	} else if move.fromSquare == A8 || move.toSquare == A8 {
		b.castlingRights &= ^CastlingBlackQueenSide
	} else if move.fromSquare == H1 || move.toSquare == H1 {
		b.castlingRights &= ^CastlingWhiteKingSide
	} else if move.fromSquare == H8 || move.toSquare == H8 {
		b.castlingRights &= ^CastlingBlackKingSide
	}

	// Castling.
	if pieceType == King {
		if move.fromSquare == E1 && move.toSquare == G1 {
			b.SetPieceAt(F1, NewPiece(Rook, White))
			b.RemovePieceAt(H1)
		} else if move.fromSquare == E1 && move.toSquare == C1 {
			b.SetPieceAt(D1, NewPiece(Rook, White))
			b.RemovePieceAt(A1)
		} else if move.fromSquare == E8 && move.toSquare == G8 {
			b.SetPieceAt(F8, NewPiece(Rook, Black))
			b.RemovePieceAt(H8)
		} else if move.fromSquare == E8 && move.toSquare == C8 {
			b.SetPieceAt(D8, NewPiece(Rook, Black))
			b.RemovePieceAt(A8)
		}
	}

	// Put piece on target square.
	b.SetPieceAt(move.toSquare, NewPiece(pieceType, b.turn))

	// Swap turn.
	b.turn ^= 1

	// Update transposition table
	b.transpositions[b.ZobristHash(nil)]++
}

// Restores the previous position and returns the last move from the stack.
func (b *Bitboard) Pop() *Move {
	move := b.moveStack.Pop().(*Move)

	// Update transposition table.
	b.transpositions[b.ZobristHash(nil)]--

	// Decrement fullmove number.
	if b.turn == White {
		b.fullMoveNumber--
	}

	// Restore state.
	b.halfMoveClock = b.halfMoveClockStack.Pop().(int)
	b.castlingRights = b.castlingRightStack.Pop().(int)
	b.epSquare = b.epSquareStack.Pop().(int)
	capturedPiece := b.capturedPieceStack.Pop().(PieceTypes)
	capturedPieceColor := b.turn

	// On a null move simply swap the turn.
	if move == nil {
		b.turn ^= 1
		return move
	}

	// Restore the source square.
	piece := b.PieceTypeAt(move.toSquare)
	if move.promotion != None {
		piece = Pawn
	}
	b.SetPieceAt(move.fromSquare, NewPiece(piece, b.turn^1))

	// Restore the target square.
	if capturedPiece > None {
		b.SetPieceAt(move.toSquare, NewPiece(capturedPiece, capturedPieceColor))
	} else {
		b.RemovePieceAt(move.toSquare)

		// Restore captured pawn after en-passant.
		diff := move.fromSquare - move.toSquare
		if diff < 0 {
			diff = -diff
		}
		if piece == Pawn && (diff == 7 || diff == 9) {
			if b.turn == White {
				b.SetPieceAt(move.toSquare+8, NewPiece(Pawn, White))
			} else {
				b.SetPieceAt(move.toSquare-8, NewPiece(Pawn, Black))
			}
		}
	}

	// Restore rook position after castling.
	if piece == King {
		if move.fromSquare == E1 && move.toSquare == G1 {
			b.RemovePieceAt(F1)
			b.SetPieceAt(H1, NewPiece(Rook, White))
		} else if move.fromSquare == E1 && move.toSquare == C1 {
			b.RemovePieceAt(D1)
			b.SetPieceAt(A1, NewPiece(Rook, White))
		} else if move.fromSquare == E8 && move.toSquare == G8 {
			b.RemovePieceAt(F8)
			b.SetPieceAt(H8, NewPiece(Rook, Black))
		} else if move.fromSquare == E8 && move.toSquare == C8 {
			b.RemovePieceAt(D8)
			b.SetPieceAt(A8, NewPiece(Rook, Black))
		}
	}

	// Swap turn.
	b.turn ^= 1

	return move
}

// Gets the last move from the move stack.
func (b *Bitboard) Peek() *Move {
	move := b.moveStack.Pop().(*Move)
	b.moveStack.Push(move)
	return move
}

// Parses the given EPD string and uses it to set the position.
//
// If present the `hmvc` and the `fmvn` are used to set the half move
// clock and the fullmove number. Otherwise `0` and `1` are used.
//
// Returns a dictionary of parsed operations. Values can be strings,
// integers, floats or move objects.
//
// Returns nil if the EPD string is invalid.
func (b *Bitboard) SetEpd(epd string) (map[string]interface{}, error) {
	// Split into 4 or 5 parts.
	parts := strings.Fields(strings.TrimRight(strings.TrimSpace(epd), ";"))

	if len(parts) < 4 {
		return nil, fmt.Errorf("epd should consist of at least 4 parts '%s'.", epd)
	}

	operations := map[string]interface{}{}

	// Parse the operations.
	if len(parts) > 4 {
		operationPart := parts[len(parts)-1]
		parts = parts[:len(parts)-1]
		operationPart += ";"

		opcode := ""
		operand := ""
		inOperand := false
		inQuotes := false
		escape := false

		var position *Bitboard

		for _, c := range operationPart {
			if !inOperand {
				if c == ';' {
					operations[opcode] = None
					opcode = ""
				} else if c == ' ' {
					if opcode != "" {
						inOperand = true
					}
				} else {
					opcode += string(c)
				}
			} else {
				if c == '"' {
					if operand != "" && !inQuotes {
						inQuotes = true
					} else if escape {
						operand += string(c)
					}
				} else if c == '\\' {
					if escape {
						operand += string(c)
					} else {
						escape = true
					}
				} else if c == 's' {
					if escape {
						operand += ";"
					} else {
						operand += string(c)
					}
				} else if c == ';' {
					if escape {
						operand += "\\"
					}

					if inQuotes {
						operations[opcode] = operand
					} else {
						tmp, err := strconv.Atoi(operand)
						if err != nil {
							tmp2, err := strconv.ParseFloat(operand, 64)
							if err != nil {
								if position == nil {
									position = NewBitboard(" " + strings.Join(append(parts, "0", "1"), ""))
								}

								operations[opcode], _ = position.ParseSan(operand)
							} else {
								operations[opcode] = tmp2
							}
						} else {
							operations[opcode] = tmp
						}
					}

					opcode = ""
					operand = ""
					inOperand = false
					inQuotes = false
					escape = false
				} else {
					operand += string(c)
				}
			}
		}
	}

	// Create a full FEN and parse it.
	if val, ok := operations["hmvc"]; ok {
		parts = append(parts, val.(string))
	} else {
		parts = append(parts, "0")
	}
	if val, ok := operations["fmvn"]; ok {
		parts = append(parts, val.(string))
	} else {
		parts = append(parts, "1")
	}
	b.SetFen(" " + strings.Join(parts, ""))

	return operations, nil
}

// Gets an EPD representation of the current position.
//
// EPD operations can be given as keyword arguments. Supported operands
// are strings, integers, floats and moves. All other operands are
// converted to strings.
//
// `hmvc` and `fmvc` are *not* included by default. You can use:
//
//     board.epd(map[string]interface{}{"hmvc":board.halfMoveClock, "fmvc":board.fullMoveNumber});
func (b *Bitboard) Epd(operations map[string]interface{}) string {
	epd := []string{}
	empty := 0

	// Position part.
	for _, square := range Squares180 {
		piece := b.PieceAt(square)

		if piece == nil {
			empty++
		} else {
			if empty > 0 {
				epd = append(epd, strconv.Itoa(empty))
				empty = 0
			}
			epd = append(epd, piece.String())
		}

		if BBSquares[square]&BBFileH > 0 {
			if empty > 0 {
				epd = append(epd, strconv.Itoa(empty))
				empty = 0
			}

			if square != H1 {
				epd = append(epd, "/")
			}
		}
	}

	epd = append(epd, " ")

	// Side to move.
	if b.turn == White {
		epd = append(epd, "w")
	} else {
		epd = append(epd, "b")
	}

	epd = append(epd, " ")

	// Castling rights.
	if b.castlingRights == 0 {
		epd = append(epd, "-")
	} else {
		if b.castlingRights&CastlingWhiteKingSide > 0 {
			epd = append(epd, "K")
		}
		if b.castlingRights&CastlingWhiteQueenSide > 0 {
			epd = append(epd, "Q")
		}
		if b.castlingRights&CastlingBlackKingSide > 0 {
			epd = append(epd, "k")
		}
		if b.castlingRights&CastlingBlackQueenSide > 0 {
			epd = append(epd, "q")
		}
	}

	epd = append(epd, " ")

	// En-passant square.
	if b.epSquare > 0 {
		epd = append(epd, SquareNames[b.epSquare])
	} else {
		epd = append(epd, "-")
	}

	// Append operations.
	for opcode, operand := range operations {
		epd = append(epd, " ")
		epd = append(epd, opcode)

		switch v := operand.(type) {
		case *Move:
			// Append SAN for moves.
			epd = append(epd, " ")
			epd = append(epd, b.San(v))
		case int:
			epd = append(epd, " ")
			epd = append(epd, strconv.Itoa(v))
		case float64:
			epd = append(epd, " ")
			epd = append(epd, strconv.FormatFloat(v, 'f', -1, 64))
		default:
			if v != nil {
				// Append as escaped string.
				epd = append(epd, " \"")
				epd = append(epd, strings.Replace(strings.Replace(strings.Replace(strings.Replace(fmt.Sprintf("%v", v), "\r", "", -1), "\n", " ", -1), "\\", "\\\\", -1), ";", "\\s", -1))
				epd = append(epd, " \"")
			}
		}
		epd = append(epd, ";")
	}

	return strings.Join(epd, "")
}

// Parses a FEN and sets the position from it.
//
// Returns an error if the FEN string is invalid.
func (b *Bitboard) SetFen(fen string) error {
	// Ensure there are six parts.
	parts := strings.Fields(fen)
	if len(parts) != 6 {
		return fmt.Errorf("fen string should consist of 6 parts: '%s'.", fen)
	}

	// Ensure the board part is valid.
	rows := strings.Split(parts[0], "/")
	if len(rows) != 8 {
		return fmt.Errorf("expected 8 rows in position part of fen: '%s'.", fen)
	}

	// Validate each row.
	for _, row := range rows {
		fieldSum := 0
		previousWasDigit := false

		for _, c := range row {
			if c >= '1' && c <= '8' {
				if previousWasDigit {
					return fmt.Errorf("two subsequent digits in position part of fen: '%s'.", fen)
				}
				fieldSum += int(c - '0')
				previousWasDigit = true
			} else if c == 'p' || c == 'n' || c == 'b' || c == 'r' || c == 'q' || c == 'k' || c == 'P' || c == 'N' || c == 'B' || c == 'R' || c == 'Q' || c == 'K' {
				fieldSum++
				previousWasDigit = false
			} else {
				return fmt.Errorf("invalid character in position part of fen: '%s'.", fen)
			}
		}

		if fieldSum != 8 {
			return fmt.Errorf("expected 8 columns per row in position part of fen: '%s'.", fen)
		}
	}

	// Check that the turn part is valid.
	if parts[1] != "w" && parts[1] != "b" {
		return fmt.Errorf("expected 'w' or 'b' for turn part of fen: '%s'.", fen)
	}

	// Check that the castling part is valid.
	if !FenCastlingRegex.MatchString(parts[2]) {
		return fmt.Errorf("invalid castling part in fen: '%s'.", fen)
	}

	// Check that the en-passant part is valid.
	if parts[3] != "-" {
		var square int
		for i, sn := range SquareNames {
			if sn == parts[3] {
				square = i
				break
			}
		}
		if parts[1] == "w" {
			if rankIndex(square) != 5 {
				return fmt.Errorf("expected en-passant square to be on sixth rank: '%s'.", fen)
			}
		} else {
			if rankIndex(square) != 2 {
				return fmt.Errorf("expected en-passant square to be on third rank: '%s'.", fen)
			}
		}
	}

	// Check that the half move part is valid.
	hm, err := strconv.Atoi(parts[4])
	if err != nil || hm < 0 {
		return fmt.Errorf("halfmove clock can not be negative: '%s'.", fen)
	}

	// Check that the fullmove number part is valid.
	// 0 is allowed for compatibility but later replaced with 1.
	fm, err := strconv.Atoi(parts[5])
	if err != nil || fm < 0 {
		return fmt.Errorf("fullmove number must be positive: '%s'.", fen)
	}

	// Clear board.
	b.Clear()

	// Put pieces on the board.
	squareIndex := 0
	for _, c := range parts[0] {
		if c >= '1' && c <= '8' {
			cint, _ := strconv.Atoi(string(c))
			squareIndex += cint
		} else if c == 'p' || c == 'b' || c == 'n' || c == 'r' || c == 'q' || c == 'k' || c == 'P' || c == 'B' || c == 'N' || c == 'R' || c == 'Q' || c == 'K' {
			b.SetPieceAt(Squares180[squareIndex], PieceFromSymbol(string(c)))
			squareIndex++
		}
	}

	// Set the turn.
	if parts[1] == "w" {
		b.turn = White
	} else {
		b.turn = Black
	}

	// Set castling flags.
	b.castlingRights = CastlingNone
	if strings.Contains(parts[2], "K") {
		b.castlingRights |= CastlingWhiteKingSide
	}
	if strings.Contains(parts[2], "Q") {
		b.castlingRights |= CastlingWhiteQueenSide
	}
	if strings.Contains(parts[2], "k") {
		b.castlingRights |= CastlingBlackKingSide
	}
	if strings.Contains(parts[2], "q") {
		b.castlingRights |= CastlingBlackQueenSide
	}

	// Set the en-passant square.
	if parts[3] == "-" {
		b.epSquare = 0
	} else {
		for i, sn := range SquareNames {
			if sn == parts[3] {
				b.epSquare = i
				break
			}
		}
	}

	// Set the mover counters.
	b.halfMoveClock = hm
	if fm > 0 {
		b.fullMoveNumber = fm
	} else {
		b.fullMoveNumber = 1
	}

	// Reset the transposition table.
	b.transpositions = map[uint64]int{b.ZobristHash(nil): 1}

	return nil
}

// Gets the FEN representation of the position.
func (b *Bitboard) Fen() string {
	fen := []string{}

	// Position, turn, castling and en passant.
	fen = append(fen, b.Epd(nil))

	// Half moves.
	fen = append(fen, " ")
	fen = append(fen, strconv.Itoa(b.halfMoveClock))

	// Ply.
	fen = append(fen, " ")
	fen = append(fen, strconv.Itoa(b.fullMoveNumber))

	return strings.Join(fen, "")
}

// Uses the current position as the context to parse a move in standard
// algebraic notation and return the corresponding move object.
//
// The returned move is guaranteed to be either legal or a null move.
//
// Returns an error if the SAN is invalid or ambiguous.
func (b *Bitboard) ParseSan(san string) (*Move, error) {
	var move *Move

	// Null moves.
	if san == "--" {
		return move, nil
	}

	// Castling.
	if san == "O-O" || san == "O-O+" || san == "O-O#" {
		if b.turn == White {
			move = NewMove(E1, G1, None)
		} else {
			move = NewMove(E8, G8, None)
		}
		if b.kings&b.occupiedCo[b.turn]&BBSquares[move.fromSquare] > 0 && b.IsLegal(move) {
			return move, nil
		} else {
			return nil, fmt.Errorf("illegal san: '%s'.", san)
		}
	} else if san == "O-O-O" || san == "O-O-O+" || san == "O-O-O#" {
		if b.turn == White {
			move = NewMove(E1, C1, None)
		} else {
			move = NewMove(E8, C8, None)
		}
		if b.kings&b.occupiedCo[b.turn]&BBSquares[move.fromSquare] > 0 && b.IsLegal(move) {
			return move, nil
		} else {
			return nil, fmt.Errorf("illegal san: '%s'.", san)
		}
	}

	// Match normal moves.
	match := SanRegex.FindStringSubmatch(san)
	if len(match) == 0 {
		return nil, fmt.Errorf("invalid san: '%s'.", san)
	}

	// Get target square.
	var toSquare int
	for i, sq := range SquareNames {
		if sq == match[4] {
			toSquare = i
			break
		}
	}

	// Get the promotion type.
	promotion := None
	if match[5] != "" {
		for i, sy := range PieceSymbols {
			if sy == strings.ToLower(string(match[5][1])) {
				promotion = PieceTypes(i)
				break
			}
		}
	}

	// Filter by piece type.
	var moves []*Move
	if match[1] == "N" {
		moves = b.GeneratePseudoLegalMoves(false, false, true, false, false, false, false)
	} else if match[1] == "B" {
		moves = b.GeneratePseudoLegalMoves(false, false, false, true, false, false, false)
	} else if match[1] == "K" {
		moves = b.GeneratePseudoLegalMoves(false, false, false, false, false, false, true)
	} else if match[1] == "R" {
		moves = b.GeneratePseudoLegalMoves(false, false, false, false, true, false, false)
	} else if match[1] == "Q" {
		moves = b.GeneratePseudoLegalMoves(false, false, false, false, false, true, false)
	} else {
		moves = b.GeneratePseudoLegalMoves(false, true, false, false, false, false, false)
	}

	// Filter by source file.
	fromMask := BBAll
	if match[2] != "" {
		for i, fn := range FileNames {
			if fn == match[2] {
				fromMask &= BBFiles[i]
				break
			}
		}
	}

	// Filter by source rank.
	if match[3] != "" {
		fromMask &= BBRanks[match[3][0]-'1']
	}

	// Match legal moves.
	var matchedMove *Move
	for _, move := range moves {
		if move.toSquare != toSquare {
			continue
		}

		if move.promotion != promotion {
			continue
		}

		if BBSquares[move.fromSquare]&fromMask == 0 {
			continue
		}

		if b.IsIntoCheck(move) {
			continue
		}

		if matchedMove != nil {
			return nil, fmt.Errorf("ambiguous san: '%s'.", san)
		}

		matchedMove = move
	}

	if matchedMove == nil {
		return nil, fmt.Errorf("illegal san: '%s'.", san)
	}

	return matchedMove, nil
}

// Parses a move in standard algebraic notation, makes the move and puts
// it on the the move stack.
//
// Returns an error if neither legal nor a null move.
//
// Returns the move.
func (b *Bitboard) PushSan(san string) (*Move, error) {
	move, err := b.ParseSan(san)
	if err != nil {
		return nil, err
	}
	b.Push(move)
	return move, nil
}

// Gets the standard algebraic notation of the given move in the context of
// the current position.
//
// There is no validation. It is only guaranteed to work if the move is
// legal or a null move.
func (b *Bitboard) San(move *Move) string {
	if move == nil {
		// Null move.
		return "--"
	}

	piece := b.PieceTypeAt(move.fromSquare)
	enPassant := false

	// Castling.
	if piece == King {
		if move.fromSquare == E1 {
			if move.toSquare == G1 {
				return "O-O"
			} else if move.toSquare == C1 {
				return "O-O-O"
			}
		} else if move.fromSquare == E8 {
			if move.toSquare == G8 {
				return "O-O"
			} else if move.toSquare == C8 {
				return "O-O-O"
			}
		}
	}

	san := ""
	if piece == Pawn {
		san = ""

		// Detect en-passant.
		if BBSquares[move.toSquare]&b.occupied == 0 {
			diff := move.fromSquare - move.toSquare
			if diff < 0 {
				diff = -diff
			}
			if diff == 7 || diff == 9 {
				enPassant = true
			}
		}
	} else {
		var others uint64
		// Get ambiguous move candidates.
		if piece == Knight {
			san = "N"
			others = b.knights & b.KnightAttacksFrom(move.toSquare)
		} else if piece == Bishop {
			san = "B"
			others = b.bishops & b.BishopAttacksFrom(move.toSquare)
		} else if piece == Rook {
			san = "R"
			others = b.rooks & b.RookAttacksFrom(move.toSquare)
		} else if piece == Queen {
			san = "Q"
			others = b.queens & b.QueenAttacksFrom(move.toSquare)
		} else if piece == King {
			san = "K"
			others = b.kings & b.KingAttacksFrom(move.toSquare)
		}

		others &= ^BBSquares[move.fromSquare]
		others &= b.occupiedCo[b.turn]

		// Remove illegal candidates.
		squares := others
		square := bitScan(squares, 0)
		for square != -1 {
			if b.IsIntoCheck(NewMove(square, move.toSquare, None)) {
				others &= ^BBSquares[square]
			}

			square = bitScan(squares, square+1)
		}

		// Disambiguate.
		if others > 0 {
			row, column := false, false

			if others&BBRanks[rankIndex(move.fromSquare)] > 0 {
				column = true
			}

			if others&BBFiles[fileIndex(move.fromSquare)] > 0 {
				row = true
			} else {
				column = true
			}

			if column {
				san += FileNames[fileIndex(move.fromSquare)]
			}
			if row {
				san += strconv.Itoa(rankIndex(move.fromSquare) + 1)
			}
		}
	}

	// Captures.
	if BBSquares[move.toSquare]&b.occupied > 0 || enPassant {
		if piece == Pawn {
			san += FileNames[fileIndex(move.fromSquare)]
		}
		san += "x"
	}

	// Destination square.
	san += SquareNames[move.toSquare]

	// Promotion.
	if move.promotion > None {
		san += "=" + strings.ToUpper(PieceSymbols[move.promotion])
	}

	// Look ahead for check or checkmate.
	b.Push(move)
	if b.IsCheck() {
		if b.IsCheckmate() {
			san += "#"
		} else {
			san += "+"
		}
	}
	b.Pop()

	return san
}

// Gets a bitmask of possible problems with the position.
// Move making, generation and validation are only guaranteed to work on
// a completely valid board.
func (b *Bitboard) Status() Status {
	errors := StatusValid

	if b.occupiedCo[White]&b.kings == 0 {
		errors |= StatusNoWhiteKing
	}
	if b.occupiedCo[Black]&b.kings == 0 {
		errors |= StatusNoBlackKing
	}
	if popCount(b.occupied&b.kings) > 2 {
		errors |= StatusTooManyKings
	}

	if popCount(b.occupiedCo[White]&b.pawns) > 8 {
		errors |= StatusTooManyWhitePawns
	}
	if popCount(b.occupiedCo[Black]&b.pawns) > 8 {
		errors |= StatusTooManyBlackPawns
	}

	if b.pawns&(BBRank1|BBRank8) > 0 {
		errors |= StatusPawnsOnBackrank
	}

	if popCount(b.occupiedCo[White]) > 16 {
		errors |= StatusTooManyWhitePieces
	}
	if popCount(b.occupiedCo[Black]) > 16 {
		errors |= StatusTooManyBlackPieces
	}

	if b.castlingRights&CastlingWhite > 0 {
		if b.kingSquares[White] != E1 {
			errors |= StatusBadCastlingRights
		}

		if b.castlingRights&CastlingWhiteQueenSide > 0 {
			if BBA1&b.occupiedCo[White]&b.rooks == 0 {
				errors |= StatusBadCastlingRights
			}
		}
		if b.castlingRights&CastlingWhiteKingSide > 0 {
			if BBH1&b.occupiedCo[White]&b.rooks == 0 {
				errors |= StatusBadCastlingRights
			}
		}
	}

	if b.castlingRights&CastlingBlack > 0 {
		if b.kingSquares[Black] != E8 {
			errors |= StatusBadCastlingRights
		}

		if b.castlingRights&CastlingBlackQueenSide > 0 {
			if BBA8&b.occupiedCo[Black]&b.rooks == 0 {
				errors |= StatusBadCastlingRights
			}
		}
		if b.castlingRights&CastlingBlackKingSide > 0 {
			if BBH8&b.occupiedCo[Black]&b.rooks == 0 {
				errors |= StatusBadCastlingRights
			}
		}
	}

	if b.epSquare > 0 {
		epRank := 2
		pawnMask := shiftUp(BBSquares[b.epSquare])
		if b.turn == White {
			epRank = 5
			pawnMask = shiftDown(BBSquares[b.epSquare])
		}

		// The en-passant square must be on the third or sixth rank.
		if rankIndex(b.epSquare) != epRank {
			errors |= StatusInvalidEpSquare
		}

		// The last move must have been a double pawn push, so there must
		// be a pawn of the correct color on the fourth or fifth rank.
		if b.pawns&b.occupiedCo[b.turn^1]&pawnMask == 0 {
			errors |= StatusInvalidEpSquare
		}
	}

	if errors&(StatusNoWhiteKing|StatusNoBlackKing|StatusTooManyKings) == 0 {
		if b.WasIntoCheck() {
			errors |= StatusOppositeCheck
		}
	}

	return errors
}

func (b *Bitboard) String() string {
	builder := []string{}

	for _, square := range Squares180 {
		piece := b.PieceAt(square)

		if piece != nil {
			builder = append(builder, piece.String())
		} else {
			builder = append(builder, ".")
		}

		if BBSquares[square]&BBFileH > 0 {
			if square != H1 {
				builder = append(builder, "\n")
			}
		} else {
			builder = append(builder, " ")
		}
	}

	return strings.Join(builder, "")
}

// Returns a Zobrist hash of the current position.
//
// A zobrist hash is an exclusive or of pseudo random values picked from
// an array. Which values are picked is decided by features of the
// position, such as piece positions, castling rights and en-passant
// squares. For this implementation an array of 781 values is required.
//
// The default behaviour is to use values from `PolyglotRandomArray`,
// which makes for hashes compatible with polyglot opening books.
func (b *Bitboard) ZobristHash(array []uint64) uint64 {
	// Hash in the board setup.
	zobristHash := b.BoardZobristHash(array)

	// Default random array is polyglot compatible.
	if array == nil {
		array = PolyglotRandomArray
	}

	// Hash in the castling flags.
	if b.castlingRights&CastlingWhiteKingSide > 0 {
		zobristHash ^= array[768]
	}
	if b.castlingRights&CastlingWhiteQueenSide > 0 {
		zobristHash ^= array[768+1]
	}
	if b.castlingRights&CastlingBlackKingSide > 0 {
		zobristHash ^= array[768+2]
	}
	if b.castlingRights&CastlingBlackQueenSide > 0 {
		zobristHash ^= array[768+3]
	}

	// Hash in the en-passant file.
	if b.epSquare > 0 {
		// But only if theres actually a pawn ready to capture it. Legality
		// of the potential capture is irrelevant.
		epMask := shiftUp(BBSquares[b.epSquare])
		if b.turn == White {
			epMask = shiftDown(BBSquares[b.epSquare])
		}
		epMask = shiftLeft(epMask) | shiftRight(epMask)

		if epMask&b.pawns&b.occupiedCo[b.turn] > 0 {
			zobristHash ^= array[772+fileIndex(b.epSquare)]
		}
	}

	// Hash in the turn.
	if b.turn == White {
		zobristHash ^= array[780]
	}

	return zobristHash
}

func (b *Bitboard) BoardZobristHash(array []uint64) uint64 {
	if array == nil {
		return b.incrementalZobristHash
	}

	zobristHash := uint64(0)

	squares := b.occupiedCo[Black]
	square := bitScan(squares, 0)
	for square != -1 {
		pieceIndex := (b.PieceTypeAt(square) - 1) * 2
		zobristHash ^= array[64*int(pieceIndex)+8*rankIndex(square)+fileIndex(square)]
		square = bitScan(squares, square+1)
	}

	squares = b.occupiedCo[White]
	square = bitScan(squares, 0)
	for square != -1 {
		pieceIndex := (b.PieceTypeAt(square)-1)*2 + 1
		zobristHash ^= array[64*int(pieceIndex)+8*rankIndex(square)+fileIndex(square)]
		square = bitScan(squares, square+1)
	}

	return zobristHash
}
