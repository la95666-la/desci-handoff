package flag

import "os"

var (
	token = ""
)

func Token() string {
	if token == "" {
		token = os.Getenv("JC_TOKEN")
	}
	return token
}
