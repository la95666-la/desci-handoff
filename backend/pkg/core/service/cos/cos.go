package cos

import (
	"errors"
	"strings"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/core/third/cos"
)

func SignedUploadUrl(uploadType string, ext string) (*SignedUploadUrlResponse, error) {
	ext = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
	allowed := map[string]struct{}{"png": {}, "jpg": {}, "jpeg": {}, "webp": {}, "gif": {}}
	if _, ok := allowed[ext]; !ok {
		return nil, errors.New("unsupported project image extension")
	}
	if uploadType != constant.COS_PROGECT_LOGO && uploadType != constant.COS_PROGECT_BANNER {
		return nil, errors.New("unsupported project image upload type")
	}
	info, err := cos.GetPreSignUrlRepository(uploadType, "", ext)
	if err != nil {
		return nil, err
	}
	return &SignedUploadUrlResponse{
		Auth:           *info.Auth,
		ExpireInSecond: int64(info.ExpireSecond),
		File:           *info.Path,
		SignedURL:      *info.Url,
		URL:            info.HostUrl,
	}, nil
}
