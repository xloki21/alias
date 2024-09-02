package domain

import "errors"

var ErrAliasNotFound = errors.New("alias not found")
var ErrAliasExpired = errors.New("alias expired")
var ErrStatsCollectingFailed = errors.New("statistics collecting failed")
var ErrUnknownStorageType = errors.New("unknown storage type")
