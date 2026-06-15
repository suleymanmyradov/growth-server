package settingslogic

import (
	"context"
	"fmt"
	"mime"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/fileManagerClient"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UploadAvatarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUploadAvatarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadAvatarLogic {
	return &UploadAvatarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UploadAvatarLogic) UploadAvatar(in *client.UploadAvatarRequest) (*client.UploadAvatarResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "UploadAvatarLogic.UploadAvatar")
	defer span.End()

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid user id")
	}

	ext := filepath.Ext(in.Format)
	if ext == "" {
		ext = "." + in.Format
	}
	filename := fmt.Sprintf("%s%s", userID.String(), ext)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "image/jpeg"
	}

	uploadResp, err := l.svcCtx.FileManagerRpc.UploadFile(ctx, &fileManagerClient.UploadFileRequest{
		Data:        in.ImageData,
		Filename:    filename,
		ContentType: contentType,
		Folder:      "avatars",
	})
	if err != nil {
		l.Errorf("upload avatar to filemanager failed: %v", err)
		return nil, status.Error(codes.Internal, "avatar upload failed")
	}

	return &client.UploadAvatarResponse{
		AvatarUrl: uploadResp.Url,
	}, nil
}
