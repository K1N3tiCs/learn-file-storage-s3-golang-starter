package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

// const maxVideoSize = 1 << 30

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading video", videoID, "by user", userID)

	// TODO: implement the upload here

	const maxMemory int64 = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't upload thumbnail", err)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't get media type", err)
		return
	}
	fmt.Println(params)
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "video type should be mp4", err)
		return
	}
	mediaExtension := strings.Split(mediaType, "/")
	/*
		data, err := io.ReadAll(file)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "couldn't read file", err)
			return
		}
	*/

	metaData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't get video info", err)
		return
	}

	if metaData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "unauthorized video", err)
		return
	}

	randomBytes := make([]byte, 32)
	_, err = rand.Read(randomBytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't fill random bytes", err)
		return
	}

	destFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't save file", err)
		return
	}
	defer os.Remove(destFile.Name())
	defer destFile.Close()

	written, err := io.Copy(destFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't write file", err)
		return
	}

	_, err = destFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't reset Tempfile's file pointer", err)
		return
	}

	fmt.Println(written)

	videoFile := base64.RawURLEncoding.EncodeToString(randomBytes) + "." + mediaExtension[1]
	s3PutObjectOutput, err := cfg.s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &videoFile,
		Body:        destFile,
		ContentType: &mediaType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't upload Tempfile to s3 bucket", err)
		return
	}

	fmt.Println(s3PutObjectOutput.Size)
	fmt.Println(s3PutObjectOutput.BucketKeyEnabled)
	fmt.Println(s3PutObjectOutput.ETag)
	fmt.Println(s3PutObjectOutput.ResultMetadata)

	// thumbnailData := base64.StdEncoding.EncodeToString(data)
	// thumbnailURL := fmt.Sprintf("data:%s;base64,%s", mediaType, thumbnailData)

	/*
		videoThumbnails[videoID] = thumbnail{
			data:      data,
			mediaType: mediaType,
		}
	*/

	videoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, videoFile)
	metaData.VideoURL = &videoURL
	err = cfg.db.UpdateVideo(metaData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't update video metadata", err)
		return
	}

	respondWithJSON(w, http.StatusOK, database.Video{
		ID:                metaData.ID,
		CreatedAt:         metaData.CreatedAt,
		UpdatedAt:         metaData.UpdatedAt,
		ThumbnailURL:      metaData.ThumbnailURL,
		VideoURL:          metaData.VideoURL,
		CreateVideoParams: metaData.CreateVideoParams,
	})

}
