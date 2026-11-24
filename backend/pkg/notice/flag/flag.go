package flag

import "os"

var ip = os.Getenv("POD_IP")
var port = os.Getenv("POD_PORT")

func IP() string {
	return ip
}

func Port() string {
	return port
}
