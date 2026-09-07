// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"

	"go-infinitechat/service/user/api/internal/svc"
	"go-infinitechat/service/user/api/internal/types"
	"go-infinitechat/service/user/api/internal/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUploadUrlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUploadUrlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUploadUrlLogic {
	return &GetUploadUrlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUploadUrlLogic) GetUploadUrl(req *types.GetUploadURLRequest) (resp *types.UploadURLResponse, err error) {
	uploadUrl := utils.GetUploadUrl(l.ctx, utils.BucketName, req.FileName, utils.PictureExpireTime)
	downLoadUrl := utils.GetDownloadUrl(utils.BucketName, req.FileName)
	return &types.UploadURLResponse{
		UploadURL:   uploadUrl,
		DownloadURL: downLoadUrl,
	}, nil
}
