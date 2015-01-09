package chess

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	NagNull = iota
	NagGoodMove
	NagMistake
	NagBrilliantMove
	NagBlunder
	NagSpeculativeMove
	NagDubiousMove
	NagForcedMove
	NagSingularMove
	NagWorstMove
	NagDrawishPosition
	NagQuietPosition
	NagActivePosition
	NagUnclearPosition
	NagWhiteSlightAdvantage
	NagBlackSlightAdvantage

	NagWhiteModerateCounterPlay = 132
	NagBlackModerateCounterPlay
	NagWhiteDecisiveCounterPlay
	NagBlackDecisiveCounterPlay
	NagWhiteModerateTimePressure
	NagBlackModerateTimePressure
	NagWhiteSevereTimePressure
	NagBlackSevereTimePressure
)

var TagRegex = regexp.MustCompile("\\[([A-Za-z0-9]+)\\s+\"(.*)\"\\]")

var MoveTextRegex = regexp.MustCompile("(?s)(%.*?[\\n\\r])|(\\{.*)|(\\$[0-9]+)|(\\()|(\\))|(\\*|1-0|0-1|1/2-1/2)|([NBKRQ]?[a-h]?[1-8]?[\\-x]?[a-h][1-8](?:=[nbrqNBRQ])?|--|O-O(?:-O)?|0-0(?:-0)?)|([\\?!]{1,2})")

type GameNode struct {
	parent          *GameNode
	move            *Move
	nags            []int
	startingComment string
	comment         string
	variations      []*GameNode

	boardCached *Bitboard
	Headers     map[string]string
}

// Gets a bitboard with the position of the node. If it's a parent, it will
// get the starting position of the game as a bitboard.
//
// Unless the `SetUp` and `FEN` header tags are set this is the default
// starting position.
//
// It's a copy, so modifying the board will not alter the game.
func (g *GameNode) Board() *Bitboard {
	if g.parent == nil {
		if fen, ok := g.Headers["FEN"]; ok {
			if v, ok := g.Headers["SetUp"]; ok && v == "1" {
				return NewBitboard(fen)
			}
		}

		return NewBitboard("")
	}

	if g.boardCached == nil {
		g.boardCached = g.parent.Board()
		g.boardCached.Push(g.move)
	}

	board := NewBitboard(g.boardCached.Fen())

	return board
}

func (g *GameNode) GetParent() *GameNode {
	return g.parent
}

// Gets the standard algebraic notation of the move leading to this node.
//
// Do not call this on the root node.
func (g *GameNode) San() string {
	return g.parent.Board().San(g.move)
}

// Gets the root node, i.e. the game.
func (g *GameNode) Root() *GameNode {
	node := g

	for node.parent != nil {
		node = node.parent
	}

	return node
}

// Follows the main variation to the end and returns the last node.
func (g *GameNode) End() *GameNode {
	node := g

	for len(node.variations) > 0 {
		node = node.variations[0]
	}

	return node
}

// Checks if this node starts a variation (and can thus have a starting
// comment). The root node does not start a variation and can have no
// starting comment.
func (g *GameNode) StartsVariation() bool {
	if g.parent == nil || len(g.parent.variations) == 0 {
		return false
	}

	return g.parent.variations[0] != g
}

// Checks if the node is in the main line of the game.
func (g *GameNode) IsMainLine() bool {
	node := g

	for node.parent != nil {
		parent := node.parent

		if len(parent.variations) == 0 || parent.variations[0] != node {
			return false
		}

		node = parent
	}

	return true
}

// Checks if this node is the first variation from the point of view of its
// parent. The root node also is in the main variation.
func (g *GameNode) IsMainVariation() bool {
	if g.parent == nil {
		return true
	}

	return len(g.parent.variations) == 0 || g.parent.variations[0] == g
}

// Gets a child node by move.
func (g *GameNode) VariationByMove(move *Move) (*GameNode, int, error) {
	for index, variation := range g.variations {
		if move.Equals(variation.move) {
			return variation, index, nil
		}
	}

	return nil, -1, fmt.Errorf("variation not found")
}

// Gets a child node by index.
func (g *GameNode) VariationByIndex(index int) (*GameNode, error) {
	if index < 0 || index >= len(g.variations) {
		return nil, fmt.Errorf("variation not found")
	}

	return g.variations[index], nil
}

// Checks if the given move appears as a variation.
func (g *GameNode) HasVariation(move *Move) bool {
	for _, variation := range g.variations {
		if move.Equals(variation.move) {
			return true
		}
	}
	return false
}

