package cos

type SignedUploadUrlResponse struct {
	Auth           string `json:"auth"`
	ExpireInSecond int64  `json:"expireInSecond"`
	File           string `json:"file"`
	SignedURL      string `json:"signedUrl"`
	URL            string `json:"url"`
}
