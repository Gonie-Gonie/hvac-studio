package jsonfile

import (
	"bytes"
	"os"
)

var utf8BOM = []byte{0xef, 0xbb, 0xbf}

func Read(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return StripUTF8BOM(data), nil
}

func StripUTF8BOM(data []byte) []byte {
	if bytes.HasPrefix(data, utf8BOM) {
		return data[len(utf8BOM):]
	}
	return data
}