// Promotes the given move to the main variation.
func (g *GameNode) PromoteToMain(move *Move) {
	variation, i, _ := g.VariationByMove(move)
	g.variations = append(g.variations[0:i], g.variations[i+1:len(g.variations)-1]...)
	g.variations = append([]*GameNode{variation}, g.variations...)
}

// Moves the given variation one up in the list of variations.
func (g *GameNode) Promote(move *Move) {
	_, i, _ := g.VariationByMove(move)
	if i > 0 {
		g.variations[i-1], g.variations[i] = g.variations[i], g.variations[i-1]
	}
}

// Moves the given variation one down in the list of variations.
func (g *GameNode) Demote(move *Move) {
	_, i, _ := g.VariationByMove(move)
	if i < len(g.variations)-1 {
		g.variations[i+1], g.variations[i] = g.variations[i], g.variations[i+1]
	}
}

// Removes a variation by move.
func (g *GameNode) RemoveVariation(move *Move) {
	_, i, _ := g.VariationByMove(move)
	g.variations = append(g.variations[0:i], g.variations[i+1:len(g.variations)-1]...)
}

// Creates a child nodew with the given attributes.
func (g *GameNode) AddVariation(move *Move, comment, startingComment string, nags []int) *GameNode {
	node := &GameNode{
		move:            move,
		nags:            nags,
		parent:          g,
		comment:         comment,
		startingComment: startingComment,
	}
	g.variations = append(g.variations, node)
	return node
}

// Creates a child node with the given attributes and promotes it to the
// main variation.
func (g *GameNode) AddMainVariation(move *Move, comment string) *GameNode {
	node := g.AddVariation(move, comment, "", nil)
	g.PromoteToMain(move)

	return node
}

type Exporter interface {
	PutFullMoveNumber(turn Colors, fullMoveNumber int, afterVariation bool)
	PutMove(board *Bitboard, move *Move)
	PutNags(nags []int)
	PutComment(comment string)
	StartVariation()
	PutStartingComment(startingComment string)
	EndVariation()
	StartGame()
	StartHeaders()
	PutHeader(tagname, tagvalue string)
	EndHeaders()
	PutResult(result string)
	EndGame()
}

func (g *GameNode) Export(exporter Exporter, comments, variations bool, board *Bitboard, afterVariation, headers bool) {
	if g.parent == nil {
		exporter.StartGame()

		if headers {
			exporter.StartHeaders()
			for tagname, tagvalue := range g.Headers {
				exporter.PutHeader(tagname, tagvalue)
			}
			exporter.EndHeaders()
		}

		if comments && len(g.comment) > 0 {
			exporter.PutStartingComment(g.comment)
		}

		g.Export(exporter, comments, variations, nil, false, false)

		exporter.PutResult(g.Headers["Result"])
		exporter.EndGame()
		return
	}

	if board == nil {
		board = g.Board()
	}

	// The mainline move goes first.
	if len(g.variations) > 0 {
		mainVariation := g.variations[0]

		// Append fullmove number.
		exporter.PutFullMoveNumber(board.turn, board.fullMoveNumber, afterVariation)

		// Append SAN.
		exporter.PutMove(board, mainVariation.move)

		if comments {
			// Append NAGs.
			exporter.PutNags(mainVariation.nags)

			// Append the comment.
			if len(mainVariation.comment) > 0 {
				exporter.PutComment(mainVariation.comment)
			}
		}
	}

	// Then export sidelines.
	if variations {
		for _, variation := range g.variations[1:] {
			// Start variation.
			exporter.StartVariation()

			// Append starting comment.
			if comments && len(variation.startingComment) > 0 {
				exporter.PutStartingComment(variation.startingComment)
			}

			// Append fullmove number.
			exporter.PutFullMoveNumber(board.turn, board.fullMoveNumber, true)

			// Append SAN.
			exporter.PutMove(board, variation.move)

			if comments {
				// Append NAGs.
				exporter.PutNags(variation.nags)

				// Append the comment.
				if len(variation.comment) > 0 {
					exporter.PutComment(variation.comment)
				}
			}

			// Recursively append the next moves.
			board.Push(variation.move)
			variation.Export(exporter, comments, variations, board, false, false)
			board.Pop()

			// End variation
			exporter.EndVariation()
		}
	}

	// The mainline is continued last.
	if len(g.variations) > 0 {
		mainVariation := g.variations[0]

		// Recursively append the next moves.
		board.Push(mainVariation.move)
		mainVariation.Export(exporter, comments, variations, board, variations && len(g.variations) > 1, false)
		board.Pop()
	}
}

