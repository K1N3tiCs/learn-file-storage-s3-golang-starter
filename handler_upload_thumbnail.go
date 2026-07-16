package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here

	const maxMemory int64 = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't upload thumbnail", err)
		return
	}

	mediaType := header.Header.Get("Content-Type")
	data, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't read file", err)
		return
	}

	metaData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't get video info", err)
		return
	}

	if metaData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "unauthorized video", err)
		return
	}

	thumbnailData := base64.StdEncoding.EncodeToString(data)
	thumbnailURL := fmt.Sprintf("data:%s;base64,%s", mediaType, thumbnailData)

	/*
		videoThumbnails[videoID] = thumbnail{
			data:      data,
			mediaType: mediaType,
		}
	*/

	metaData.ThumbnailURL = &thumbnailURL
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
