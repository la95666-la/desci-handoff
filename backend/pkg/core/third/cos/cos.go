package cos

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
	"github.com/tencentyun/cos-go-sdk-v5"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/http/ret"
	"joincare_server/pkg/common/i18n"
	"joincare_server/pkg/common/utils"
)

var ccs = new(sync.Map)

type Client struct {
	bucket    string
	region    string
	secretId  string
	secretKey string

	cli     *cos.Client
	cliOnce *sync.Once

	tmpCli     *cos.Client
	tmpCliOnce *sync.Once
}

func NewClient(bucket, region, secretId, secretKey string) (cc *Client) {
	key := bucket + "|" + region
	if obj, has := ccs.Load(key); has {
		cc = obj.(*Client)
	} else {
		cc = &Client{
			bucket:    bucket,
			region:    region,
			secretId:  secretId,
			secretKey: secretKey,
		}
		cc.tmpCliOnce, cc.cliOnce = new(sync.Once), new(sync.Once)
		ccs.Store(key, cc)
	}

	return
}

func (v *Client) mkBaseUrl() *cos.BaseURL {
	bucketUrl, _ := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", v.bucket, v.region))
	serviceUrl, _ := url.Parse(fmt.Sprintf("https://cos.%s.myqcloud.com", v.region))
	ciUrl, _ := url.Parse(fmt.Sprintf("https://%s.ci.%s.myqcloud.com", v.bucket, v.region))

	return &cos.BaseURL{
		BucketURL:  bucketUrl,
		ServiceURL: serviceUrl,
		CIURL:      ciUrl,
	}
}

func (v *Client) client() *cos.Client {
	v.cliOnce.Do(func() {
		transport := &cos.AuthorizationTransport{
			SecretID:  v.secretId,
			SecretKey: v.secretKey,
		}
		v.cli = cos.NewClient(v.mkBaseUrl(), &http.Client{Transport: transport})
	})

	return v.cli
}

func (v *Client) Upload(file string, bs *[]byte) bool {
	res, err := v.client().Object.Put(context.Background(), file, bytes.NewReader(*bs), nil)
	if err != nil {
		return false
	}

	if res.StatusCode != 200 {
		return false
	}

	return true
}

func (v *Client) UploadByUrl(uploadType, urlStr string) (string, bool) {
	ext := utils.GetFileExtension(urlStr)
	info, err := GetPreSignUrlRepository(uploadType, "", ext)
	if err != nil {
		return "", false
	}
	bs, err := http.Get(urlStr)
	if err != nil {
		return "", false
	}
	res, err := v.client().Object.Put(context.Background(), *info.Path, bs.Body, nil)
	if err != nil {
		return "", false
	}

	if res.StatusCode != 200 {
		return "", false
	}

	return *info.Path, true
}

func (v *Client) PresignedUploadUrl(file string, expired time.Duration, operations string) (*string, *string, error) {
	if presignedURL, err := v.client().Object.
		GetPresignedURL(context.Background(), http.MethodPut, file, v.secretId, v.secretKey, expired, mkOpt(file, operations)); err == nil {
		u := presignedURL.String()
		auth, _ := cos.DecodeURIComponent(presignedURL.RawQuery)
		return &u, &auth, nil
	} else {
		return nil, nil, err
	}
}

func GetPreSignUrlRepository(uploadType, operations, extension string) (*UploadImageRes, error) {
	cc, path, host := MkClient(uploadType)
	if cc == nil {
		return nil, ret.CusError(i18n.ERR_000_00003)
	}

	day := time.Now().Format("20060102")
	uuid := utils.NewUUID32()
	file := fmt.Sprintf("/%s/%s/%s.%s", path, day, uuid, extension)
	u, auth, err := cc.PresignedUploadUrl(file, 180*time.Second, operations)
	if err != nil {
		return nil, err
	}

	res := UploadImageRes{
		ExpireSecond: 180,
		Url:          u,
		HostUrl:      GenerateSignedURL(host, file, viper.GetString("cdn.privateKey")),
		Path:         &file,
		Auth:         auth,
	}
	return &res, nil
}

func MkClient(uploadType string) (cc *Client, path, host string) {
	secretId, secretKey := viper.GetString("tencent.secretId"), viper.GetString("tencent.secretKey")
	bucket, region := viper.GetString("cos.bucket"), viper.GetString("cos.region")
	host = viper.GetString("cos.host")
	switch uploadType {
	case constant.COS_PROGECT_LOGO:
		cc = NewClient(bucket, region, secretId, secretKey)
		path = "project/logo"
	case constant.COS_PROGECT_BANNER:
		cc = NewClient(bucket, region, secretId, secretKey)
		path = "project/banner"
	}
	return
}

func mkOpt(file string, operations ...string) *cos.PresignedURLOptions {
	opt := &cos.PresignedURLOptions{
		Query:  &url.Values{},
		Header: &http.Header{},
	}
	opt.Query.Add("x-cos-security-token", utils.NewUUID32())
	if len(operations) > 0 && operations[0] != "" {
		fileStrs := strings.Split(file, "/")
		pic := &cos.PicOperations{
			IsPicInfo: 1,
			Rules: []cos.PicOperationsRules{
				{
					FileId: fileStrs[len(fileStrs)-1],
					Rule:   operations[0],
				},
			},
		}
		opt.Header.Add("Pic-Operations", cos.EncodePicOperations(pic))
	}
	return opt
}

func GenerateSignedURL(host, path, key string) string {
	host = strings.TrimRight(strings.TrimSpace(host), "/")
	if host == "" || key == "" || path == "" {
		return ""
	}
	now := time.Now().Unix() // 当前时间戳
	signKey := "sign"        // url签名字段
	encoded, _ := url.Parse(path)
	path = encoded.EscapedPath()
	ts := strconv.FormatInt(now, 10)
	randStr := utils.GenerateRandomWord(6, utils.LowercaseLetter, utils.CapitalLetter, utils.Number)
	data := fmt.Sprintf("%s-%s-%s-%d-%s", path, ts, randStr, 0, key)
	hash := md5.Sum([]byte(data))
	sign := hex.EncodeToString(hash[:])
	return fmt.Sprintf("%s%s?%s=%s", host, path, signKey, fmt.Sprintf("%s-%s-%d-%s", ts, randStr, 0, sign))
}
