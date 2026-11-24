package cos

type UploadImageRes struct {
	ExpireSecond int     `json:"expireSecond"`
	Url          *string `json:"url"`
	Path         *string `json:"path"`
	UrlConverted string  `json:"urlConverted,omitempty"`
	HostUrl      string  `json:"hostUrl"`
	Auth         *string `json:"auth"`
}
