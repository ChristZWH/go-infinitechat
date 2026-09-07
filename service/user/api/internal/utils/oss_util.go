package utils

import (
	"context"
	"go-infinitechat/common/common"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	minioClient *minio.Client
	minioUrl    string
)

const (
	BucketName        = "go-inifinitechat-img"
	PictureExpireTime = 3000
)

// InitMinio 初始化 MinIO 客户端，在 ServiceContext 中调用
func InitMinio(rawUrl, accessKey, secretKey string) {
	minioUrl = rawUrl

	u, err := url.Parse(rawUrl)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "Minio初始化错误", err)

	useSSL := u.Scheme == "https"

	// endpoint 只接受 host 形式
	client, err := minio.New(u.Host, &minio.Options{
		// credentials.NewStaticV4(ak, sk, "")：静态 AK/SK 凭证 + V4 签名算法；第三个参数是临时凭证的 session token，本地部署永远留空
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	common.ThrowIfWithMsg(err != nil, common.SystemError, "create minio client error", err)

	minioClient = client // 存包级全局，单例
}

// GetUploadUrl 生成预签名的上传 URL（PUT 方法）
// GetUploadUrl(BucketName, "avatar/10001/xxx.png", 300)
func GetUploadUrl(ctx context.Context, bucketName string, objectName string, expires int32) string {
	presignedUrl, err := minioClient.PresignedPutObject(
		ctx,
		bucketName,
		objectName,
		time.Duration(expires)*time.Second,
	)
	common.ThrowIfWithMsg(err != nil, common.SystemError, "获取上传 URL 失败", err)
	return presignedUrl.String()
}

// GetDownloadUrl 拼接下载 URL
// http://localhost:9000/go-inifinitchat-img/avatar/10001/xxx.png
func GetDownloadUrl(bucketName string, fileName string) string {
	return minioUrl + "/" + bucketName + "/" + fileName
}
