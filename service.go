package main

import (
	"errors"
	pb "github.com/agxp/cloudflix/video-hosting-svc/proto"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"golang.org/x/net/context"
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

func (srv *service) GetVideoInfo(ctx context.Context, req *pb.Request, res *pb.Response) error {
	sp, _ := opentracing.StartSpanFromContext(context.Background(), "GetVideoInfo_Service")

	logger.Info("Request for GetVideoInfo_Service received")
	defer sp.Finish()

	title, description, date_created, views, likes, dislikes, resolutions, err := srv.repo.GetVideoInfo(sp.Context(), req.Id)
	if err != nil {
		logger.Error("failed GetVideoInfo", zap.Error(err))
		return err
	}

	res.Title = title
	res.Description = description
	res.DateCreated = date_created
	res.Views = views
	res.Likes = likes
	res.Dislikes = dislikes
	res.Resolutions = resolutions

	return nil
}

func (srv *service) GetVideo(ctx context.Context, req *pb.GetVideoRequest, res *pb.GetVideoResponse) error {
	sp, _ := opentracing.StartSpanFromContext(context.Background(), "GetVideo_Service")
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
