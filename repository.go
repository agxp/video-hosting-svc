package main

import (
	"context"
	"database/sql"
	pb "github.com/agxp/cloudflix/video-hosting-svc/proto"
	"github.com/minio/minio-go"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"log"
	"time"
)

type Repository interface {
	GetVideoInfo(p opentracing.SpanContext, id string) (string, string, string, uint64, uint64, uint64, *pb.Response_Resolutions, error)
	GetVideo(p opentracing.SpanContext, id string, resolution string) (string, error)
}

type HostRepository struct {
	s3     *minio.Client
	pg     *sql.DB
	tracer *opentracing.Tracer
}

func (repo *HostRepository) GetVideoInfo(parent opentracing.SpanContext, id string) (string, string, string, uint64, uint64, uint64, *pb.Response_Resolutions, error) {
	sp, _ := opentracing.StartSpanFromContext(context.Background(), "GetVideoInfo_Repo", opentracing.ChildOf(parent))

	sp.LogKV("id", id)

	defer sp.Finish()

	psSP, _ := opentracing.StartSpanFromContext(context.Background(), "PG_GetVideoInfo", opentracing.ChildOf(sp.Context()))

	psSP.LogKV("id", id)

	var title string
	var description string
	var date_created string
	var views uint64
	var likes uint64
	var dislikes uint64

	selectQuery := `select title, description, date_uploaded, view_count, likes, dislikes from videos where id=$1`
	err := repo.pg.QueryRow(selectQuery, id).Scan(&title, &description, &date_created, &views, &likes, &dislikes)
	if err != nil {
		log.Print(err)
		psSP.Finish()
		return "", "", "", 0, 0, 0, nil, err
	}

	psSP.Finish()

	sp.LogKV("title", title)
	sp.LogKV("description", description)
	sp.LogKV("date_created", date_created)
	sp.LogKV("views", views) 				// NOTE: Protobuf does not transmit variables set
	sp.LogKV("likes", likes) 				// to the default values. Therefore, if views, likes,
	sp.LogKV("dislikes", dislikes) 		// or dislikes = 0, they will not appear in the response

	resolutions := &pb.Response_Resolutions{Q720P: true}

	return title, description, date_created, views, likes, dislikes, resolutions, nil
}

func (repo *HostRepository) GetVideo(p opentracing.SpanContext, id string, resolution string) (string, error) {
	sp, _ := opentracing.StartSpanFromContext(context.Background(), "GetVideo_Repo", opentracing.ChildOf(p))

	sp.LogKV("id", id, "resolution", resolution)

	defer sp.Finish()

	logger.Info("id", zap.String("id", id))
	logger.Info("resolution", zap.String("resolution", resolution))

	dbSP, _ := opentracing.StartSpanFromContext(context.Background(), "PG_WriteVideoProperties", opentracing.ChildOf(sp.Context()))

	dbSP.LogKV("id", id, "resolution", resolution)

	// This won't work until I finish video-encoding-svc
	// selectQuery := `select ` + resolution + ` from video_qualities where id=$1`
	//var url string
	//err := repo.pg.QueryRow(selectQuery, id).Scan(&url)
	//if err != nil {
	//	log.Print(err)
	//	dbSP.Finish()
	//	return "", err
	//}
	selectQuery := `select file_path from videos where id=$1`
	var path string
	err := repo.pg.QueryRow(selectQuery, id).Scan(&path)

	if err != nil {
		log.Print(err)
		dbSP.Finish()
		return "", err
	}
	dbSP.Finish()

	s3SP, _ := opentracing.StartSpanFromContext(context.Background(), "PG_WriteVideoProperties", opentracing.ChildOf(sp.Context()))

	s3SP.LogKV("path", path)

	presignedUrl, err := repo.s3.PresignedGetObject("videos", path, time.Hour*24, nil)
	if err != nil {
		logger.Error("failed presignedGetObject", zap.Error(err))
		s3SP.Finish()
		return "", err
	}

	s3SP.Finish()


	return presignedUrl.String(), nil
}
