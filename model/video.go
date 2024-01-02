package model

import (
	"time"

	"gorm.io/gorm"
)

type DownloadStatus int

const (
	DownloadStatusInit DownloadStatus = iota
	DownloadStatusDownloading
	DownloadStatusDownloaded
	DownloadStatusError
)

type Video struct {
	gorm.Model
	Name        string
	Url         string
	Headers     string
	IsLive      bool
	Status      DownloadStatus
	CreatedDate time.Time
	UpdatedDate time.Time
}
