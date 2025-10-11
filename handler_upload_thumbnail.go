package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
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
	multiFile, multiFileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "couldn't parse file", err)
		return
	}
	fileType := multiFileHeader.Header.Get("Content-Type")
	if fileType == "" {
		respondWithError(w, http.StatusBadRequest, "couldn't find content-type header", err)
		return
	}
    mediaTypes, _, err := mime.ParseMediaType(fileType)
    if err != nil {
        respondWithError(w, http.StatusBadRequest,"couldn't parse content-type header", err)
        return
    }

    var fileExt string
    switch mediaTypes {
    case "image/jpeg", "image/png":
        _,fileExt,err = parseMimeType(mediaTypes)
        if err != nil {
            respondWithError(w,http.StatusInternalServerError,"couldn't parse header contents", err)
            return
        }
    default:
        respondWithError(w, http.StatusBadRequest,"unsupported MIME type",errors.New("unsupported MIME type"))
        return

    }

	data, err := io.ReadAll(multiFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't read file contents", err)
		return
	}
	meta, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "couldn't find video metadata", err)
		return
	}
	if meta.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "user is not video owner", errors.New("unauthorized"))
		return
	}
	thumb := thumbnail{
		data:      data,
		mediaType: fileExt,
	}
    fileName := fmt.Sprintf("%s.%s",videoID.String(),thumb.mediaType)
    filePath := filepath.Join(cfg.assetsRoot,fileName)
    f, err := os.Create(filePath)
    if err != nil {
        respondWithError(w,http.StatusInternalServerError,"couldn't create thumbnail file",err)
        return
    }
    defer f.Close()
    if _, err = io.Copy(f,bytes.NewReader(thumb.data)); err != nil {
        respondWithError(w,http.StatusInternalServerError,"couldn't write to file", err)
        return
    }
    thumbURL := fmt.Sprintf("http://localhost:8091/assets/%s.%s",videoID.String(),thumb.mediaType)
	meta.ThumbnailURL = &thumbURL
	if err = cfg.db.UpdateVideo(meta); err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to update video entry", err)
		return
	}

	respondWithJSON(w, http.StatusOK, meta)
}

func parseMimeType(s string) (string, string, error) {
    sepString := strings.Split(s,"/")
    if len(sepString) != 2 {
        return "","",errors.New("malformed input")
    }
    return sepString[0],sepString[1],nil
}
