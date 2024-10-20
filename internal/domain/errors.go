package domain

import "errors"

var ErrAliasNotFound = errors.New("alias not found")
var ErrAliasDecodeFailed = errors.New("alias decode failed")
var ErrAliasExpired = errors.New("alias expired")
var ErrAliasSearchEngineFailure = errors.New("alias search engine failure")
var ErrAliasCreationFailed = errors.New("alias creation failed")
var ErrStatsCollectingFailed = errors.New("statistics collecting failed")
var ErrUnknownStorageType = errors.New("unknown storage type")
var ErrInternal = errors.New("internal service error")
var ErrProducerGeneralFailure = errors.New("producer general failure")
var ErrUnknownBrokerMessageType = errors.New("unknown broker message type")
var ErrInvalidURLFormat = errors.New("invalid URL format")
