package main

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<30)
	videoIDFromPath := r.PathValue("videoID")
	if videoIDFromPath == "" {
		respondWithError(w, http.StatusBadRequest, "no video ID passed", nil)
		return
	}
	videoID, err := uuid.Parse(videoIDFromPath)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "bad video ID", err)
		return
	}
	jwtToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Not authorized", err)
		return
	}

	userID, err := auth.ValidateJWT(jwtToken, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Not authorized", err)
		return
	}
	videoMeta, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video not found", err)
		return
	}
	if videoMeta.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized", nil)
		return
	}

	f, fHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "upload failed", err)
		return
	}
	defer f.Close()

	mimeType, _, err := mime.ParseMediaType(fHeader.Header.Get("Content-Type"))
	if mimeType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "invalid video format", nil)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to create temporary file", err)
		return
	}
	defer func() {
		err := os.Remove(tempFile.Name())
		if err != nil {
			log.Fatalf("Failed to clean up temporary file")
		}
	}()
	defer tempFile.Close()

	io.Copy(tempFile, f)
	aspectRatio, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get video aspect ratio ", err)
        return
	}
	videoPrefix := func() string {
		switch aspectRatio {
		case "16:9":
			return "landscape"
		case "9:16":
			return "portrait"
		default:
			return "other"
		}
	}()

    log.Default().Printf("aspect ratio: %s, prefix: %s", aspectRatio,videoPrefix)
	tempFile.Seek(0, io.SeekStart)

	fileKey := fmt.Sprintf("%s/%s.mp4",videoPrefix, videoID.String())
	videoURL := cfg.getAwsURL(fileKey)

	_, err = cfg.s3Client.PutObject(
		r.Context(),
		&s3.PutObjectInput{
			Bucket:      &cfg.s3Bucket,
			Key:         &fileKey,
			Body:        tempFile,
			ContentType: &mimeType,
		},
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to upload to s3", err)
		return
	}
	videoMeta.VideoURL = &videoURL
	if err = cfg.db.UpdateVideo(videoMeta); err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to update video db entry", err)
		return
	}
	respondWithJSON(w, http.StatusOK, videoMeta)

}

func (cfg *apiConfig) getAwsURL(k string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, k)
}
