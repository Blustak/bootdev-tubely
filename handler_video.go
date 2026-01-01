package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"math"
	"os/exec"
)

func getVideoAspectRatio(filePath string) (string, error) {
	var err error
	cmd := exec.Command("ffprobe",
		"-v",
		"error",
		"-print_format",
		"json",
		"-show_streams",
		filePath,
	)
	var buf []byte
	byteBuf := bytes.NewBuffer(buf)
	cmd.Stdout = byteBuf
	if err = cmd.Run(); err != nil {
		return "", err
	}
	var body struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int64  `json:"width"`
			Height    int64  `json:"height"`
		} `json:"streams"`
	}
	decoder := json.NewDecoder(byteBuf)
	if err = decoder.Decode(&body); err != nil {
		return "", err
	}
	for _, s := range body.Streams {
		if s.CodecType == "video" {
			width, height := float64(s.Width), float64(s.Height)
			log.Default().Printf("width:%f, height:%f, division:%f, landscape:%v",
				width,
				height,
				width/height,
				(width/height == 16.0/9.0))
			if math.Abs(width/height - 16.0/9.0) <= 0.01 {
				return "16:9", nil
			}
			if math.Abs(width/height - 9.0/16.0) <= 0.01 {
				return "9:16", nil
			}
			return "other", nil
		}
	}
	return "", errors.New("failed to find video stream")
}
