package studio

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/korean"
	textunicode "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func decodeDatasetBytes(data []byte, requestedEncoding string) ([]byte, string, error) {
	encodingName := normalizeDatasetEncoding(requestedEncoding)
	if encodingName == "" || encodingName == "auto" {
		encodingName = detectDatasetEncoding(data)
	}
	switch encodingName {
	case "utf-8", "utf-8-bom":
		decoded := bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
		if !utf8.Valid(decoded) {
			return nil, "", apperror.Errorf(apperror.CodeInput, "dataset source is not valid UTF-8; choose Auto or the correct encoding")
		}
		return decoded, encodingName, nil
	case "utf-16", "utf-16le", "utf-16be":
		endian := textunicode.LittleEndian
		if encodingName == "utf-16be" {
			endian = textunicode.BigEndian
		}
		decoder := textunicode.UTF16(endian, textunicode.UseBOM).NewDecoder()
		if encodingName == "utf-16le" || encodingName == "utf-16be" {
			decoder = textunicode.UTF16(endian, textunicode.IgnoreBOM).NewDecoder()
		}
		decoded, err := transformDatasetBytes(data, decoder)
		if err != nil {
			return nil, "", apperror.Wrap(apperror.CodeInput, fmt.Errorf("decode dataset as %s: %w", encodingName, err))
		}
		return decoded, encodingName, nil
	case "cp949", "euc-kr":
		decoded, err := transformDatasetBytes(data, korean.EUCKR.NewDecoder())
		if err != nil {
			return nil, "", apperror.Wrap(apperror.CodeInput, fmt.Errorf("decode dataset as %s: %w", encodingName, err))
		}
		return decoded, encodingName, nil
	default:
		return nil, "", apperror.Errorf(apperror.CodeValidation, "unsupported dataset encoding: %s", requestedEncoding)
	}
}

func transformDatasetBytes(data []byte, decoder *encoding.Decoder) ([]byte, error) {
	reader := transform.NewReader(bytes.NewReader(data), decoder)
	return io.ReadAll(reader)
}

func normalizeDatasetEncoding(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "auto", "detect":
		return "auto"
	case "utf8", "utf-8":
		return "utf-8"
	case "utf8bom", "utf-8-bom", "utf-8 bom":
		return "utf-8-bom"
	case "utf16", "utf-16":
		return "utf-16"
	case "utf16le", "utf-16le", "utf-16 le":
		return "utf-16le"
	case "utf16be", "utf-16be", "utf-16 be":
		return "utf-16be"
	case "cp949", "windows-949", "ms949", "euc-kr", "euckr", "ks_c_5601-1987", "ksc5601":
		return "cp949"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func detectDatasetEncoding(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		return "utf-8-bom"
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}), bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return "utf-16"
	case looksLikeUTF16LE(data):
		return "utf-16le"
	case looksLikeUTF16BE(data):
		return "utf-16be"
	case utf8.Valid(data):
		return "utf-8"
	default:
		return "cp949"
	}
}

func looksLikeUTF16LE(data []byte) bool {
	return utf16ZeroPattern(data, 1)
}

func looksLikeUTF16BE(data []byte) bool {
	return utf16ZeroPattern(data, 0)
}

func utf16ZeroPattern(data []byte, zeroIndex int) bool {
	pairs := len(data) / 2
	if pairs < 4 {
		return false
	}
	limit := pairs
	if limit > 64 {
		limit = 64
	}
	zeros := 0
	for index := 0; index < limit; index++ {
		if data[index*2+zeroIndex] == 0 {
			zeros++
		}
	}
	return zeros*2 >= limit
}
