package model

var allowedMLModelFormats = map[string]bool{
	"pickle":     true,
	"joblib":     true,
	"onnx":       true,
	"torch":      true,
	"tensorflow": true,
	"custom":     true,
}

func IsAllowedMLModelFormat(value string) bool {
	return allowedMLModelFormats[value]
}

func AllowedMLModelFormats() []string {
	return []string{"pickle", "joblib", "onnx", "torch", "tensorflow", "custom"}
}