func (g *GameNode) String() string {
	exporter := NewStringExporter(0)
	g.Export(exporter, true, true, nil, false, false)
	return exporter.String()
}

func NewGame() *GameNode {
	game := &GameNode{}
	game.Headers = map[string]string{}

	game.Headers["Event"] = "?"
	game.Headers["Site"] = "?"
	game.Headers["Date"] = "????.??.??"
	game.Headers["Round"] = "?"
	game.Headers["White"] = "?"
	game.Headers["Black"] = "?"
	game.Headers["Result"] = "*"

	return game
}

// Setup a specific starting position. This sets (or resets) the `SetUp`
// and `FEN` header tags.
func (g *GameNode) Setup(board *Bitboard) {
	fen := board.Fen()

	if fen == StartingFen {
		delete(g.Headers, "SetUp")
		delete(g.Headers, "FEN")
	} else {
		g.Headers["SetUp"] = "1"
		g.Headers["FEN"] = fen
	}
}

// Allows exporting a game as a string.
//
// The export method of `Game` also provides options to include or exclude
// headers, variations or comments. By default everything is included.
//
//     exporter := NewStringExporter(0)
//     game.Export(exporter, true, true, true)
//     pgnString = exporter.String()
//
// Only `columns` characters are written per line. If `columns` is `None` then
// the entire movetext will be on a single line. This does not affect header
// tags and comments.
//
// There will be no newlines at the end of the string.
type StringExporter struct {
	lines       []string
	columns     int
	currentLine string
}

func NewStringExporter(columns int) *StringExporter {
	return &StringExporter{
		columns: columns,
	}
}

func (s *StringExporter) FlushCurrentLine() {
	if s.currentLine != "" {
		s.lines = append(s.lines, strings.TrimRightFunc(s.currentLine, unicode.IsSpace))
	}
	s.currentLine = ""
}

func (s *StringExporter) WriteToken(token string) {
	if s.columns > 0 && s.columns-len(s.currentLine) < len(token) {
		s.FlushCurrentLine()
	}
	s.currentLine += token
}

func (s *StringExporter) WriteLine(line string) {
	s.FlushCurrentLine()
	s.lines = append(s.lines, strings.TrimRightFunc(line, unicode.IsSpace))
}

func (s *StringExporter) StartGame()    {}
func (s *StringExporter) StartHeaders() {}

func (s *StringExporter) EndGame() {
	s.WriteLine("")
}

func (s *StringExporter) PutHeader(tagname, tagvalue string) {
	s.WriteLine(fmt.Sprintf("[%s \"%s\"]", tagname, tagvalue))
}

func (s *StringExporter) EndHeaders() {
	s.WriteLine("")
}

func (s *StringExporter) StartVariation() {
	s.WriteToken("( ")
}

func (s *StringExporter) EndVariation() {
	s.WriteToken(") ")
}

func (s *StringExporter) PutStartingComment(comment string) {
	s.PutComment(comment)
}

func (s *StringExporter) PutComment(comment string) {
	s.WriteToken("{ " + strings.TrimSpace(strings.Replace(comment, "}", "", -1)) + " } ")
}

func (s *StringExporter) PutNags(nags []int) {
	sort.Ints(nags)
	for _, nag := range nags {
		s.PutNag(nag)
	}
}

func (s *StringExporter) PutNag(nag int) {
	s.WriteToken("$" + strconv.Itoa(nag) + " ")
}

func (s *StringExporter) PutFullMoveNumber(turn Colors, fullMoveNumber int, variationStart bool) {
	if turn == White {
		s.WriteToken(strconv.Itoa(fullMoveNumber) + ". ")
	} else if variationStart {
		s.WriteToken(strconv.Itoa(fullMoveNumber) + "... ")
	}
}

func (s *StringExporter) PutMove(board *Bitboard, move *Move) {
	s.WriteToken(board.San(move) + " ")
}

func (s *StringExporter) PutResult(result string) {
	s.WriteToken(result + " ")
}

func (s *StringExporter) String() string {
	if len(s.currentLine) > 0 {
		return strings.TrimRightFunc(strings.Join(append(s.lines, strings.TrimRightFunc(s.currentLine, unicode.IsSpace)), "\n"), unicode.IsSpace)
	}

	return strings.TrimRightFunc(strings.Join(s.lines, "\n"), unicode.IsSpace)
}

