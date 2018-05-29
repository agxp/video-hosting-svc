package main

import (
	"errors"
	pb "github.com/agxp/cloudflix/video-hosting-svc/proto"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"github.com/micro/protobuf/proto"
)

type service struct {
	repo   Repository
	tracer *opentracing.Tracer
	logger *zap.Logger
}

var VALID_RESOLUTIONS = map[string]bool{
	"144p":  true,
	"240p":  true,
	"360p":  true,
	"480p":  true,
	"720p":  true,
	"1080p": true,
}

func (srv *service) GetVideoInfo(ctx context.Context, req *pb.GetVideoInfoRequest, res *pb.GetVideoInfoResponse) error {
	sp, _ := opentracing.StartSpanFromContext(ctx, "GetVideoInfo_Service")

	logger.Info("Request for GetVideoInfo_Service received")
	defer sp.Finish()

	rsp, err := srv.repo.GetVideoInfo(sp.Context(), req.Id)
	if err != nil {
		logger.Error("failed GetVideoInfo", zap.Error(err))
		return err
	}

	data, err := proto.Marshal(rsp)
	if err != nil {
		logger.Error("marshal error", zap.Error(err))
	}

	err = proto.Unmarshal(data, res)
	if err != nil {
		logger.Error("unmarshal error", zap.Error(err))
		return err
	}

	sp.LogKV("rsp_title", rsp.Title)
	sp.LogKV("rsp_description", rsp.Description)
	sp.LogKV("rsp_date_created", rsp.DateCreated)
	sp.LogKV("rsp_views", rsp.Views)       // NOTE: Protobuf does not transmit variables set
	sp.LogKV("rsp_likes", rsp.Likes)       // to the default values. Therefore, if views, likes,
	sp.LogKV("rsp_dislikes", rsp.Dislikes) // or dislikes = 0, they will not appear in the response
	sp.LogKV("rsp_thumbnail_url", rsp.ThumbnailUrl)

	sp.LogKV("res_title", res.Title)
	sp.LogKV("res_description", res.Description)
	sp.LogKV("res_date_created", res.DateCreated)
	sp.LogKV("res_views", res.Views)       // NOTE: Protobuf does not transmit variables set
	sp.LogKV("res_likes", res.Likes)       // to the default values. Therefore, if views, likes,
	sp.LogKV("res_dislikes", res.Dislikes) // or dislikes = 0, they will not appear in the response
	sp.LogKV("res_thumbnail_url", res.ThumbnailUrl)





	return nil
}

func (srv *service) GetVideo(ctx context.Context, req *pb.GetVideoRequest, res *pb.GetVideoResponse) error {
	sp, _ := opentracing.StartSpanFromContext(ctx, "GetVideo_Service")
	logger.Info("Request for GetVideo_Service received")
	defer sp.Finish()

	if !VALID_RESOLUTIONS[req.Resolution] {
		err := errors.New("not a valid resolution")
		logger.Error("failed GetVideo_Service", zap.Error(err))
		return err
	}

	url, err := srv.repo.GetVideo(sp.Context(), req.Id, req.Resolution)
	if err != nil {
		logger.Error("failed GetVideo", zap.Error(err))
		return err
	}

	res.PresignedUrl = url

	return nil
}
