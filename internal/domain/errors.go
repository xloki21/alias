package domain

import "errors"

var ErrAliasNotFound = errors.New("alias not found")
var ErrAliasExpired = errors.New("alias expired")
var ErrAliasCreationFailed = errors.New("alias creation failed")
var ErrStatsCollectingFailed = errors.New("statistics collecting failed")
var ErrUnknownStorageType = errors.New("unknown storage type")
var ErrInternal = errors.New("internal service error")
var ErrInvalidURLFormat = errors.New("invalid URL format")