// Like a StringExporter, but games are written directly to a text file.
//
// There will always be a blank line after each game. Handling encodings is up
// to the caller.
//
//     newPgn, _ := os.Create("new.pgn")
//     exporter := NewFileExporter(newPgn)
//     game.Export(exporter, true, true, nil, false)
type FileExporter struct {
	*StringExporter

	handle *os.File
}

func NewFileExporter(handle *os.File, columns int) *FileExporter {
	exporter := &FileExporter{
		handle: handle,
	}
	exporter.StringExporter = NewStringExporter(columns)

	return exporter
}

func (f *FileExporter) FlushCurrentLine() {
	if f.currentLine != "" {
		f.handle.WriteString(strings.TrimRightFunc(f.currentLine, unicode.IsSpace))
		f.handle.Write([]byte{'\n'})
	}
	f.currentLine = ""
}

func (f *FileExporter) WriteLine(line string) {
	f.FlushCurrentLine()
	f.handle.WriteString(strings.TrimRightFunc(line, unicode.IsSpace))
	f.handle.Write([]byte{'\n'})
}

type PGNReader struct {
	reader *bufio.Reader
	game   *GameNode
	err    error
}

func NewPGNReader(handle io.ReadSeeker) *PGNReader {
	return &PGNReader{reader: bufio.NewReader(handle)}
}

func (r *PGNReader) Scan() (*GameNode, error) {
	return r.game, r.err
}

