package runtime

import (
	"bytes"
	"os"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
)

var utf8BOM = []byte{0xef, 0xbb, 0xbf}

func readInputJSONBytes(inputPath string) ([]byte, error) {
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInput, err)
	}
	return stripUTF8BOM(inputBytes), nil
}

func stripUTF8BOM(inputBytes []byte) []byte {
	if bytes.HasPrefix(inputBytes, utf8BOM) {
		return inputBytes[len(utf8BOM):]
	}
	return inputBytes
}
