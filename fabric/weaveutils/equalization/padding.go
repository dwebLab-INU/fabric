package padding

import (
	"encoding/json"

	"github.com/Watchdog-Network/types"
)

// EqualizeParsedBlockLentghs adds padding to parsed blocks to make them devided by number of source symbols.
// Returns a slice of serialized parsedblock with padding.
func EqualizeParsedBlockLengths(numSourceSymbols int, symbolAlignmentSize int, numEncodedSourceSymbols int, parsedBlock types.HLFEncodedBlock) []byte {

	marshalledBlock, _ := json.Marshal(parsedBlock)

	var padding []int
	if len(marshalledBlock)%numSourceSymbols != 0 {
		if ((numSourceSymbols-(len(marshalledBlock)%numSourceSymbols))/2)%2 == 0 {
			padding = make([]int, ((numSourceSymbols-(len(marshalledBlock)%numSourceSymbols))/2)+2)
		} else {
			padding = make([]int, ((numSourceSymbols-(len(marshalledBlock)%numSourceSymbols))/2)+1)
		}
	}

	parsedBlock.Padding = padding
	marshalledBlock, _ = json.Marshal(parsedBlock)

	return marshalledBlock
}
