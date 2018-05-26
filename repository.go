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
	"os"
	"github.com/go-redis/redis"
	"github.com/micro/protobuf/proto"
)

type Repository interface {
	GetVideoInfo(p opentracing.SpanContext, id string) (*pb.GetVideoInfoResponse, error)
	GetVideo(p opentracing.SpanContext, id string, resolution string) (string, error)
}

type HostRepository struct {
	s3     *minio.Client
	pg     *sql.DB
	cache  *redis.Client
	tracer *opentracing.Tracer
}

var THUMB_HOST = os.Getenv("THUMB_HOST")

func (repo *HostRepository) GetVideoInfo(parent opentracing.SpanContext, id string) (*pb.GetVideoInfoResponse, error) {
	sp, _ := opentracing.StartSpanFromContext(context.TODO(), "GetVideoInfo_Repo", opentracing.ChildOf(parent))

	sp.LogKV("id", id)

	defer sp.Finish()

	// check for id in cache
	val, err := repo.cache.Get(id).Result()
	if err == redis.Nil {
		// id not in cache
		psSP, _ := opentracing.StartSpanFromContext(context.TODO(), "PG_GetVideoInfo", opentracing.ChildOf(sp.Context()))

		psSP.LogKV("id", id)

		var title string
		var description string
		var date_created string
		var views uint64
		var likes uint64
		var dislikes uint64
		var thumbnail_url string

		selectQuery := `select title, description, date_uploaded, view_count, likes, dislikes from videos where id=$1`
		err := repo.pg.QueryRow(selectQuery, id).Scan(&title, &description, &date_created, &views, &likes, &dislikes)
		if err != nil {
			log.Print(err)
			psSP.Finish()
			return nil, err
		}

		thumbnail_url = THUMB_HOST + "/" + id + ".jpg"

		psSP.Finish()

		sp.LogKV("title", title)
		sp.LogKV("description", description)
		sp.LogKV("date_created", date_created)
		sp.LogKV("views", views)       // NOTE: Protobuf does not transmit variables set
		sp.LogKV("likes", likes)       // to the default values. Therefore, if views, likes,
		sp.LogKV("dislikes", dislikes) // or dislikes = 0, they will not appear in the response
		sp.LogKV("thumbnail_url", thumbnail_url)

		resolutions := &pb.GetVideoInfoResponse_Resolutions{Q720P: true}

		res := &pb.GetVideoInfoResponse{
			Id:           id,
			Title:        title,
			Description:  description,
			DateCreated:  date_created,
			Views:        views,
			Likes:        likes,
			Dislikes:     dislikes,
			Resolutions:  resolutions,
			ThumbnailUrl: thumbnail_url,
		}

		v, err := proto.Marshal(res)
		if err != nil {
			logger.Error("failed to marshal", zap.Error(err))
			return nil, err
		}


		// add res to cache
		repo.cache.Set(id, v, 0)

		return res, nil

	} else if err != nil {
		logger.Error("cache error", zap.Error(err))
		return nil, err
	} else {
		// id found in cache
		logger.Info("cacheRes", zap.String("val", val))

		var res pb.GetVideoInfoResponse
		proto.UnmarshalText(val, &res)

		return &res, nil
	}

	return nil, nil
}

func (repo *HostRepository) GetVideo(p opentracing.SpanContext, id string, resolution string) (string, error) {
	sp, _ := opentracing.StartSpanFromContext(context.TODO(), "GetVideo_Repo", opentracing.ChildOf(p))

	sp.LogKV("id", id, "resolution", resolution)

	defer sp.Finish()

	logger.Info("id", zap.String("id", id))
	logger.Info("resolution", zap.String("resolution", resolution))

	// check for filepath in cache
	filepath, err := repo.cache.Get(id + resolution).Result()
	if err == redis.Nil {
		// filepath not in cache
		// get filepath from db
		dbSP, _ := opentracing.StartSpanFromContext(context.TODO(), "PG_WriteVideoProperties", opentracing.ChildOf(sp.Context()))

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
		err := repo.pg.QueryRow(selectQuery, id).Scan(&filepath)

		if err != nil {
			log.Print(err)
			dbSP.Finish()
			return "", err
		}
		dbSP.Finish()

	} else if err != nil {
		logger.Error("cache error", zap.Error(err))
		return "", err
	} else {
		// filepath in cache
		// continue
	}

	// check for streaming url in cache
	url, err := repo.cache.Get(filepath).Result()
	if err == redis.Nil {
		// url not in cache
		// get url from S3

		s3SP, _ := opentracing.StartSpanFromContext(context.TODO(), "S3_WriteVideoProperties", opentracing.ChildOf(sp.Context()))

		s3SP.LogKV("filepath", filepath)

		presignedUrl, err := repo.s3.PresignedGetObject("videos", filepath, time.Hour*24, nil)
		if err != nil {
			logger.Error("failed presignedGetObject", zap.Error(err))
			s3SP.Finish()
			return "", err
		}

		s3SP.Finish()
		url = presignedUrl.String()

	} else if err != nil {
		logger.Error("cache error", zap.Error(err))
		return "", err
	} else {
		// url in cache
		// continue
	}
	return url, nil

}
