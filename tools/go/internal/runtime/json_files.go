package runtime

import (
	"github.com/goniegonie/hvac-studio/tools/go/internal/apperror"
	"github.com/goniegonie/hvac-studio/tools/go/internal/jsonfile"
)

func readInputJSONBytes(inputPath string) ([]byte, error) {
	inputBytes, err := jsonfile.Read(inputPath)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInput, err)
	}
	return inputBytes, nil
}