// Reads a game from an io.Reader interface.
//
//     pgn, _ := os.Open("data/games/kasparov-deep-blue-1997.pgn")
//     firstGame, _ := chess.ReadGame(pgn)
//     secondGame, _ := chess.ReadGame(pgn)
//
//     fmt.Println(firstGame.Headers["Event"]) // IBM Man-Machine, New York USA
//
// Use `strings.Reader` to parse games from a string.
//
//     pgnString := "1. e4 e5 2. Nf3 *"
//     pgn := strings.NewReader(pgnString)
//     game, _ := chess.ReadGame(pgn)
//
// The end of a game is determined by a completely blank line or the end of
// the file. (Of course blank lines in comments are possible.)
//
// According to the standard at least the usual 7 header tags are required
// for a valid game. This parser also handles games without any headers just
// fine.
//
// The parser is relatively forgiving when it comes to errors. It skips over
// tokens it can not parse. However it is difficult to handle illegal or
// ambiguous moves. If such a move is encountered the default behaviour is to
// stop right in the middle of the game and return an error.
//
// Returns the parsed game or nil if the EOF is reached.
func (r *PGNReader) Next() bool {
	game := NewGame()
	foundGame := false
	foundContent := false

	// Parse game headers.
	line, _ := r.reader.ReadString('\n')
	for len(line) > 0 {
		// Skip empty lines and comments.
		if len(strings.TrimSpace(line)) == 0 || strings.HasPrefix(strings.TrimSpace(line), "%") {
			line, _ = r.reader.ReadString('\n')
			continue
		}

		foundGame = true

		// Read header tags.
		tagMatch := TagRegex.FindStringSubmatch(line)
		if len(tagMatch) > 0 {
			game.Headers[tagMatch[1]] = tagMatch[2]
		} else {
			break
		}

		line, _ = r.reader.ReadString('\n')
	}

	// Get the next non-empty line.
	for len(strings.TrimSpace(line)) == 0 {
		line, _ = r.reader.ReadString('\n')
	}

	// Movetext parser state.
	startingComment := ""
	variationStack := new(Stack)
	variationStack.Push(game)
	boardStack := new(Stack)
	boardStack.Push(game.Board())
	inVariation := false

	// Parse movetext.
	prevLine := ""
	for len(line) > 0 {
		readNextLine := true

		// An empty line is the end of a game.
		if len(strings.TrimSpace(line)) == 0 && foundGame && foundContent {
			r.game = game
			r.err = nil
			return true
		}

		for _, match := range MoveTextRegex.FindAllStringSubmatch(line, -1) {
			token := match[0]

			if strings.HasPrefix(token, "%") {
				// Ignore the rest of the line.
				goto next_line
			}

			foundGame = true

			if strings.HasPrefix(token, "{") {
				// Consume until the end of the comment.
				line = token[1:]
				commentLines := []string{}
				for len(line) > 0 && !strings.Contains(line, "}") {
					commentLines = append(commentLines, strings.TrimRightFunc(line, unicode.IsSpace))
					var err error
					line, err = r.reader.ReadString('\n')
					if err == io.EOF && prevLine == line {
						line = ""
					}
					prevLine = line
				}
				endIndex := strings.Index(line, "}")
				commentLines = append(commentLines, line[:endIndex+1])
				if strings.Contains(line, "}") {
					line = line[endIndex+1:]
				} else {
					line = ""
				}

				tmp := variationStack.Pop()
				if inVariation || (tmp != nil && tmp.(*GameNode).parent == nil) {
					// Add the comment if in the middle of a variation or
					// directly to the game.
					if len(tmp.(*GameNode).comment) > 0 {
						commentLines = append([]string{tmp.(*GameNode).comment}, commentLines...)
					}
					tmp.(*GameNode).comment = strings.TrimSpace(strings.Join(commentLines, "\n"))
				} else {
					// Otherwise it is a starting comment.
					if len(startingComment) > 0 {
						commentLines = append([]string{startingComment}, commentLines...)
					}
					startingComment = strings.TrimSpace(strings.Join(commentLines, "\n"))
				}
				variationStack.Push(tmp)

				// Continue with the current or the next line.
				if len(line) > 0 {
					readNextLine = false
				}

				break
			} else if strings.HasPrefix(token, "$") {
				// Found a NAG.
				tmp := variationStack.Pop().(*GameNode)
				nag, _ := strconv.Atoi(token[1:])
				tmp.nags = append(tmp.nags, nag)
				variationStack.Push(tmp)
			} else if token == "?" {
				tmp := variationStack.Pop().(*GameNode)
				tmp.nags = append(tmp.nags, NagMistake)
				variationStack.Push(tmp)
			} else if token == "??" {
				tmp := variationStack.Pop().(*GameNode)
				tmp.nags = append(tmp.nags, NagBlunder)
				variationStack.Push(tmp)
			} else if token == "!" {
				tmp := variationStack.Pop().(*GameNode)
				tmp.nags = append(tmp.nags, NagGoodMove)
				variationStack.Push(tmp)
			} else if token == "!!" {
				tmp := variationStack.Pop().(*GameNode)
				tmp.nags = append(tmp.nags, NagBrilliantMove)
				variationStack.Push(tmp)
			} else if token == "!?" {
				tmp := variationStack.Pop().(*GameNode)
				tmp.nags = append(tmp.nags, NagSpeculativeMove)
				variationStack.Push(tmp)
			} else if token == "?!" {
				tmp := variationStack.Pop().(*GameNode)
				tmp.nags = append(tmp.nags, NagDubiousMove)
				variationStack.Push(tmp)
			} else if token == "(" {
				// Found a start variation token.
				tmp := variationStack.Pop().(*GameNode)
				if tmp.parent != nil {
					variationStack.Push(tmp)
					variationStack.Push(tmp.parent)

					tmpBoard := boardStack.Pop().(*Bitboard)
					board := NewBitboard(tmpBoard.Fen())
					board.Pop()
					boardStack.Push(tmpBoard)
					boardStack.Push(board)

					inVariation = false
				} else {
					variationStack.Push(tmp)
				}
			} else if token == ")" {
				// Found a close variation token. Always leave at least the
				// root node on the stack.
				if variationStack.Len() > 1 {
					variationStack.Pop()
					boardStack.Pop()
				}
			} else if (token == "1-0" || token == "0-1" || token == "1/2-1/2" || token == "*") && variationStack.Len() == 1 {
				// Found a result token.
				foundContent = true

				// Set result header if not present, yet.
				if _, ok := game.Headers["Result"]; !ok {
					game.Headers["Result"] = token
				}
			} else {
				// Found a SAN token.
				foundContent = true

				// Replace zeroes castling notation.
				if token == "0-0" {
					token = "O-O"
				} else if token == "0-0-0" {
					token = "O-O-O"
				}

				// Parse the SAN.
				tmp := boardStack.Pop().(*Bitboard)
				boardStack.Push(tmp)
				move, err := tmp.ParseSan(token)
				if err != nil {
					r.game = game
					r.err = err
					return true
				}
				inVariation = true
				tmpVar := variationStack.Pop().(*GameNode)
				tmpVar = tmpVar.AddVariation(move, "", "", nil)
				tmpVar.startingComment = startingComment
				variationStack.Push(tmpVar)
				tmp.Push(move)
				startingComment = ""
			}
		}

	next_line:
		if readNextLine {
			var err error
			line, err = r.reader.ReadString('\n')
			if err == io.EOF && prevLine == line {
				line = ""
			}
			prevLine = line
		}
	}

	if foundGame {
		r.game = game
		r.err = nil
		return true
	}

	r.game = nil
	r.err = fmt.Errorf("game not found")
	return false
}

// Scan a PGN from io.Reader for game offsets and headers.
//
// Returns an array of offsets for the games a map for game headers.
//
// Since actually parsing many games from a big file is relatively expensive,
// this is a better way to look only for specific games and seek and parse
// them later.
//
// This example scans for the first game with Kasparov as the white player.
//
//     pgn, _ := os.Open("mega.pgn")
//     offsets, headers := chess.ScanHeaders(pgn, 0)
//     for index, header := range headers {
//         if strings.Contains(header["White"], "Kasparov") {
//             kasparovOffset = offsets[index]
//             break
//         }
//     }
//
// Then it can later be seeked an parsed.
//
//     pgn.Seek(kasparovOffset, 0)
//     game = chess.ReadGame(pgn)
//
// Be careful when seeking a game in the file while more offsets are being
// generated.
func ScanHeaders(handle io.ReadSeeker) ([]int64, []map[string]string) {
	offsets := []int64{}
	headers := []map[string]string{}

	inComment := false

	var gameHeaders map[string]string
	var gamePos int64
	hasInit := false

	lastPos, _ := handle.Seek(0, 1)

	scanner := bufio.NewScanner(handle)
	scanner.Scan()
	line := scanner.Text()

	for len(line) > 0 {
		// Skip single line comments.
		if strings.HasPrefix(line, "%") {
			lastPos, _ = handle.Seek(0, 1)
			scanner.Scan()
			line = scanner.Text()
			continue
		}

		// Reading a header tag. Parse it and add it to the current headers.
		if !inComment && strings.HasPrefix(line, "[") {
			tagMatch := TagRegex.FindStringSubmatch(line)
			if len(tagMatch) > 0 {
				if !hasInit {
					gameHeaders = map[string]string{
						"Event":  "?",
						"Site":   "?",
						"Date":   "????.??.??",
						"Round":  "?",
						"White":  "?",
						"Black":  "?",
						"Result": "*",
					}

					gamePos = lastPos
					hasInit = true
				}

				gameHeaders[tagMatch[1]] = tagMatch[2]

				lastPos, _ = handle.Seek(0, 1)
				scanner.Scan()
				line = scanner.Text()
				continue
			}
		}

		// Reading movetext. Update parser state inComment in order to skip
		// comments that look like header tags.
		if (!inComment && strings.Contains(line, "{")) || (inComment && strings.Contains(line, "}")) {
			inComment = strings.LastIndex(line, "{") > strings.LastIndex(line, "}")
		}

		// Reading movetext. If there were headers, previously, those are now
		// complete and can be appended.
		if hasInit {
			offsets = append(offsets, gamePos)
			headers = append(headers, gameHeaders)
			hasInit = false
		}

		lastPos, _ = handle.Seek(0, 1)
		scanner.Scan()
		line = scanner.Text()
	}

	// Append the headers of the last game.
	if hasInit {
		offsets = append(offsets, gamePos)
		headers = append(headers, gameHeaders)
	}

	return offsets, headers
}

// Scan a PGN for game offsets.
//
// Returns the starting offsets of all the games, so that they can be seeked
// later. This is just like `ScanHeaders()` but more efficient if you do
// not actually need the header information.
//
// The PGN standard requires each game to start with an Event-tag. So does
// this scanner.
func ScanOffsets(handle io.ReadSeeker) []int64 {
	reader := bufio.NewReader(handle)
	inComment := false
	result := []int64{}

	lastPos, _ := handle.Seek(0, 1)
	line, err := reader.ReadString('\n')

	for err != io.EOF {
		if !inComment && strings.HasPrefix(line, "[Event \"") {
			result = append(result, lastPos)
		} else if (!inComment && strings.Contains(line, "{")) || (inComment && strings.Contains(line, "}")) {
			inComment = strings.LastIndex(line, "{") > strings.LastIndex(line, "}")
		}

		lastPos, _ = handle.Seek(0, 1)
		line, err = reader.ReadString('\n')
	}

	return result
}
